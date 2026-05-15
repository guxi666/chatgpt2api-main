package httpapi

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"chatgpt2api/internal/service"
	"chatgpt2api/internal/util"
)

type agencyTierRuntime struct {
	Key          string
	Name         string
	Description  string
	PriceCents   int
	CommissionBP int
	DiscountBP   int
	RoleName     string
}

func (a *App) handleAgency(w http.ResponseWriter, r *http.Request) {
	identity, ok := a.requireIdentity(w, r, "")
	if !ok {
		return
	}

	switch r.Method {
	case http.MethodGet:
		if identity.Role == service.AuthRoleAdmin {
			a.ensureAgencyTierRoles()
		}
		util.WriteJSON(w, http.StatusOK, agencyPayload(a, identity.Role == service.AuthRoleAdmin))
	case http.MethodPost:
		if identity.Role != service.AuthRoleAdmin {
			util.WriteError(w, http.StatusForbidden, "admin permission required")
			return
		}
		a.ensureAgencyTierRoles()
		body, _ := readJSONMap(r)
		updates := map[string]any{
			"agency_tier_basic_cents":           maxZeroInt(body["agency_tier_basic_cents"]),
			"agency_tier_pro_cents":             maxZeroInt(body["agency_tier_pro_cents"]),
			"agency_tier_premium_cents":         maxZeroInt(body["agency_tier_premium_cents"]),
			"agency_tier_basic_commission_bp":   clampAgencyBasisPoint(body["agency_tier_basic_commission_bp"], 3000),
			"agency_tier_pro_commission_bp":     clampAgencyBasisPoint(body["agency_tier_pro_commission_bp"], 4500),
			"agency_tier_premium_commission_bp": clampAgencyBasisPoint(body["agency_tier_premium_commission_bp"], 6000),
			"agency_tier_basic_discount_bp":     clampAgencyBasisPoint(body["agency_tier_basic_discount_bp"], 500),
			"agency_tier_pro_discount_bp":       clampAgencyBasisPoint(body["agency_tier_pro_discount_bp"], 1000),
			"agency_tier_premium_discount_bp":   clampAgencyBasisPoint(body["agency_tier_premium_discount_bp"], 1500),
		}
		if _, err := a.config.Update(updates); err != nil {
			util.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		a.ensureAgencyTierRoles()
		util.WriteJSON(w, http.StatusOK, agencyPayload(a, true))
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (a *App) handleAgencyJoin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	identity, ok := a.requireIdentity(w, r, "")
	if !ok {
		return
	}
	if identity.Role == service.AuthRoleAdmin {
		util.WriteError(w, http.StatusBadRequest, "admin account cannot join agency tier")
		return
	}
	a.deactivateAgencyIfRoleRevoked(identity)

	body, _ := readJSONMap(r)
	tierKey := strings.TrimSpace(util.Clean(body["tier"]))
	payType := strings.ToLower(strings.TrimSpace(firstNonEmpty(util.Clean(body["pay_type"]), util.Clean(body["type"]), "alipay")))
	tier, found := a.agencyTierByKey(tierKey)
	if !found {
		util.WriteError(w, http.StatusBadRequest, "invalid tier")
		return
	}

	order, err := a.billing.CreateAgencyYiPayOrder(identity, tier.Key, tier.PriceCents, payType, a.yiPayGatewayConfigForPath(r, "/agency"), true)
	if err != nil {
		util.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	util.WriteJSON(w, http.StatusOK, map[string]any{
		"ok":              true,
		"pending_payment": true,
		"tier":            tier.Key,
		"order":           order,
	})
}

func (a *App) handleAgencyUpgrade(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	identity, ok := a.requireIdentity(w, r, "")
	if !ok {
		return
	}
	if identity.Role == service.AuthRoleAdmin {
		util.WriteError(w, http.StatusBadRequest, "admin account cannot upgrade agency tier")
		return
	}
	a.deactivateAgencyIfRoleRevoked(identity)

	body, _ := readJSONMap(r)
	tierKey := strings.TrimSpace(util.Clean(body["tier"]))
	payType := strings.ToLower(strings.TrimSpace(firstNonEmpty(util.Clean(body["pay_type"]), util.Clean(body["type"]), "alipay")))
	tier, found := a.agencyTierByKey(tierKey)
	if !found {
		util.WriteError(w, http.StatusBadRequest, "invalid tier")
		return
	}

	order, err := a.billing.CreateAgencyYiPayOrder(identity, tier.Key, tier.PriceCents, payType, a.yiPayGatewayConfigForPath(r, "/agency"), true)
	if err != nil {
		util.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	util.WriteJSON(w, http.StatusOK, map[string]any{
		"ok":              true,
		"pending_payment": true,
		"tier":            tier.Key,
		"order":           order,
	})
}

func (a *App) handleAgencyCommission(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	identity, ok := a.requireIdentity(w, r, "")
	if !ok {
		return
	}

	registerURL := strings.TrimSpace(a.config.BaseURL())
	if registerURL == "" {
		registerURL = "http://" + strings.TrimSpace(r.Host)
	}
	registerURL = strings.TrimRight(registerURL, "/") + "/login"
	payload, err := a.billing.AgencyDashboardByIdentity(identity, registerURL)
	if err != nil {
		util.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	util.WriteJSON(w, http.StatusOK, payload)
}

func (a *App) handleAgencyWithdrawals(w http.ResponseWriter, r *http.Request) {
	identity, ok := a.requireIdentity(w, r, "")
	if !ok {
		return
	}
	if identity.Role == service.AuthRoleAdmin {
		util.WriteError(w, http.StatusForbidden, "admin permission required")
		return
	}

	switch r.Method {
	case http.MethodGet:
		limit := util.ToInt(r.URL.Query().Get("limit"), 100)
		if limit <= 0 {
			limit = 100
		}
		items, err := a.billing.ListAgencyWithdrawalRequestsByIdentity(identity, limit)
		if err != nil {
			util.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		util.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
	case http.MethodPost:
		body, _ := readJSONMap(r)
		amountYuan := strings.TrimSpace(firstNonEmpty(util.Clean(body["amount"]), util.Clean(body["amount_yuan"])))
		amountCents := util.ToInt(body["amount_cents"], 0)
		if amountCents <= 0 {
			if amountYuan == "" {
				util.WriteError(w, http.StatusBadRequest, "withdraw amount is required")
				return
			}
			parsed, err := strconv.ParseFloat(amountYuan, 64)
			if err != nil || parsed <= 0 {
				util.WriteError(w, http.StatusBadRequest, "invalid withdraw amount")
				return
			}
			amountCents = int(parsed * 100)
			if amountCents <= 0 {
				amountCents = 1
			}
		}
		item, err := a.billing.CreateAgencyWithdrawalRequest(
			identity,
			amountCents,
			util.Clean(body["alipay_qr_code"]),
			firstNonEmpty(util.Clean(body["wechat_qr_code"]), util.Clean(body["wx_qr_code"])),
			util.Clean(body["phone"]),
			firstNonEmpty(util.Clean(body["wechat_id"]), util.Clean(body["wx_id"])),
		)
		if err != nil {
			util.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		util.WriteJSON(w, http.StatusOK, map[string]any{"ok": true, "item": item})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (a *App) handleAgencyAdminWithdrawals(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	identity, ok := a.requireIdentity(w, r, "")
	if !ok {
		return
	}
	if identity.Role != service.AuthRoleAdmin {
		util.WriteError(w, http.StatusForbidden, "admin permission required")
		return
	}
	limit := util.ToInt(r.URL.Query().Get("limit"), 500)
	if limit <= 0 {
		limit = 500
	}
	items := a.billing.ListAgencyWithdrawalRequestsForAdmin(limit)
	util.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (a *App) handleAgencyAdminUsers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	identity, ok := a.requireIdentity(w, r, "")
	if !ok {
		return
	}
	if identity.Role != service.AuthRoleAdmin {
		util.WriteError(w, http.StatusForbidden, "admin permission required")
		return
	}

	if r.Method == http.MethodGet {
		items := a.billing.ListUsersForAdmin()
		out := make([]map[string]any, 0, len(items))
		for _, item := range items {
			if strings.TrimSpace(util.Clean(item["agency_tier"])) == "" {
				continue
			}
			out = append(out, item)
		}
		util.WriteJSON(w, http.StatusOK, map[string]any{"items": out})
		return
	}

	body, _ := readJSONMap(r)
	userID := strings.TrimSpace(util.Clean(body["user_id"]))
	tierKey := strings.TrimSpace(util.Clean(body["tier"]))
	tier, found := a.agencyTierByKey(tierKey)
	if userID == "" || !found {
		util.WriteError(w, http.StatusBadRequest, "invalid user_id or tier")
		return
	}

	a.ensureAgencyTierRoles()
	wallet, err := a.billing.ActivateAgencyByUserID(userID, service.AgencyTierBenefit{
		Tier:         tier.Key,
		CommissionBP: tier.CommissionBP,
		DiscountBP:   tier.DiscountBP,
	}, true)
	if err != nil {
		util.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if roleID, ok := a.agencyRoleIDByTier(tier.Key); ok {
		_ = a.auth.UpdateUser(userID, map[string]any{"role_id": roleID})
	}

	util.WriteJSON(w, http.StatusOK, map[string]any{
		"ok":     true,
		"wallet": wallet,
		"tier":   tier.Key,
	})
}

func agencyPayload(a *App, editable bool) map[string]any {
	tiers := a.agencyTiers()
	items := make([]map[string]any, 0, len(tiers))
	for _, tier := range tiers {
		items = append(items, map[string]any{
			"key":                tier.Key,
			"name":               tier.Name,
			"price_cents":        tier.PriceCents,
			"price_yuan":         float64(tier.PriceCents) / 100.0,
			"description":        tier.Description,
			"commission_bp":      tier.CommissionBP,
			"commission_percent": formatBasisPointPercent(tier.CommissionBP),
			"discount_bp":        tier.DiscountBP,
			"discount_percent":   formatBasisPointPercent(tier.DiscountBP),
			"role_name":          tier.RoleName,
		})
	}
	return map[string]any{
		"editable": editable,
		"tiers":    items,
	}
}

func (a *App) agencyTiers() []agencyTierRuntime {
	return []agencyTierRuntime{
		{
			Key:          service.AgencyTierBasic,
			Name:         "基础代理",
			Description:  "入门代理套餐，适合个人测试与轻运营。",
			PriceCents:   a.config.AgencyTierBasicCents(),
			CommissionBP: a.config.AgencyTierBasicCommissionBP(),
			DiscountBP:   a.config.AgencyTierBasicDiscountBP(),
			RoleName:     "代理-基础",
		},
		{
			Key:          service.AgencyTierPro,
			Name:         "进阶代理",
			Description:  "覆盖更多用户场景，适合中小团队运营。",
			PriceCents:   a.config.AgencyTierProCents(),
			CommissionBP: a.config.AgencyTierProCommissionBP(),
			DiscountBP:   a.config.AgencyTierProDiscountBP(),
			RoleName:     "代理-进阶",
		},
		{
			Key:          service.AgencyTierPremium,
			Name:         "旗舰代理",
			Description:  "高阶代理方案，适合规模化业务团队。",
			PriceCents:   a.config.AgencyTierPremiumCents(),
			CommissionBP: a.config.AgencyTierPremiumCommissionBP(),
			DiscountBP:   a.config.AgencyTierPremiumDiscountBP(),
			RoleName:     "代理-旗舰",
		},
	}
}

func (a *App) agencyTierByKey(key string) (agencyTierRuntime, bool) {
	for _, item := range a.agencyTiers() {
		if item.Key == key {
			return item, true
		}
	}
	return agencyTierRuntime{}, false
}

func (a *App) ensureAgencyTierRoles() {
	currentRoles := a.auth.ListRoles()
	for _, tier := range a.agencyTiers() {
		roleID := ""
		for _, role := range currentRoles {
			if strings.TrimSpace(util.Clean(role["name"])) == tier.RoleName {
				roleID = strings.TrimSpace(util.Clean(role["id"]))
				break
			}
		}

		menuPaths := []string{
			"/image",
			"/image-manager",
			"/wallet",
			"/agency",
			"/agency-commission",
		}
		apiPermissions := []string{
			service.APIPermissionKey("GET", "/v1/models"),
			service.APIPermissionKey("POST", "/v1/images/generations"),
			service.APIPermissionKey("POST", "/v1/images/edits"),
			service.APIPermissionKey("POST", "/v1/chat/completions"),
			service.APIPermissionKey("POST", "/v1/responses"),
			service.APIPermissionKey("POST", "/v1/messages"),
			service.APIPermissionKey("GET", "/api/creation-tasks"),
			service.APIPermissionKey("POST", "/api/creation-tasks"),
			service.APIPermissionKey("GET", "/api/images"),
			service.APIPermissionKey("PATCH", "/api/images/visibility"),
			service.APIPermissionKey("GET", "/api/wallet"),
			service.APIPermissionKey("POST", "/api/wallet/redeem"),
			service.APIPermissionKey("GET", "/api/pay/orders"),
			service.APIPermissionKey("POST", "/api/pay/orders"),
			service.APIPermissionKey("GET", "/api/agency"),
			service.APIPermissionKey("GET", "/api/agency/commission"),
			service.APIPermissionKey("GET", "/api/agency/withdrawals"),
			service.APIPermissionKey("POST", "/api/agency/withdrawals"),
			service.APIPermissionKey("POST", "/api/agency/join"),
			service.APIPermissionKey("POST", "/api/agency/upgrade"),
		}
		payload := map[string]any{
			"name":            tier.RoleName,
			"description":     fmt.Sprintf("%s权限组（分成%s，充值优惠%s）", tier.Name, formatBasisPointPercent(tier.CommissionBP), formatBasisPointPercent(tier.DiscountBP)),
			"menu_paths":      menuPaths,
			"api_permissions": apiPermissions,
		}
		if roleID == "" {
			_, _ = a.auth.CreateRole(payload)
		} else {
			_, _ = a.auth.UpdateRole(roleID, payload)
		}
	}
}

func (a *App) agencyRoleIDByTier(tierKey string) (string, bool) {
	targetRole := ""
	for _, tier := range a.agencyTiers() {
		if tier.Key == tierKey {
			targetRole = tier.RoleName
			break
		}
	}
	if targetRole == "" {
		return "", false
	}
	for _, role := range a.auth.ListRoles() {
		if strings.TrimSpace(util.Clean(role["name"])) == targetRole {
			id := strings.TrimSpace(util.Clean(role["id"]))
			if id != "" {
				return id, true
			}
		}
	}
	return "", false
}

func formatBasisPointPercent(bp int) string {
	return fmt.Sprintf("%.2f%%", float64(bp)/100.0)
}

func maxZeroInt(value any) int {
	n := util.ToInt(value, 0)
	if n < 0 {
		return 0
	}
	return n
}

func clampAgencyBasisPoint(value any, fallback int) int {
	bp := util.ToInt(value, fallback)
	if bp < 0 {
		return 0
	}
	if bp > 10000 {
		return 10000
	}
	return bp
}

func (a *App) deactivateAgencyIfRoleRevoked(identity service.Identity) {
	if isAgencyRoleID(a.auth.ListRoles(), strings.TrimSpace(identity.RoleID)) {
		return
	}
	userID := strings.TrimSpace(identity.OwnerID)
	if userID == "" {
		userID = strings.TrimSpace(identity.ID)
	}
	if userID == "" {
		return
	}
	_, _ = a.billing.DeactivateAgencyByUserID(userID)
}
