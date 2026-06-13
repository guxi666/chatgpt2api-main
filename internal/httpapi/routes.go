package httpapi

import (
	"encoding/csv"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"chatgpt2api/internal/service"
	"chatgpt2api/internal/util"
)

func looksLikeEmail(value string) bool {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" || strings.Contains(value, " ") {
		return false
	}
	at := strings.LastIndex(value, "@")
	if at <= 0 || at >= len(value)-1 {
		return false
	}
	domain := value[at+1:]
	return strings.Contains(domain, ".")
}

func isLocalInvalidEmail(value string) bool {
	value = strings.TrimSpace(strings.ToLower(value))
	return strings.HasSuffix(value, "@local.invalid")
}

func managedUserEmailCandidate(values ...string) string {
	for _, value := range values {
		email := strings.TrimSpace(value)
		if !looksLikeEmail(email) || isLocalInvalidEmail(email) {
			continue
		}
		return email
	}
	return ""
}

func inferManagedUserProvider(authProvider, username, accountEmail, billingProvider, billingEmail string) string {
	if strings.TrimSpace(authProvider) == service.AuthProviderLinuxDo {
		return service.AuthProviderLinuxDo
	}
	if strings.TrimSpace(billingProvider) == service.AuthProviderEmail && managedUserEmailCandidate(billingEmail) != "" {
		return service.AuthProviderEmail
	}
	if managedUserEmailCandidate(username, accountEmail, billingEmail) != "" {
		return service.AuthProviderEmail
	}
	if strings.TrimSpace(authProvider) == "" {
		return service.AuthProviderLocal
	}
	return strings.TrimSpace(authProvider)
}

func (a *App) handleUserKeys(w http.ResponseWriter, r *http.Request) {
	identity, ok := a.requireIdentity(w, r, "")
	if !ok {
		return
	}
	filter, owner, canManage := userKeyScope(identity)
	if !canManage {
		util.WriteError(w, http.StatusForbidden, "Linuxdo login or admin permission required")
		return
	}
	base := "/api/auth/users"
	if r.URL.Path == base {
		switch r.Method {
		case http.MethodGet:
			items := a.auth.ListKeys(filter)
			if identity.Role != service.AuthRoleAdmin {
				items = a.auth.ListSingleAPIKeyForOwner(identity.OwnerID)
			}
			util.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
		case http.MethodPost:
			body, _ := readJSONMap(r)
			var item map[string]any
			var raw string
			var err error
			if identity.Role == service.AuthRoleAdmin {
				item, raw, err = a.auth.CreateAPIKey(service.AuthRoleUser, util.Clean(body["name"]), owner)
			} else {
				item, raw, err = a.auth.UpsertAPIKeyForOwner(util.Clean(body["name"]), owner)
			}
			if err != nil {
				util.WriteError(w, http.StatusBadRequest, err.Error())
				return
			}
			util.WriteJSON(w, http.StatusOK, map[string]any{"item": item, "key": raw, "items": a.auth.ListKeys(filter)})
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
		return
	}
	parts := splitPath(r.URL.Path)
	if len(parts) < 4 || parts[0] != "api" || parts[1] != "auth" || parts[2] != "users" {
		http.NotFound(w, r)
		return
	}
	keyID := parts[3]
	if len(parts) == 5 && parts[4] == "key" {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		key, found := a.auth.RevealKey(keyID, filter)
		if !found {
			util.WriteError(w, http.StatusNotFound, "user key not found")
			return
		}
		util.WriteJSON(w, http.StatusOK, map[string]any{"key": key})
		return
	}
	if len(parts) != 4 {
		http.NotFound(w, r)
		return
	}
	switch r.Method {
	case http.MethodPost:
		body, _ := readJSONMap(r)
		updates := map[string]any{}
		if value, ok := body["name"]; ok {
			updates["name"] = value
		}
		if value, ok := body["enabled"]; ok {
			updates["enabled"] = value
		}
		if len(updates) == 0 {
			util.WriteError(w, http.StatusBadRequest, "no updates provided")
			return
		}
		item := a.auth.UpdateKey(keyID, updates, filter)
		if item == nil {
			util.WriteError(w, http.StatusNotFound, "user key not found")
			return
		}
		util.WriteJSON(w, http.StatusOK, map[string]any{"item": item, "items": a.auth.ListKeys(filter)})
	case http.MethodDelete:
		if !a.auth.DeleteKey(keyID, filter) {
			util.WriteError(w, http.StatusNotFound, "user key not found")
			return
		}
		util.WriteJSON(w, http.StatusOK, map[string]any{"items": a.auth.ListKeys(filter)})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func userKeyScope(identity service.Identity) (service.AuthKeyFilter, service.AuthOwner, bool) {
	filter := service.AuthKeyFilter{Role: service.AuthRoleUser, Kind: service.AuthKindAPIKey}
	if identity.Role == service.AuthRoleAdmin {
		return filter, service.AuthOwner{}, true
	}
	if identity.Role != service.AuthRoleUser || identity.OwnerID == "" {
		return service.AuthKeyFilter{}, service.AuthOwner{}, false
	}
	filter.OwnerID = identity.OwnerID
	return filter, service.AuthOwner{ID: identity.OwnerID, Name: identity.Name, Provider: identity.Provider}, true
}

func (a *App) handleProfile(w http.ResponseWriter, r *http.Request) {
	identity, ok := a.requireIdentity(w, r, "")
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		a.writeLoginResponse(w, identity, "")
	case http.MethodPost:
		body, err := readJSONMap(r)
		if err != nil {
			util.WriteError(w, http.StatusBadRequest, "invalid json body")
			return
		}
		updated, err := a.auth.UpdateProfileName(identity, util.Clean(body["name"]))
		if err != nil {
			util.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		a.writeLoginResponse(w, *updated, "")
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (a *App) handleProfilePassword(w http.ResponseWriter, r *http.Request) {
	identity, ok := a.requireIdentity(w, r, "")
	if !ok {
		return
	}
	body, err := readJSONMap(r)
	if err != nil {
		util.WriteError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if err := a.auth.ChangeProfilePassword(identity, util.Clean(body["current_password"]), util.Clean(body["new_password"])); err != nil {
		util.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	util.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (a *App) handleProfileAPIKey(w http.ResponseWriter, r *http.Request) {
	identity, ok := a.requireIdentity(w, r, "")
	if !ok {
		return
	}
	filter, ok := profileAPIKeyFilter(identity)
	if !ok {
		util.WriteError(w, http.StatusForbidden, "profile API key requires a bound user account")
		return
	}
	base := "/api/profile/api-key"
	if r.URL.Path == base {
		switch r.Method {
		case http.MethodGet:
			util.WriteJSON(w, http.StatusOK, map[string]any{"items": a.auth.ListPersonalAPIKey(identity)})
		case http.MethodPost:
			body, _ := readJSONMap(r)
			item, raw, err := a.auth.UpsertPersonalAPIKey(identity, util.Clean(body["name"]))
			if err != nil {
				util.WriteError(w, http.StatusBadRequest, err.Error())
				return
			}
			util.WriteJSON(w, http.StatusOK, map[string]any{"item": item, "key": raw, "items": a.auth.ListPersonalAPIKey(identity)})
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
		return
	}

	parts := splitPath(r.URL.Path)
	if len(parts) < 4 || parts[0] != "api" || parts[1] != "profile" || parts[2] != "api-key" {
		http.NotFound(w, r)
		return
	}
	keyID := parts[3]
	if len(parts) == 5 && parts[4] == "key" {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		key, found := a.auth.RevealKey(keyID, filter)
		if !found {
			util.WriteError(w, http.StatusNotFound, "profile API key not found")
			return
		}
		util.WriteJSON(w, http.StatusOK, map[string]any{"key": key})
		return
	}
	if len(parts) != 4 {
		http.NotFound(w, r)
		return
	}
	switch r.Method {
	case http.MethodPost:
		body, _ := readJSONMap(r)
		updates := map[string]any{}
		if value, ok := body["name"]; ok {
			updates["name"] = value
		}
		if value, ok := body["enabled"]; ok {
			updates["enabled"] = value
		}
		if len(updates) == 0 {
			util.WriteError(w, http.StatusBadRequest, "no updates provided")
			return
		}
		item := a.auth.UpdateKey(keyID, updates, filter)
		if item == nil {
			util.WriteError(w, http.StatusNotFound, "profile API key not found")
			return
		}
		util.WriteJSON(w, http.StatusOK, map[string]any{"item": item, "items": a.auth.ListPersonalAPIKey(identity)})
	case http.MethodDelete:
		if !a.auth.DeleteKey(keyID, filter) {
			util.WriteError(w, http.StatusNotFound, "profile API key not found")
			return
		}
		util.WriteJSON(w, http.StatusOK, map[string]any{"items": a.auth.ListPersonalAPIKey(identity)})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func profileAPIKeyFilter(identity service.Identity) (service.AuthKeyFilter, bool) {
	role := identity.Role
	if role != service.AuthRoleAdmin && role != service.AuthRoleUser {
		return service.AuthKeyFilter{}, false
	}
	ownerID := util.Clean(identity.OwnerID)
	if ownerID == "" {
		return service.AuthKeyFilter{}, false
	}
	return service.AuthKeyFilter{Role: role, Kind: service.AuthKindAPIKey, OwnerID: ownerID}, true
}

func (a *App) handleProfilePromptFavorites(w http.ResponseWriter, r *http.Request) {
	identity, ok := a.requireIdentity(w, r, "")
	if !ok {
		return
	}
	ownerID := util.Clean(identity.OwnerID)
	if ownerID == "" {
		util.WriteError(w, http.StatusForbidden, "prompt favorites require a bound user account")
		return
	}

	base := "/api/profile/prompt-favorites"
	if r.URL.Path == base {
		switch r.Method {
		case http.MethodGet:
			util.WriteJSON(w, http.StatusOK, map[string]any{"items": a.prompts.List(ownerID)})
		case http.MethodPost:
			body, err := readJSONMap(r)
			if err != nil {
				util.WriteError(w, http.StatusBadRequest, "invalid json body")
				return
			}
			item, err := a.prompts.Upsert(ownerID, body)
			if err != nil {
				util.WriteError(w, http.StatusBadRequest, err.Error())
				return
			}
			util.WriteJSON(w, http.StatusOK, map[string]any{"item": item, "items": a.prompts.List(ownerID)})
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
		return
	}

	parts := splitPath(r.URL.Path)
	if len(parts) != 4 || parts[0] != "api" || parts[1] != "profile" || parts[2] != "prompt-favorites" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodDelete {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if !a.prompts.Delete(ownerID, parts[3]) {
		util.WriteError(w, http.StatusNotFound, "prompt favorite not found")
		return
	}
	util.WriteJSON(w, http.StatusOK, map[string]any{"items": a.prompts.List(ownerID)})
}

func (a *App) handleAdminRoles(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireIdentity(w, r, ""); !ok {
		return
	}
	a.ensureAgencyTierRoles()
	a.ensureSubscriptionTierRoles()
	base := "/api/admin/roles"
	if r.URL.Path == base {
		switch r.Method {
		case http.MethodGet:
			util.WriteJSON(w, http.StatusOK, map[string]any{"items": a.managedRolesWithSubscriptionCounts(a.auth.ListRoles())})
		case http.MethodPost:
			body, _ := readJSONMap(r)
			item, err := a.auth.CreateRole(body)
			if err != nil {
				util.WriteError(w, http.StatusBadRequest, err.Error())
				return
			}
			util.WriteJSON(w, http.StatusOK, map[string]any{"item": item, "items": a.managedRolesWithSubscriptionCounts(a.auth.ListRoles())})
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
		return
	}

	parts := splitPath(r.URL.Path)
	if len(parts) != 4 || parts[0] != "api" || parts[1] != "admin" || parts[2] != "roles" {
		http.NotFound(w, r)
		return
	}
	roleID := parts[3]
	switch r.Method {
	case http.MethodPost:
		body, _ := readJSONMap(r)
		item, err := a.auth.UpdateRole(roleID, body)
		if err != nil {
			status := http.StatusBadRequest
			if err.Error() == "role not found" {
				status = http.StatusNotFound
			}
			util.WriteError(w, status, err.Error())
			return
		}
		util.WriteJSON(w, http.StatusOK, map[string]any{"item": item, "items": a.managedRolesWithSubscriptionCounts(a.auth.ListRoles())})
	case http.MethodDelete:
		deleted, err := a.auth.DeleteRole(roleID)
		if err != nil {
			status := http.StatusBadRequest
			if err.Error() == "role is assigned to users" {
				status = http.StatusConflict
			}
			util.WriteError(w, status, err.Error())
			return
		}
		if !deleted {
			util.WriteError(w, http.StatusNotFound, "role not found")
			return
		}
		util.WriteJSON(w, http.StatusOK, map[string]any{"items": a.managedRolesWithSubscriptionCounts(a.auth.ListRoles())})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (a *App) handleAdminUsers(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireIdentity(w, r, ""); !ok {
		return
	}
	base := "/api/admin/users"
	if r.URL.Path == base {
		switch r.Method {
		case http.MethodGet:
			items := a.managedUsers(util.ToBool(r.URL.Query().Get("compact")))
			if util.ToBool(r.URL.Query().Get("billing_only")) {
				filtered := make([]map[string]any, 0, len(items))
				for _, item := range items {
					if util.ToBool(item["billing_user"]) || util.Clean(item["provider"]) == service.AuthProviderLocal || util.Clean(item["provider"]) == service.AuthProviderEmail {
						filtered = append(filtered, item)
					}
				}
				items = filtered
			}
			util.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
		case http.MethodPost:
			body, err := readJSONMap(r)
			if err != nil {
				util.WriteError(w, http.StatusBadRequest, "invalid json body")
				return
			}
			enabled := true
			if value, ok := body["enabled"]; ok {
				enabled = util.ToBool(value)
			}
			item, err := a.auth.CreatePasswordUser(
				util.Clean(body["username"]),
				util.Clean(body["password"]),
				util.Clean(body["name"]),
				util.Clean(body["role_id"]),
				enabled,
			)
			if err != nil {
				util.WriteError(w, http.StatusBadRequest, err.Error())
				return
			}
			userID := util.Clean(item["id"])
			if userID != "" {
				a.billing.EnsureWalletUserWithEmail(
					userID,
					util.Clean(item["email"]),
					util.Clean(item["name"]),
					service.AuthProviderLocal,
				)
			}
			items := a.managedUsers(false)
			if current := findManagedUser(items, util.Clean(item["id"])); current != nil {
				item = current
			}
			util.WriteJSON(w, http.StatusOK, map[string]any{"item": item, "items": items})
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
		return
	}

	parts := splitPath(r.URL.Path)
	if len(parts) < 4 || parts[0] != "api" || parts[1] != "admin" || parts[2] != "users" {
		http.NotFound(w, r)
		return
	}
	userID := parts[3]
	if len(parts) == 5 && parts[4] == "key" {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		user := findManagedUser(a.auth.ListUsers(), userID)
		if user == nil {
			util.WriteError(w, http.StatusNotFound, "user not found")
			return
		}
		if util.Clean(user["provider"]) == service.AuthProviderLinuxDo {
			util.WriteError(w, http.StatusForbidden, "Linuxdo user tokens are not managed by administrators")
			return
		}
		key, found := a.auth.RevealUserAPIKey(userID)
		if !found {
			util.WriteError(w, http.StatusNotFound, "user API key not found")
			return
		}
		util.WriteJSON(w, http.StatusOK, map[string]any{"key": key})
		return
	}
	if len(parts) == 5 && parts[4] == "reset-key" {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		body, _ := readJSONMap(r)
		user := findManagedUser(a.auth.ListUsers(), userID)
		if user == nil {
			util.WriteError(w, http.StatusNotFound, "user not found")
			return
		}
		if util.Clean(user["provider"]) == service.AuthProviderLinuxDo {
			util.WriteError(w, http.StatusForbidden, "Linuxdo user tokens are not managed by administrators")
			return
		}
		item, apiKey, raw, found, err := a.auth.ResetUserAPIKey(userID, util.Clean(body["name"]))
		if err != nil {
			util.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		if !found {
			util.WriteError(w, http.StatusNotFound, "user not found")
			return
		}
		items := a.managedUsers(false)
		if current := findManagedUser(items, userID); current != nil {
			item = current
		}
		util.WriteJSON(w, http.StatusOK, map[string]any{"item": item, "api_key": apiKey, "key": raw, "items": items})
		return
	}
	if len(parts) == 5 && parts[4] == "password" {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		body, err := readJSONMap(r)
		if err != nil {
			util.WriteError(w, http.StatusBadRequest, "invalid json body")
			return
		}
		if err := a.auth.AdminResetPasswordByUserID(userID, util.Clean(body["password"])); err != nil {
			status := http.StatusBadRequest
			if err.Error() == "user not found" {
				status = http.StatusNotFound
			}
			util.WriteError(w, status, err.Error())
			return
		}
		util.WriteJSON(w, http.StatusOK, map[string]any{"ok": true, "items": a.managedUsers(false)})
		return
	}
	if len(parts) != 4 {
		http.NotFound(w, r)
		return
	}
	switch r.Method {
	case http.MethodPost:
		body, _ := readJSONMap(r)
		updates := map[string]any{}
		if value, ok := body["name"]; ok {
			updates["name"] = value
		}
		if value, ok := body["enabled"]; ok {
			updates["enabled"] = value
		}
		if value, ok := body["role_id"]; ok {
			if roleID := util.Clean(value); roleID != "" && !a.auth.RoleExists(roleID) {
				util.WriteError(w, http.StatusBadRequest, "role not found")
				return
			}
			updates["role_id"] = value
		}
		if len(updates) == 0 {
			util.WriteError(w, http.StatusBadRequest, "no updates provided")
			return
		}
		item := a.auth.UpdateUser(userID, updates)
		if item == nil {
			util.WriteError(w, http.StatusNotFound, "user not found")
			return
		}
		if _, hasRoleID := updates["role_id"]; hasRoleID {
			a.syncAgencyByRoleID(userID, util.Clean(item["role_id"]))
		}
		items := a.managedUsers(false)
		if current := findManagedUser(items, userID); current != nil {
			item = current
		}
		util.WriteJSON(w, http.StatusOK, map[string]any{"item": item, "items": items})
	case http.MethodDelete:
		if !a.auth.DeleteUser(userID) {
			util.WriteError(w, http.StatusNotFound, "user not found")
			return
		}
		util.WriteJSON(w, http.StatusOK, map[string]any{"items": a.managedUsers(false)})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (a *App) managedUsers(compact bool) []map[string]any {
	items := a.auth.ListUsers()
	billingUsers := map[string]map[string]any{}
	for _, item := range a.billing.ListUsersForAdmin() {
		id := util.Clean(item["id"])
		if id != "" {
			billingUsers[id] = item
		}
	}
	stats := map[string]map[string]any{}
	if !compact {
		stats = a.logs.UserUsageStats(14)
	}
	for _, item := range items {
		userID := util.Clean(item["id"])
		item["has_password"] = a.auth.HasPasswordAccountByUserID(userID)
		if userID != "" && util.Clean(item["provider"]) == service.AuthProviderLocal {
			providerHint := inferManagedUserProvider(
				util.Clean(item["provider"]),
				util.Clean(item["username"]),
				util.Clean(item["email"]),
				"",
				"",
			)
			if wallet := a.billing.EnsureWalletUserWithEmail(
				userID,
				util.Clean(item["email"]),
				util.Clean(item["name"]),
				providerHint,
			); wallet != nil {
				billingUsers[userID] = wallet
			}
		}
		if !compact {
			usage := stats[userID]
			if usage == nil {
				usage = service.ZeroUserUsageStats(14)
			}
			for key, value := range usage {
				item[key] = value
			}
		}
		if billing, exists := billingUsers[userID]; exists {
			email := managedUserEmailCandidate(
				util.Clean(billing["email"]),
				util.Clean(item["email"]),
				util.Clean(item["username"]),
			)
			if email != "" {
				item["email"] = email
			} else if isLocalInvalidEmail(util.Clean(item["email"])) {
				item["email"] = ""
			}
			item["provider"] = inferManagedUserProvider(
				util.Clean(item["provider"]),
				util.Clean(item["username"]),
				util.Clean(item["email"]),
				util.Clean(billing["provider"]),
				util.Clean(billing["email"]),
			)
			item["balance_cents"] = util.ToInt(billing["balance_cents"], 0)
			item["total_recharge_cents"] = util.ToInt(billing["total_recharge_cents"], 0)
			item["total_consume_cents"] = util.ToInt(billing["total_consume_cents"], 0)
			item["subscription_tier"] = util.Clean(billing["subscription_tier"])
			item["subscription_start_at"] = util.Clean(billing["subscription_start_at"])
			item["subscription_expire_at"] = util.Clean(billing["subscription_expire_at"])
			item["subscription_active"] = util.ToBool(billing["subscription_active"])
			item["subscription_remaining_days"] = subscriptionRemainingDays(util.Clean(billing["subscription_expire_at"]), util.ToBool(billing["subscription_active"]))
			item["billing_user"] = true
			item["image_price_cents"] = a.config.ImagePriceCents()
			continue
		}
		if email := managedUserEmailCandidate(util.Clean(item["email"]), util.Clean(item["username"])); email != "" {
			item["email"] = email
		} else if isLocalInvalidEmail(util.Clean(item["email"])) {
			item["email"] = ""
		}
		item["provider"] = inferManagedUserProvider(
			util.Clean(item["provider"]),
			util.Clean(item["username"]),
			util.Clean(item["email"]),
			"",
			"",
		)
		item["balance_cents"] = 0
		item["total_recharge_cents"] = 0
		item["total_consume_cents"] = 0
		item["subscription_tier"] = ""
		item["subscription_start_at"] = ""
		item["subscription_expire_at"] = ""
		item["subscription_active"] = false
		item["subscription_remaining_days"] = 0
		item["billing_user"] = false
		item["image_price_cents"] = a.config.ImagePriceCents()
	}
	return items
}

func (a *App) managedRolesWithSubscriptionCounts(roles []map[string]any) []map[string]any {
	activeByTier := map[string]int{}
	for _, user := range a.billing.ListUsersForAdmin() {
		if !util.ToBool(user["subscription_active"]) {
			continue
		}
		tier := strings.TrimSpace(util.Clean(user["subscription_tier"]))
		if tier == "" {
			continue
		}
		activeByTier[tier]++
	}
	roleTierByID := map[string]string{}
	for _, tier := range []string{service.SubscriptionTierMonthly, service.SubscriptionTierQuarterly, service.SubscriptionTierYearly} {
		if roleID, ok := a.subscriptionRoleIDByTier(tier); ok {
			roleTierByID[roleID] = tier
		}
	}
	out := make([]map[string]any, 0, len(roles))
	for _, role := range roles {
		item := util.CopyMap(role)
		roleID := strings.TrimSpace(util.Clean(item["id"]))
		if tier := roleTierByID[roleID]; tier != "" {
			item["subscription_tier"] = tier
			item["subscription_active_user_count"] = activeByTier[tier]
			item["user_count"] = activeByTier[tier]
		}
		out = append(out, item)
	}
	return out
}

func subscriptionRemainingDays(expireAt string, active bool) int {
	if !active {
		return 0
	}
	expire := parseFlexibleTime(expireAt)
	if expire.IsZero() {
		return 0
	}
	duration := expire.Sub(time.Now().UTC())
	if duration <= 0 {
		return 0
	}
	days := int(duration.Hours() / 24)
	if duration > time.Duration(days)*24*time.Hour {
		days++
	}
	return days
}

func (a *App) decorateAdminBillingOrderUsers(items []map[string]any) {
	if len(items) == 0 {
		return
	}
	billingUsersByID := map[string]map[string]any{}
	for _, user := range a.billing.ListUsersForAdmin() {
		if id := strings.TrimSpace(util.Clean(user["id"])); id != "" {
			billingUsersByID[id] = user
		}
	}
	usersByID := map[string]map[string]any{}
	for _, user := range a.managedUsers(false) {
		if id := strings.TrimSpace(util.Clean(user["id"])); id != "" {
			usersByID[id] = user
		}
	}
	for _, item := range items {
		if item == nil {
			continue
		}
		userID := strings.TrimSpace(util.Clean(item["user_id"]))
		rawEmail := util.Clean(item["user_email"])
		email := managedUserEmailCandidate(rawEmail)
		display := ""
		if billingUser := billingUsersByID[userID]; billingUser != nil {
			email = managedUserEmailCandidate(util.Clean(billingUser["email"]), rawEmail)
			display = firstNonEmpty(util.Clean(billingUser["name"]), email, userID)
		}
		if user := usersByID[userID]; user != nil {
			email = managedUserEmailCandidate(
				util.Clean(user["email"]),
				util.Clean(user["username"]),
				email,
				rawEmail,
			)
			display = firstNonEmpty(
				util.Clean(user["name"]),
				email,
				util.Clean(user["username"]),
				userID,
			)
		}
		if email == "" && isLocalInvalidEmail(rawEmail) {
			item["user_email"] = ""
		} else {
			item["user_email"] = email
		}
		item["user_display"] = firstNonEmpty(display, email, userID)
	}
}

func findManagedUser(items []map[string]any, id string) map[string]any {
	for _, item := range items {
		if item["id"] == id {
			return item
		}
	}
	return nil
}

func isAgencyRoleID(roles []map[string]any, roleID string) bool {
	roleID = strings.TrimSpace(roleID)
	if roleID == "" || roleID == service.AuthRoleAdmin || roleID == service.DefaultManagedRoleID {
		return false
	}
	for _, role := range roles {
		if strings.TrimSpace(util.Clean(role["id"])) != roleID {
			continue
		}
		name := strings.TrimSpace(util.Clean(role["name"]))
		if strings.Contains(name, "代理") {
			return true
		}
		return false
	}
	return false
}

func (a *App) handleAdminBilling(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireIdentity(w, r, ""); !ok {
		return
	}
	parts := splitPath(r.URL.Path)
	if len(parts) < 4 || parts[0] != "api" || parts[1] != "admin" || parts[2] != "billing" {
		http.NotFound(w, r)
		return
	}

	if len(parts) >= 4 && parts[3] == "users" {
		if len(parts) == 4 {
			if r.Method != http.MethodGet {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			util.WriteJSON(w, http.StatusOK, map[string]any{"items": a.billing.ListUsersForAdmin()})
			return
		}
		if len(parts) == 6 && parts[5] == "balance" {
			userID, decodeErr := url.PathUnescape(parts[4])
			if decodeErr != nil {
				util.WriteError(w, http.StatusBadRequest, "invalid user id")
				return
			}
			if r.Method != http.MethodPost {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			body, err := readJSONMap(r)
			if err != nil {
				util.WriteError(w, http.StatusBadRequest, "invalid json body")
				return
			}
			note := strings.TrimSpace(util.Clean(body["note"]))
			var (
				wallet    map[string]any
				adjustErr error
			)
			if _, ok := body["balance_cents"]; ok {
				wallet, adjustErr = a.billing.AdminSetUserBalance(userID, util.ToInt(body["balance_cents"], 0), note)
			} else {
				wallet, adjustErr = a.billing.AdminAdjustUserBalance(userID, util.ToInt(body["delta_cents"], 0), note)
			}
			if adjustErr != nil {
				util.WriteError(w, http.StatusBadRequest, adjustErr.Error())
				return
			}
			util.WriteJSON(w, http.StatusOK, map[string]any{
				"wallet": wallet,
				"items":  a.managedUsers(false),
			})
			return
		}
		if len(parts) == 6 && parts[5] == "subscription" {
			userID, decodeErr := url.PathUnescape(parts[4])
			if decodeErr != nil {
				util.WriteError(w, http.StatusBadRequest, "invalid user id")
				return
			}
			if r.Method != http.MethodPost {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			body, err := readJSONMap(r)
			if err != nil {
				util.WriteError(w, http.StatusBadRequest, "invalid json body")
				return
			}
			mode := strings.TrimSpace(util.Clean(body["mode"]))
			tier := strings.TrimSpace(util.Clean(body["tier"]))
			expireAt := strings.TrimSpace(util.Clean(body["expire_at"]))
			extendDays := util.ToInt(body["extend_days"], 0)
			status, updateErr := a.billing.AdminUpdateSubscriptionByUserID(userID, mode, tier, expireAt, extendDays)
			if updateErr != nil {
				util.WriteError(w, http.StatusBadRequest, updateErr.Error())
				return
			}
			if mode != "clear" {
				effectiveTier := strings.TrimSpace(util.Clean(status["tier"]))
				if effectiveTier != "" {
					a.switchSubscriptionRoleByTier(userID, effectiveTier, "Admin adjust")
				}
			}
			util.WriteJSON(w, http.StatusOK, map[string]any{
				"status": status,
				"items":  a.managedUsers(false),
			})
			return
		}
		http.NotFound(w, r)
		return
	}

	if len(parts) >= 5 && parts[3] == "subscriptions" && parts[4] == "report" {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		query := r.URL.Query()
		statusFilter := strings.ToLower(strings.TrimSpace(query.Get("status")))
		if statusFilter == "" {
			statusFilter = service.BillingOrderStatusPaid
		}
		tierFilter := strings.ToLower(strings.TrimSpace(query.Get("tier")))
		startAt, startErr := parseSubscriptionReportDateParam(query.Get("start_at"), false)
		if startErr != nil {
			util.WriteError(w, http.StatusBadRequest, startErr.Error())
			return
		}
		endAt, endErr := parseSubscriptionReportDateParam(query.Get("end_at"), true)
		if endErr != nil {
			util.WriteError(w, http.StatusBadRequest, endErr.Error())
			return
		}
		items, _ := a.billing.ListOrdersForAdmin(0)
		a.decorateAdminBillingOrderUsers(items)
		report := buildSubscriptionReport(items, adminSubscriptionReportFilter{
			Status:  statusFilter,
			Tier:    tierFilter,
			StartAt: startAt,
			EndAt:   endAt,
		})
		if strings.EqualFold(strings.TrimSpace(query.Get("export")), "csv") {
			writeSubscriptionReportCSV(w, report)
			return
		}
		util.WriteJSON(w, http.StatusOK, report)
		return
	}

	if len(parts) >= 4 && parts[3] == "redeem-codes" {
		if len(parts) == 4 {
			switch r.Method {
			case http.MethodGet:
				limit := util.ToInt(r.URL.Query().Get("limit"), 200)
				util.WriteJSON(w, http.StatusOK, map[string]any{"items": a.billing.ListRedeemCodes(limit)})
			case http.MethodPost:
				body, err := readJSONMap(r)
				if err != nil {
					util.WriteError(w, http.StatusBadRequest, "invalid json body")
					return
				}
				amountCents := util.ToInt(body["amount_cents"], 0)
				if amountCents < 1 {
					if amountYuan := strings.TrimSpace(util.Clean(body["amount"])); amountYuan != "" {
						if parsed, parseErr := strconv.ParseFloat(amountYuan, 64); parseErr == nil {
							amountCents = int(parsed*100 + 0.5)
						}
					}
				}
				count := util.ToInt(body["count"], 1)
				expiresAt := strings.TrimSpace(util.Clean(body["expires_at"]))
				note := strings.TrimSpace(util.Clean(body["note"]))
				created, createErr := a.billing.CreateRedeemCodes(amountCents, count, expiresAt, note)
				if createErr != nil {
					util.WriteError(w, http.StatusBadRequest, createErr.Error())
					return
				}
				util.WriteJSON(w, http.StatusOK, map[string]any{
					"items":   a.billing.ListRedeemCodes(200),
					"created": created,
				})
			default:
				w.WriteHeader(http.StatusMethodNotAllowed)
			}
			return
		}
		if len(parts) == 5 {
			code, decodeErr := url.PathUnescape(parts[4])
			if decodeErr != nil {
				util.WriteError(w, http.StatusBadRequest, "invalid redeem code")
				return
			}
			switch r.Method {
			case http.MethodPost:
				body, err := readJSONMap(r)
				if err != nil {
					util.WriteError(w, http.StatusBadRequest, "invalid json body")
					return
				}
				var enabledPtr *bool
				if value, ok := body["enabled"]; ok {
					enabledValue := util.ToBool(value)
					enabledPtr = &enabledValue
				}
				var notePtr *string
				if value, ok := body["note"]; ok {
					noteValue := util.Clean(value)
					notePtr = &noteValue
				}
				var expiresPtr *string
				if value, ok := body["expires_at"]; ok {
					expiresValue := util.Clean(value)
					expiresPtr = &expiresValue
				}
				item, updateErr := a.billing.UpdateRedeemCode(code, enabledPtr, notePtr, expiresPtr)
				if updateErr != nil {
					util.WriteError(w, http.StatusBadRequest, updateErr.Error())
					return
				}
				util.WriteJSON(w, http.StatusOK, map[string]any{
					"item":  item,
					"items": a.billing.ListRedeemCodes(200),
				})
			case http.MethodDelete:
				if !a.billing.DeleteRedeemCode(code) {
					util.WriteError(w, http.StatusNotFound, "redeem code not found")
					return
				}
				util.WriteJSON(w, http.StatusOK, map[string]any{"items": a.billing.ListRedeemCodes(200)})
			default:
				w.WriteHeader(http.StatusMethodNotAllowed)
			}
			return
		}
		http.NotFound(w, r)
		return
	}

	if len(parts) >= 4 && parts[3] == "orders" {
		if len(parts) != 4 {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		query := r.URL.Query()
		limit := util.ToInt(query.Get("limit"), 0)
		items, stats := a.billing.ListOrdersForAdmin(limit)
		a.decorateAdminBillingOrderUsers(items)
		statusFilter := strings.ToLower(strings.TrimSpace(query.Get("status")))
		kindFilter := strings.ToLower(strings.TrimSpace(query.Get("order_kind")))
		userKeyword := strings.ToLower(strings.TrimSpace(query.Get("user_keyword")))
		filtered := make([]map[string]any, 0, len(items))
		for _, item := range items {
			if statusFilter != "" && statusFilter != "all" && strings.ToLower(strings.TrimSpace(util.Clean(item["status"]))) != statusFilter {
				continue
			}
			if kindFilter != "" && kindFilter != "all" && strings.ToLower(strings.TrimSpace(util.Clean(item["order_kind"]))) != kindFilter {
				continue
			}
			if userKeyword != "" {
				candidate := strings.ToLower(strings.TrimSpace(
					firstNonEmpty(
						util.Clean(item["user_email"]),
						util.Clean(item["user_id"]),
						util.Clean(item["out_trade_no"]),
					),
				))
				if !strings.Contains(candidate, userKeyword) {
					continue
				}
			}
			filtered = append(filtered, item)
		}
		pageSize := util.ToInt(query.Get("page_size"), 10)
		if pageSize < 1 {
			pageSize = 10
		}
		if pageSize > 500 {
			pageSize = 500
		}
		page := util.ToInt(query.Get("page"), 1)
		if page < 1 {
			page = 1
		}
		total := len(filtered)
		totalPages := 1
		if total > 0 {
			totalPages = (total + pageSize - 1) / pageSize
		}
		if page > totalPages {
			page = totalPages
		}
		start := (page - 1) * pageSize
		if start < 0 {
			start = 0
		}
		if start > total {
			start = total
		}
		end := start + pageSize
		if end > total {
			end = total
		}
		util.WriteJSON(w, http.StatusOK, map[string]any{
			"items":      filtered[start:end],
			"stats":      stats,
			"limit":      limit,
			"scope":      "all_time",
			"source":     "billing_orders",
			"total":      total,
			"page":       page,
			"page_size":  pageSize,
			"total_page": totalPages,
		})
		return
	}

	http.NotFound(w, r)
}

type adminSubscriptionTierSummary struct {
	Orders         int
	RevenueCents   int
	NewSubscribers int
	Renewals       int
}

type adminSubscriptionReportFilter struct {
	Status  string
	Tier    string
	StartAt time.Time
	EndAt   time.Time
}

func buildSubscriptionReport(items []map[string]any, filter adminSubscriptionReportFilter) map[string]any {
	statusFilter := strings.ToLower(strings.TrimSpace(filter.Status))
	if statusFilter == "" {
		statusFilter = service.BillingOrderStatusPaid
	}
	tierFilter := strings.ToLower(strings.TrimSpace(filter.Tier))
	tierStats := map[string]*adminSubscriptionTierSummary{
		service.SubscriptionTierMonthly:   {},
		service.SubscriptionTierQuarterly: {},
		service.SubscriptionTierYearly:    {},
	}
	subscriptionItems := make([]map[string]any, 0)
	for _, item := range items {
		kind := strings.ToLower(strings.TrimSpace(util.Clean(item["order_kind"])))
		if kind != service.BillingOrderKindSubMonthly && kind != service.BillingOrderKindSubQuarterly && kind != service.BillingOrderKindSubYearly {
			continue
		}
		status := strings.ToLower(strings.TrimSpace(util.Clean(item["status"])))
		if statusFilter != "all" && status != statusFilter {
			continue
		}
		createdAt := serviceTime(item)
		if !filter.StartAt.IsZero() && createdAt.Before(filter.StartAt) {
			continue
		}
		if !filter.EndAt.IsZero() && createdAt.After(filter.EndAt) {
			continue
		}
		copyItem := util.CopyMap(item)
		subscriptionItems = append(subscriptionItems, copyItem)
	}
	sortSubscriptionOrders(subscriptionItems)
	seenUserPaid := map[string]int{}
	totalRevenue := 0
	totalOrders := 0
	for _, item := range subscriptionItems {
		tier := strings.TrimSpace(util.Clean(item["subscription_tier"]))
		if tier == "" {
			switch strings.ToLower(strings.TrimSpace(util.Clean(item["order_kind"]))) {
			case service.BillingOrderKindSubMonthly:
				tier = service.SubscriptionTierMonthly
			case service.BillingOrderKindSubQuarterly:
				tier = service.SubscriptionTierQuarterly
			case service.BillingOrderKindSubYearly:
				tier = service.SubscriptionTierYearly
			}
		}
		if tierFilter != "" && tierFilter != "all" && tier != tierFilter {
			continue
		}
		entry, ok := tierStats[tier]
		if !ok {
			entry = &adminSubscriptionTierSummary{}
			tierStats[tier] = entry
		}
		amount := util.ToInt(item["amount_cents"], 0)
		entry.Orders++
		entry.RevenueCents += amount
		totalOrders++
		totalRevenue += amount
		userID := strings.TrimSpace(util.Clean(item["user_id"]))
		if seenUserPaid[userID] > 0 {
			entry.Renewals++
		} else {
			entry.NewSubscribers++
		}
		seenUserPaid[userID]++
	}
	return map[string]any{
		"summary": map[string]any{
			"orders":          totalOrders,
			"revenue_cents":   totalRevenue,
			"revenue_yuan":    fmt.Sprintf("%.2f", float64(totalRevenue)/100.0),
			"renewal_orders":  countRenewals(tierStats),
			"new_subscribers": countNewSubscribers(tierStats),
			"paid_user_count": len(seenUserPaid),
			"generated_at":    time.Now().Format(time.RFC3339Nano),
			"filter_status":   statusFilter,
			"filter_tier":     tierFilter,
			"filter_start_at": formatOptionalTime(filter.StartAt),
			"filter_end_at":   formatOptionalTime(filter.EndAt),
		},
		"tiers": map[string]any{
			service.SubscriptionTierMonthly: map[string]any{
				"orders":          tierStats[service.SubscriptionTierMonthly].Orders,
				"revenue_cents":   tierStats[service.SubscriptionTierMonthly].RevenueCents,
				"revenue_yuan":    fmt.Sprintf("%.2f", float64(tierStats[service.SubscriptionTierMonthly].RevenueCents)/100.0),
				"new_subscribers": tierStats[service.SubscriptionTierMonthly].NewSubscribers,
				"renewals":        tierStats[service.SubscriptionTierMonthly].Renewals,
			},
			service.SubscriptionTierQuarterly: map[string]any{
				"orders":          tierStats[service.SubscriptionTierQuarterly].Orders,
				"revenue_cents":   tierStats[service.SubscriptionTierQuarterly].RevenueCents,
				"revenue_yuan":    fmt.Sprintf("%.2f", float64(tierStats[service.SubscriptionTierQuarterly].RevenueCents)/100.0),
				"new_subscribers": tierStats[service.SubscriptionTierQuarterly].NewSubscribers,
				"renewals":        tierStats[service.SubscriptionTierQuarterly].Renewals,
			},
			service.SubscriptionTierYearly: map[string]any{
				"orders":          tierStats[service.SubscriptionTierYearly].Orders,
				"revenue_cents":   tierStats[service.SubscriptionTierYearly].RevenueCents,
				"revenue_yuan":    fmt.Sprintf("%.2f", float64(tierStats[service.SubscriptionTierYearly].RevenueCents)/100.0),
				"new_subscribers": tierStats[service.SubscriptionTierYearly].NewSubscribers,
				"renewals":        tierStats[service.SubscriptionTierYearly].Renewals,
			},
		},
		"items": subscriptionItems,
	}
}

func countRenewals(stats map[string]*adminSubscriptionTierSummary) int {
	total := 0
	for _, item := range stats {
		total += item.Renewals
	}
	return total
}

func countNewSubscribers(stats map[string]*adminSubscriptionTierSummary) int {
	total := 0
	for _, item := range stats {
		total += item.NewSubscribers
	}
	return total
}

func sortSubscriptionOrders(items []map[string]any) {
	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			left := serviceTime(items[i])
			right := serviceTime(items[j])
			if left.After(right) {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
}

func serviceTime(item map[string]any) time.Time {
	for _, key := range []string{"paid_at", "updated_at", "created_at"} {
		if parsed := parseFlexibleTime(util.Clean(item[key])); !parsed.IsZero() {
			return parsed
		}
	}
	return time.Time{}
}

func parseFlexibleTime(value string) time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}
	}
	if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return parsed
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed
	}
	if parsed, err := time.ParseInLocation("2006-01-02 15:04:05", value, time.Local); err == nil {
		return parsed
	}
	if parsed, err := time.ParseInLocation("2006-01-02", value, time.Local); err == nil {
		return parsed
	}
	return time.Time{}
}

func parseSubscriptionReportDateParam(value string, end bool) (time.Time, error) {
	text := strings.TrimSpace(value)
	if text == "" {
		return time.Time{}, nil
	}
	if onlyDate, err := time.ParseInLocation("2006-01-02", text, time.Local); err == nil {
		if end {
			return onlyDate.Add(24*time.Hour - time.Nanosecond), nil
		}
		return onlyDate, nil
	}
	parsed := parseFlexibleTime(text)
	if parsed.IsZero() {
		if end {
			return time.Time{}, fmt.Errorf("invalid end_at format")
		}
		return time.Time{}, fmt.Errorf("invalid start_at format")
	}
	return parsed, nil
}

func formatOptionalTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format(time.RFC3339Nano)
}

func writeSubscriptionReportCSV(w http.ResponseWriter, report map[string]any) {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="subscription-orders.csv"`)
	writer := csv.NewWriter(w)
	_ = writer.Write([]string{"order_id", "out_trade_no", "user_id", "user_email", "tier", "amount_cents", "amount_yuan", "status", "paid_at", "created_at"})
	for _, raw := range util.AsMapSlice(report["items"]) {
		amountCents := util.ToInt(raw["amount_cents"], 0)
		_ = writer.Write([]string{
			util.Clean(raw["id"]),
			util.Clean(raw["out_trade_no"]),
			util.Clean(raw["user_id"]),
			util.Clean(raw["user_email"]),
			util.Clean(raw["subscription_tier"]),
			strconv.Itoa(amountCents),
			fmt.Sprintf("%.2f", float64(amountCents)/100.0),
			util.Clean(raw["status"]),
			util.Clean(raw["paid_at"]),
			util.Clean(raw["created_at"]),
		})
	}
	writer.Flush()
}

func (a *App) handlePublicAnnouncements(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	util.WriteJSON(w, http.StatusOK, map[string]any{"items": a.announce.ListVisible(strings.TrimSpace(r.URL.Query().Get("target")))})
}

func (a *App) handleAdminAnnouncements(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireIdentity(w, r, ""); !ok {
		return
	}
	base := "/api/admin/announcements"
	if r.URL.Path == base {
		switch r.Method {
		case http.MethodGet:
			util.WriteJSON(w, http.StatusOK, map[string]any{"items": a.announce.ListAll()})
		case http.MethodPost:
			body, err := readJSONMap(r)
			if err != nil {
				util.WriteError(w, http.StatusBadRequest, "invalid json body")
				return
			}
			if util.Clean(body["content"]) == "" {
				util.WriteError(w, http.StatusBadRequest, "content is required")
				return
			}
			item := a.announce.Create(body)
			util.WriteJSON(w, http.StatusOK, map[string]any{"item": item, "items": a.announce.ListAll()})
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
		return
	}
	parts := splitPath(r.URL.Path)
	if len(parts) != 4 || parts[0] != "api" || parts[1] != "admin" || parts[2] != "announcements" {
		http.NotFound(w, r)
		return
	}
	id := parts[3]
	switch r.Method {
	case http.MethodPost:
		body, err := readJSONMap(r)
		if err != nil {
			util.WriteError(w, http.StatusBadRequest, "invalid json body")
			return
		}
		if value, exists := body["content"]; exists && util.Clean(value) == "" {
			util.WriteError(w, http.StatusBadRequest, "content is required")
			return
		}
		item := a.announce.Update(id, body)
		if item == nil {
			util.WriteError(w, http.StatusNotFound, "announcement not found")
			return
		}
		util.WriteJSON(w, http.StatusOK, map[string]any{"item": item, "items": a.announce.ListAll()})
	case http.MethodDelete:
		if !a.announce.Delete(id) {
			util.WriteError(w, http.StatusNotFound, "announcement not found")
			return
		}
		util.WriteJSON(w, http.StatusOK, map[string]any{"items": a.announce.ListAll()})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (a *App) handleAccounts(w http.ResponseWriter, r *http.Request) {
	identity, ok := a.requireIdentity(w, r, "")
	if !ok {
		return
	}
	switch {
	case r.URL.Path == "/api/accounts" && r.Method == http.MethodGet:
		util.WriteJSON(w, http.StatusOK, map[string]any{"items": a.accountItemsForIdentity(identity)})
	case r.URL.Path == "/api/accounts/tokens" && r.Method == http.MethodGet:
		util.WriteJSON(w, http.StatusOK, map[string]any{"tokens": a.accounts.ListTokens()})
	case r.URL.Path == "/api/accounts" && r.Method == http.MethodPost:
		body, _ := readJSONMap(r)
		tokens := util.AsStringSlice(body["tokens"])
		if len(tokens) == 0 {
			util.WriteError(w, http.StatusBadRequest, "tokens is required")
			return
		}
		result := a.accounts.AddAccounts(tokens)
		refresh := a.accounts.RefreshAccounts(r.Context(), tokens)
		for key, value := range refresh {
			if key == "refreshed" || key == "errors" || key == "items" {
				result[key] = value
			}
		}
		a.redactAccountPayloadForIdentity(identity, result)
		util.WriteJSON(w, http.StatusOK, result)
	case r.URL.Path == "/api/accounts" && r.Method == http.MethodDelete:
		body, _ := readJSONMap(r)
		tokens := util.AsStringSlice(body["tokens"])
		accountIDs := util.AsStringSlice(body["account_ids"])
		if len(tokens) == 0 {
			tokens = a.accounts.ListTokensByIDs(accountIDs)
		}
		if len(tokens) == 0 {
			if len(accountIDs) > 0 {
				util.WriteError(w, http.StatusNotFound, "account not found")
				return
			}
			util.WriteError(w, http.StatusBadRequest, "tokens or account_ids is required")
			return
		}
		result := a.accounts.DeleteAccounts(tokens)
		a.redactAccountPayloadForIdentity(identity, result)
		util.WriteJSON(w, http.StatusOK, result)
	case r.URL.Path == "/api/accounts/refresh" && r.Method == http.MethodPost:
		body, _ := readJSONMap(r)
		tokens := util.AsStringSlice(body["access_tokens"])
		accountIDs := util.AsStringSlice(body["account_ids"])
		if len(tokens) == 0 && len(accountIDs) > 0 {
			tokens = a.accounts.ListTokensByIDs(accountIDs)
		}
		if len(tokens) == 0 && len(accountIDs) == 0 {
			tokens = a.accounts.ListTokens()
		}
		if len(tokens) == 0 {
			if len(accountIDs) > 0 {
				util.WriteError(w, http.StatusNotFound, "account not found")
				return
			}
			util.WriteError(w, http.StatusBadRequest, "access_tokens or account_ids is required")
			return
		}
		result := a.accounts.RefreshAccounts(r.Context(), tokens)
		a.redactAccountPayloadForIdentity(identity, result)
		util.WriteJSON(w, http.StatusOK, result)
	case r.URL.Path == "/api/accounts/update" && r.Method == http.MethodPost:
		body, _ := readJSONMap(r)
		token := util.Clean(body["access_token"])
		accountID := util.Clean(body["account_id"])
		if token == "" && accountID != "" {
			token = a.accounts.GetTokenByID(accountID)
			if token == "" {
				util.WriteError(w, http.StatusNotFound, "account not found")
				return
			}
		}
		if token == "" {
			util.WriteError(w, http.StatusBadRequest, "access_token or account_id is required")
			return
		}
		updates := map[string]any{}
		for _, key := range []string{"type", "status", "quota"} {
			if value, ok := body[key]; ok && value != nil {
				updates[key] = value
			}
		}
		if len(updates) == 0 {
			util.WriteError(w, http.StatusBadRequest, "no updates provided")
			return
		}
		item := a.accounts.UpdateAccount(token, updates)
		if item == nil {
			util.WriteError(w, http.StatusNotFound, "account not found")
			return
		}
		result := map[string]any{"item": item, "items": a.accounts.ListAccounts()}
		a.redactAccountPayloadForIdentity(identity, result)
		util.WriteJSON(w, http.StatusOK, result)
	default:
		http.NotFound(w, r)
	}
}

func (a *App) accountItemsForIdentity(identity service.Identity) []map[string]any {
	items := a.accounts.ListAccounts()
	if !a.identityCanAccessAPI(identity, http.MethodGet, "/api/accounts/tokens") {
		redactAccountTokens(items)
	}
	return items
}

func (a *App) redactAccountPayloadForIdentity(identity service.Identity, payload map[string]any) {
	if a.identityCanAccessAPI(identity, http.MethodGet, "/api/accounts/tokens") {
		return
	}
	if item, ok := payload["item"].(map[string]any); ok {
		redactAccountToken(item)
	}
	if items, ok := payload["items"].([]map[string]any); ok {
		redactAccountTokens(items)
	}
	if errors, ok := payload["errors"].([]map[string]string); ok {
		for _, item := range errors {
			token := item["access_token"]
			delete(item, "access_token")
			if token != "" {
				item["account_id"] = util.SHA1Short(token, 16)
			}
		}
	}
}

func redactAccountTokens(items []map[string]any) {
	for _, item := range items {
		redactAccountToken(item)
	}
}

func redactAccountToken(item map[string]any) {
	delete(item, "access_token")
}

func (a *App) handleCPA(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireIdentity(w, r, ""); !ok {
		return
	}
	parts := splitPath(r.URL.Path)
	if len(parts) == 3 && r.URL.Path == "/api/cpa/pools" {
		switch r.Method {
		case http.MethodGet:
			util.WriteJSON(w, http.StatusOK, map[string]any{"pools": sanitizeCPAPools(a.cpa.ListPools())})
		case http.MethodPost:
			body, _ := readJSONMap(r)
			if util.Clean(body["base_url"]) == "" {
				util.WriteError(w, http.StatusBadRequest, "base_url is required")
				return
			}
			if util.Clean(body["secret_key"]) == "" {
				util.WriteError(w, http.StatusBadRequest, "secret_key is required")
				return
			}
			pool := a.cpa.AddPool(util.Clean(body["name"]), util.Clean(body["base_url"]), util.Clean(body["secret_key"]))
			util.WriteJSON(w, http.StatusOK, map[string]any{"pool": sanitizeCPAPool(pool), "pools": sanitizeCPAPools(a.cpa.ListPools())})
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
		return
	}
	if len(parts) < 4 {
		http.NotFound(w, r)
		return
	}
	poolID := parts[3]
	pool := a.cpa.GetPool(poolID)
	if pool == nil {
		util.WriteError(w, http.StatusNotFound, "pool not found")
		return
	}
	if len(parts) == 4 {
		switch r.Method {
		case http.MethodPost:
			body, _ := readJSONMap(r)
			updated := a.cpa.UpdatePool(poolID, body)
			util.WriteJSON(w, http.StatusOK, map[string]any{"pool": sanitizeCPAPool(updated), "pools": sanitizeCPAPools(a.cpa.ListPools())})
		case http.MethodDelete:
			if !a.cpa.DeletePool(poolID) {
				util.WriteError(w, http.StatusNotFound, "pool not found")
				return
			}
			util.WriteJSON(w, http.StatusOK, map[string]any{"pools": sanitizeCPAPools(a.cpa.ListPools())})
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
		return
	}
	if len(parts) == 5 && parts[4] == "files" && r.Method == http.MethodGet {
		files, err := a.cpaImport.ListRemoteFiles(r.Context(), pool)
		if err != nil {
			util.WriteError(w, http.StatusBadGateway, err.Error())
			return
		}
		util.WriteJSON(w, http.StatusOK, map[string]any{"pool_id": poolID, "files": files})
		return
	}
	if len(parts) == 5 && parts[4] == "import" {
		if r.Method == http.MethodGet {
			util.WriteJSON(w, http.StatusOK, map[string]any{"import_job": pool["import_job"]})
			return
		}
		if r.Method == http.MethodPost {
			body, _ := readJSONMap(r)
			job, err := a.cpaImport.StartImport(pool, util.AsStringSlice(body["names"]))
			if err != nil {
				util.WriteError(w, http.StatusBadRequest, err.Error())
				return
			}
			util.WriteJSON(w, http.StatusOK, map[string]any{"import_job": job})
			return
		}
	}
	http.NotFound(w, r)
}

func (a *App) handleSub2API(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireIdentity(w, r, ""); !ok {
		return
	}
	parts := splitPath(r.URL.Path)
	if r.URL.Path == "/api/sub2api/servers" {
		switch r.Method {
		case http.MethodGet:
			util.WriteJSON(w, http.StatusOK, map[string]any{"servers": sanitizeSub2Servers(a.sub2.ListServers())})
		case http.MethodPost:
			body, _ := readJSONMap(r)
			if util.Clean(body["base_url"]) == "" {
				util.WriteError(w, http.StatusBadRequest, "base_url is required")
				return
			}
			hasLogin := util.Clean(body["email"]) != "" && util.Clean(body["password"]) != ""
			hasAPIKey := util.Clean(body["api_key"]) != ""
			if !hasLogin && !hasAPIKey {
				util.WriteError(w, http.StatusBadRequest, "email+password or api_key is required")
				return
			}
			server := a.sub2.AddServer(util.Clean(body["name"]), util.Clean(body["base_url"]), util.Clean(body["email"]), util.Clean(body["password"]), util.Clean(body["api_key"]), util.Clean(body["group_id"]))
			util.WriteJSON(w, http.StatusOK, map[string]any{"server": sanitizeSub2Server(server), "servers": sanitizeSub2Servers(a.sub2.ListServers())})
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
		return
	}
	if len(parts) < 4 {
		http.NotFound(w, r)
		return
	}
	serverID := parts[3]
	server := a.sub2.GetServer(serverID)
	if server == nil {
		util.WriteError(w, http.StatusNotFound, "server not found")
		return
	}
	if len(parts) == 4 {
		switch r.Method {
		case http.MethodPost:
			body, _ := readJSONMap(r)
			updated := a.sub2.UpdateServer(serverID, body)
			util.WriteJSON(w, http.StatusOK, map[string]any{"server": sanitizeSub2Server(updated), "servers": sanitizeSub2Servers(a.sub2.ListServers())})
		case http.MethodDelete:
			if !a.sub2.DeleteServer(serverID) {
				util.WriteError(w, http.StatusNotFound, "server not found")
				return
			}
			util.WriteJSON(w, http.StatusOK, map[string]any{"servers": sanitizeSub2Servers(a.sub2.ListServers())})
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
		return
	}
	if len(parts) == 5 && parts[4] == "groups" && r.Method == http.MethodGet {
		groups, err := a.sub2Import.ListRemoteGroups(r.Context(), server)
		if err != nil {
			util.WriteError(w, http.StatusBadGateway, err.Error())
			return
		}
		util.WriteJSON(w, http.StatusOK, map[string]any{"server_id": serverID, "groups": groups})
		return
	}
	if len(parts) == 5 && parts[4] == "accounts" && r.Method == http.MethodGet {
		accounts, err := a.sub2Import.ListRemoteAccounts(r.Context(), server)
		if err != nil {
			util.WriteError(w, http.StatusBadGateway, err.Error())
			return
		}
		util.WriteJSON(w, http.StatusOK, map[string]any{"server_id": serverID, "accounts": accounts})
		return
	}
	if len(parts) == 5 && parts[4] == "import" {
		if r.Method == http.MethodGet {
			util.WriteJSON(w, http.StatusOK, map[string]any{"import_job": server["import_job"]})
			return
		}
		if r.Method == http.MethodPost {
			body, _ := readJSONMap(r)
			job, err := a.sub2Import.StartImport(server, util.AsStringSlice(body["account_ids"]))
			if err != nil {
				util.WriteError(w, http.StatusBadRequest, err.Error())
				return
			}
			util.WriteJSON(w, http.StatusOK, map[string]any{"import_job": job})
			return
		}
	}
	http.NotFound(w, r)
}

func (a *App) handleCreationTasks(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	identity, ok := a.requireIdentity(w, r, "")
	if !ok {
		return
	}
	parts := splitPath(r.URL.Path)
	if r.URL.Path == "/api/creation-tasks" && r.Method == http.MethodGet {
		util.WriteJSON(w, http.StatusOK, a.tasks.ListTasks(identity, util.ParseCommaList(r.URL.Query().Get("ids"))))
		return
	}
	if len(parts) == 4 && parts[0] == "api" && parts[1] == "creation-tasks" && parts[3] == "cancel" {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		task, err := a.tasks.CancelTask(identity, parts[2])
		if err != nil {
			util.WriteError(w, http.StatusNotFound, err.Error())
			return
		}
		util.WriteJSON(w, http.StatusOK, task)
		return
	}
	if r.URL.Path == "/api/creation-tasks/image-generations" && r.Method == http.MethodPost {
		body, _ := readJSONMap(r)
		normalizeImageRequestPayload(body)
		if !a.validateImageSingleCount(w, util.ToInt(body["n"], 1)) {
			return
		}
		if !a.ensureImageBillingCredit(w, identity, body) {
			return
		}
		task, err := a.tasks.SubmitGenerationWithOptions(r.Context(), identity, util.Clean(body["client_task_id"]), util.Clean(body["prompt"]), firstNonEmpty(util.Clean(body["model"]), util.ImageModelAuto), util.Clean(body["size"]), util.Clean(body["quality"]), a.resolveImageBaseURL(r), util.ToInt(body["n"], 1), body["messages"], imageTaskRequestMetadata(body), imageOutputOptionsFromBody(body), util.Clean(body["visibility"]))
		if err != nil {
			writeCreationTaskSubmitError(w, err)
			return
		}
		util.WriteJSON(w, http.StatusOK, task)
		return
	}
	if r.URL.Path == "/api/creation-tasks/response-image-generations" && r.Method == http.MethodPost {
		body, _ := readJSONMap(r)
		normalizeImageRequestPayload(body)
		if !a.validateImageSingleCount(w, util.ToInt(body["n"], 1)) {
			return
		}
		if !a.ensureImageBillingCredit(w, identity, body) {
			return
		}
		task, err := a.tasks.SubmitResponseImageGenerationWithOptions(r.Context(), identity, util.Clean(body["client_task_id"]), util.Clean(body["prompt"]), firstNonEmpty(util.Clean(body["model"]), util.ImageModelAuto), util.Clean(body["size"]), util.Clean(body["quality"]), a.resolveImageBaseURL(r), body["images"], util.ToInt(body["n"], 1), body["messages"], imageTaskRequestMetadata(body), imageOutputOptionsFromBody(body), util.Clean(body["visibility"]))
		if err != nil {
			writeCreationTaskSubmitError(w, err)
			return
		}
		util.WriteJSON(w, http.StatusOK, task)
		return
	}
	if r.URL.Path == "/api/creation-tasks/chat-completions" && r.Method == http.MethodPost {
		body, _ := readJSONMap(r)
		task, err := a.tasks.SubmitChat(r.Context(), identity, util.Clean(body["client_task_id"]), util.Clean(body["prompt"]), firstNonEmpty(util.Clean(body["model"]), util.ImageModelAuto), body["messages"], body["images"])
		if err != nil {
			writeCreationTaskSubmitError(w, err)
			return
		}
		util.WriteJSON(w, http.StatusOK, task)
		return
	}
	if r.URL.Path == "/api/creation-tasks/image-edits" && r.Method == http.MethodPost {
		body, images, err := readMultipartImageBody(r)
		if err != nil {
			util.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		normalizeImageRequestPayload(body)
		if !a.validateImageSingleCount(w, util.ToInt(body["n"], 1)) {
			return
		}
		if !a.ensureImageBillingCredit(w, identity, body) {
			return
		}
		task, err := a.tasks.SubmitEditWithOptions(r.Context(), identity, util.Clean(body["client_task_id"]), util.Clean(body["prompt"]), firstNonEmpty(util.Clean(body["model"]), util.ImageModelAuto), util.Clean(body["size"]), util.Clean(body["quality"]), a.resolveImageBaseURL(r), images, util.ToInt(body["n"], 1), body["messages"], imageTaskRequestMetadata(body), imageOutputOptionsFromBody(body), util.Clean(body["visibility"]))
		if err != nil {
			writeCreationTaskSubmitError(w, err)
			return
		}
		util.WriteJSON(w, http.StatusOK, task)
		return
	}
	http.NotFound(w, r)
}

func imageTaskRequestMetadata(body map[string]any) map[string]any {
	size := util.Clean(body["size"])
	metadata := map[string]any{}
	if preset := service.NormalizeImageResolutionPreset(firstNonEmpty(util.Clean(body["resolution"]), util.Clean(body["image_resolution"]))); preset != "" {
		metadata["resolution"] = preset
		metadata["image_resolution"] = preset
	}
	if size != "" {
		metadata["requested_size"] = size
	}
	return metadata
}

func imageOutputOptionsFromBody(body map[string]any) service.ImageOutputOptions {
	format := service.NormalizeImageOutputFormat(util.Clean(body["output_format"]))
	options := service.ImageOutputOptions{Format: format}
	if format != "png" {
		if compression, ok := imageOutputCompressionFromBody(body["output_compression"]); ok {
			options.Compression = &compression
		}
	}
	return options
}

func imageOutputCompressionFromBody(value any) (int, bool) {
	if value == nil || strings.TrimSpace(util.Clean(value)) == "" {
		return 0, false
	}
	compression := util.ToInt(value, -1)
	if compression < 0 {
		return 0, false
	}
	if compression > 100 {
		compression = 100
	}
	return compression, true
}

func writeCreationTaskSubmitError(w http.ResponseWriter, err error) {
	var limitErr service.ImageTaskLimitError
	if errors.As(err, &limitErr) {
		util.WriteError(w, http.StatusTooManyRequests, limitErr.Error())
		return
	}
	util.WriteError(w, http.StatusBadRequest, err.Error())
}

func (a *App) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/api/register/events" {
		token := r.URL.Query().Get("token")
		if _, ok := a.requireIdentity(w, r, "Bearer "+token); !ok {
			return
		}
		a.streamRegisterEvents(w, r)
		return
	}
	if _, ok := a.requireIdentity(w, r, ""); !ok {
		return
	}
	switch {
	case r.URL.Path == "/api/register" && r.Method == http.MethodGet:
		util.WriteJSON(w, http.StatusOK, map[string]any{"register": a.register.Get()})
	case r.URL.Path == "/api/register" && r.Method == http.MethodPost:
		body, _ := readJSONMap(r)
		util.WriteJSON(w, http.StatusOK, map[string]any{"register": a.register.Update(body)})
	case r.URL.Path == "/api/register/start" && r.Method == http.MethodPost:
		util.WriteJSON(w, http.StatusOK, map[string]any{"register": a.register.Start()})
	case r.URL.Path == "/api/register/stop" && r.Method == http.MethodPost:
		util.WriteJSON(w, http.StatusOK, map[string]any{"register": a.register.Stop()})
	case r.URL.Path == "/api/register/reset" && r.Method == http.MethodPost:
		util.WriteJSON(w, http.StatusOK, map[string]any{"register": a.register.Reset()})
	default:
		http.NotFound(w, r)
	}
}

func (a *App) streamRegisterEvents(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	flusher, _ := w.(http.Flusher)
	last := ""
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			payload := jsonString(a.register.Get())
			if payload != last {
				last = payload
				fmt.Fprintf(w, "data: %s\n\n", payload)
				if flusher != nil {
					flusher.Flush()
				}
			}
		}
	}
}

func sanitizeCPAPool(pool map[string]any) map[string]any {
	if pool == nil {
		return nil
	}
	out := util.CopyMap(pool)
	delete(out, "secret_key")
	return out
}

func sanitizeCPAPools(pools []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(pools))
	for _, pool := range pools {
		out = append(out, sanitizeCPAPool(pool))
	}
	return out
}

func sanitizeSub2Server(server map[string]any) map[string]any {
	if server == nil {
		return nil
	}
	out := util.CopyMap(server)
	out["has_api_key"] = util.Clean(server["api_key"]) != ""
	delete(out, "password")
	delete(out, "api_key")
	return out
}

func sanitizeSub2Servers(servers []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(servers))
	for _, server := range servers {
		out = append(out, sanitizeSub2Server(server))
	}
	return out
}

func splitPath(path string) []string {
	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "/")
}
