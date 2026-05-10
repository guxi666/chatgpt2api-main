package httpapi

import (
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
	if identity.Role != service.AuthRoleUser || identity.Provider != service.AuthProviderLinuxDo || identity.OwnerID == "" {
		return service.AuthKeyFilter{}, service.AuthOwner{}, false
	}
	filter.OwnerID = identity.OwnerID
	return filter, service.AuthOwner{ID: identity.OwnerID, Name: identity.Name, Provider: identity.Provider}, true
}

func (a *App) handleAdminUsers(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireAdmin(w, r); !ok {
		return
	}
	base := "/api/admin/users"
	if r.URL.Path == base {
		switch r.Method {
		case http.MethodGet:
			util.WriteJSON(w, http.StatusOK, map[string]any{"items": a.managedUsers()})
		case http.MethodPost:
			body, _ := readJSONMap(r)
			apiKey, raw, err := a.auth.CreateAPIKey(service.AuthRoleUser, util.Clean(body["name"]), service.AuthOwner{})
			if err != nil {
				util.WriteError(w, http.StatusBadRequest, err.Error())
				return
			}
			userID := util.Clean(apiKey["id"])
			if userID != "" {
				a.billing.EnsureWalletUser(userID, util.Clean(apiKey["name"]), service.AuthProviderLocal)
			}
			items := a.managedUsers()
			util.WriteJSON(w, http.StatusOK, map[string]any{"item": findManagedUser(items, util.Clean(apiKey["id"])), "api_key": apiKey, "key": raw, "items": items})
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
	userID, decodeErr := url.PathUnescape(parts[3])
	if decodeErr != nil {
		util.WriteError(w, http.StatusBadRequest, "invalid user id")
		return
	}
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
		items := a.managedUsers()
		if current := findManagedUser(items, userID); current != nil {
			item = current
		}
		util.WriteJSON(w, http.StatusOK, map[string]any{"item": item, "api_key": apiKey, "key": raw, "items": items})
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
		item := a.auth.UpdateUser(userID, updates)
		if item == nil {
			util.WriteError(w, http.StatusNotFound, "user not found")
			return
		}
		items := a.managedUsers()
		if current := findManagedUser(items, userID); current != nil {
			item = current
		}
		util.WriteJSON(w, http.StatusOK, map[string]any{"item": item, "items": items})
	case http.MethodDelete:
		if !a.auth.DeleteUser(userID) {
			util.WriteError(w, http.StatusNotFound, "user not found")
			return
		}
		util.WriteJSON(w, http.StatusOK, map[string]any{"items": a.managedUsers()})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (a *App) managedUsers() []map[string]any {
	items := a.auth.ListUsers()
	billingUsers := map[string]map[string]any{}
	for _, item := range a.billing.ListUsersForAdmin() {
		id := util.Clean(item["id"])
		if id != "" {
			billingUsers[id] = item
		}
	}
	stats := a.logs.UserUsageStats(14)
	for _, item := range items {
		userID := util.Clean(item["id"])
		if util.Clean(item["provider"]) == service.AuthProviderLocal && userID != "" {
			if wallet := a.billing.EnsureWalletUser(userID, util.Clean(item["name"]), service.AuthProviderLocal); wallet != nil {
				billingUsers[userID] = wallet
			}
		}
		usage := stats[userID]
		if usage == nil {
			usage = service.ZeroUserUsageStats(14)
		}
		for key, value := range usage {
			item[key] = value
		}
		if billing, exists := billingUsers[userID]; exists {
			if util.Clean(item["provider"]) == service.AuthProviderEmail {
				if email := strings.TrimSpace(util.Clean(billing["email"])); email != "" {
					item["email"] = email
				}
			}
			item["balance_cents"] = util.ToInt(billing["balance_cents"], 0)
			item["total_recharge_cents"] = util.ToInt(billing["total_recharge_cents"], 0)
			item["total_consume_cents"] = util.ToInt(billing["total_consume_cents"], 0)
			item["billing_user"] = true
			item["image_price_cents"] = a.config.ImagePriceCents()
			continue
		}
		item["balance_cents"] = 0
		item["total_recharge_cents"] = 0
		item["total_consume_cents"] = 0
		item["billing_user"] = false
		item["image_price_cents"] = a.config.ImagePriceCents()
	}
	return items
}

func findManagedUser(items []map[string]any, id string) map[string]any {
	for _, item := range items {
		if item["id"] == id {
			return item
		}
	}
	return nil
}

func (a *App) handleAdminBilling(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireAdmin(w, r); !ok {
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
				"items":  a.managedUsers(),
			})
			return
		}
		http.NotFound(w, r)
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
		limit := util.ToInt(r.URL.Query().Get("limit"), 0)
		items, stats := a.billing.ListOrdersForAdmin(limit)
		util.WriteJSON(w, http.StatusOK, map[string]any{
			"items":  items,
			"stats":  stats,
			"limit":  limit,
			"scope":  "all_time",
			"source": "billing_orders",
		})
		return
	}

	http.NotFound(w, r)
}

func (a *App) handlePublicAnnouncements(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	util.WriteJSON(w, http.StatusOK, map[string]any{"items": a.announce.ListVisible(strings.TrimSpace(r.URL.Query().Get("target")))})
}

func (a *App) handleAdminAnnouncements(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireAdmin(w, r); !ok {
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
	if _, ok := a.requireAdmin(w, r); !ok {
		return
	}
	switch {
	case r.URL.Path == "/api/accounts" && r.Method == http.MethodGet:
		util.WriteJSON(w, http.StatusOK, map[string]any{"items": a.accounts.ListAccounts()})
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
		util.WriteJSON(w, http.StatusOK, result)
	case r.URL.Path == "/api/accounts" && r.Method == http.MethodDelete:
		body, _ := readJSONMap(r)
		tokens := util.AsStringSlice(body["tokens"])
		if len(tokens) == 0 {
			util.WriteError(w, http.StatusBadRequest, "tokens is required")
			return
		}
		util.WriteJSON(w, http.StatusOK, a.accounts.DeleteAccounts(tokens))
	case r.URL.Path == "/api/accounts/refresh" && r.Method == http.MethodPost:
		body, _ := readJSONMap(r)
		tokens := util.AsStringSlice(body["access_tokens"])
		if len(tokens) == 0 {
			tokens = a.accounts.ListTokens()
		}
		if len(tokens) == 0 {
			util.WriteError(w, http.StatusBadRequest, "access_tokens is required")
			return
		}
		util.WriteJSON(w, http.StatusOK, a.accounts.RefreshAccounts(r.Context(), tokens))
	case r.URL.Path == "/api/accounts/update" && r.Method == http.MethodPost:
		body, _ := readJSONMap(r)
		token := util.Clean(body["access_token"])
		if token == "" {
			util.WriteError(w, http.StatusBadRequest, "access_token is required")
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
		util.WriteJSON(w, http.StatusOK, map[string]any{"item": item, "items": a.accounts.ListAccounts()})
	default:
		http.NotFound(w, r)
	}
}

func (a *App) handleCPA(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireAdmin(w, r); !ok {
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
	if _, ok := a.requireAdmin(w, r); !ok {
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

func (a *App) handleImageTasks(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	identity, ok := a.requireIdentity(w, r, "")
	if !ok {
		return
	}
	parts := splitPath(r.URL.Path)
	if r.URL.Path == "/api/image-tasks" && r.Method == http.MethodGet {
		util.WriteJSON(w, http.StatusOK, a.tasks.ListTasks(identity, util.ParseCommaList(r.URL.Query().Get("ids"))))
		return
	}
	if len(parts) == 4 && parts[0] == "api" && parts[1] == "image-tasks" && parts[3] == "cancel" {
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
	if r.URL.Path == "/api/image-tasks/generations" && r.Method == http.MethodPost {
		if !a.ensureImageBillingCredit(w, identity) {
			return
		}
		body, _ := readJSONMap(r)
		task, err := a.tasks.SubmitGeneration(r.Context(), identity, util.Clean(body["client_task_id"]), util.Clean(body["prompt"]), firstNonEmpty(util.Clean(body["model"]), util.ImageModelAuto), util.Clean(body["size"]), util.Clean(body["quality"]), a.resolveImageBaseURL(r), util.ToInt(body["n"], 1), body["messages"])
		if err != nil {
			writeImageTaskSubmitError(w, err)
			return
		}
		util.WriteJSON(w, http.StatusOK, task)
		return
	}
	if r.URL.Path == "/api/image-tasks/edits" && r.Method == http.MethodPost {
		if !a.ensureImageBillingCredit(w, identity) {
			return
		}
		body, images, err := readMultipartImageBody(r)
		if err != nil {
			util.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		task, err := a.tasks.SubmitEdit(r.Context(), identity, util.Clean(body["client_task_id"]), util.Clean(body["prompt"]), firstNonEmpty(util.Clean(body["model"]), util.ImageModelAuto), util.Clean(body["size"]), util.Clean(body["quality"]), a.resolveImageBaseURL(r), images, util.ToInt(body["n"], 1), body["messages"])
		if err != nil {
			writeImageTaskSubmitError(w, err)
			return
		}
		util.WriteJSON(w, http.StatusOK, task)
		return
	}
	http.NotFound(w, r)
}

func writeImageTaskSubmitError(w http.ResponseWriter, err error) {
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
	if _, ok := a.requireAdmin(w, r); !ok {
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
