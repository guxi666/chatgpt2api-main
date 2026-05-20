package httpapi

import (
	"fmt"
	"net/http"
	"strings"

	"chatgpt2api/internal/service"
	"chatgpt2api/internal/util"
)

type subscriptionTierRuntime struct {
	Key         string
	Name        string
	Description string
	Badge       string
	PriceCents  int
	PriceNote   string
	Features    []string
	RoleName    string
}

func (a *App) handleSubscriptions(w http.ResponseWriter, r *http.Request) {
	identity, ok := a.requireIdentity(w, r, "")
	if !ok {
		return
	}
	a.ensureSubscriptionTierRoles()

	switch r.Method {
	case http.MethodGet:
		if !a.config.SubscriptionEnabled() {
			wallet := a.billing.GetWalletByIdentity(identity)
			if wallet == nil {
				wallet = map[string]any{
					"balance_cents":        0,
					"total_recharge_cents": 0,
					"total_consume_cents":  0,
				}
			}
			util.WriteJSON(w, http.StatusOK, map[string]any{
				"enabled":      false,
				"plans":        subscriptionPlansPayload(a.subscriptionTiers()),
				"status":       a.billing.SubscriptionStatusByIdentity(identity),
				"wallet":       wallet,
				"pay_channels": []string{},
				"heading":      a.config.SubscriptionHeading(),
				"subheading":   a.config.SubscriptionSubheading(),
				"safety_text":  a.config.SubscriptionSafetyText(),
				"agent_hint":   a.config.SubscriptionAgentHint(),
			})
			return
		}
		wallet := a.billing.GetWalletByIdentity(identity)
		payChannels := a.availablePayChannels()
		discountBP := subscriptionDiscountBasisPoint(wallet)
		if wallet != nil {
			payChannels = append([]string{"balance"}, payChannels...)
		} else {
			wallet = map[string]any{
				"balance_cents":        0,
				"total_recharge_cents": 0,
				"total_consume_cents":  0,
			}
		}
		plans := a.subscriptionTiers()
		if discountBP > 0 {
			for index := range plans {
				basePrice := plans[index].PriceCents
				plans[index].PriceCents = applySubscriptionDiscount(basePrice, discountBP)
				discountHint := fmt.Sprintf("代理优惠 %.2f%%", float64(discountBP)/100.0)
				if strings.TrimSpace(plans[index].PriceNote) == "" {
					plans[index].PriceNote = discountHint
				} else {
					plans[index].PriceNote = strings.TrimSpace(plans[index].PriceNote) + " · " + discountHint
				}
			}
		}
		util.WriteJSON(w, http.StatusOK, map[string]any{
			"enabled":      true,
			"plans":        subscriptionPlansPayload(plans),
			"status":       a.billing.SubscriptionStatusByIdentity(identity),
			"wallet":       wallet,
			"pay_channels": uniqueNonEmptyStrings(payChannels),
			"heading":      a.config.SubscriptionHeading(),
			"subheading":   a.config.SubscriptionSubheading(),
			"safety_text":  a.config.SubscriptionSafetyText(),
			"agent_hint":   a.config.SubscriptionAgentHint(),
		})
	case http.MethodPost:
		if !a.config.SubscriptionEnabled() {
			util.WriteError(w, http.StatusForbidden, "subscription is disabled")
			return
		}
		body, err := readJSONMap(r)
		if err != nil {
			util.WriteError(w, http.StatusBadRequest, "invalid json body")
			return
		}
		tierKey := strings.ToLower(strings.TrimSpace(util.Clean(body["tier"])))
		tier, found := a.subscriptionTierByKey(tierKey)
		if !found {
			util.WriteError(w, http.StatusBadRequest, "invalid subscription tier")
			return
		}
		discountBP := subscriptionDiscountBasisPoint(a.billing.GetWalletByIdentity(identity))
		payableCents := applySubscriptionDiscount(tier.PriceCents, discountBP)
		payType := strings.ToLower(strings.TrimSpace(firstNonEmpty(util.Clean(body["pay_type"]), util.Clean(body["type"]), "alipay")))
		if payType == "balance" {
			order, orderErr := a.billing.CreateSubscriptionByBalance(identity, tier.Key, payableCents)
			if orderErr != nil {
				util.WriteError(w, http.StatusBadRequest, orderErr.Error())
				return
			}
			userID := strings.TrimSpace(identity.OwnerID)
			if userID == "" {
				userID = strings.TrimSpace(identity.ID)
			}
			a.activateSubscriptionFromPayResult(map[string]any{
				"user_id":           userID,
				"subscription_tier": tier.Key,
			}, "Balance pay")
			util.WriteJSON(w, http.StatusOK, map[string]any{
				"ok":              true,
				"pending_payment": false,
				"tier":            tier.Key,
				"order":           order,
				"wallet":          a.billing.GetWalletByIdentity(identity),
				"status":          a.billing.SubscriptionStatusByIdentity(identity),
			})
			return
		}
		order, orderErr := a.billing.CreateSubscriptionYiPayOrder(identity, tier.Key, payableCents, payType, a.yiPayGatewayConfigForPath(r, "/subscription"))
		if orderErr != nil {
			util.WriteError(w, http.StatusBadRequest, orderErr.Error())
			return
		}
		util.WriteJSON(w, http.StatusOK, map[string]any{
			"ok":              true,
			"pending_payment": true,
			"tier":            tier.Key,
			"order":           order,
		})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func subscriptionDiscountBasisPoint(wallet map[string]any) int {
	if wallet == nil || !util.ToBool(wallet["agency_enabled"]) {
		return 0
	}
	bp := util.ToInt(wallet["agency_discount_bp"], 0)
	if bp < 0 {
		return 0
	}
	if bp > 10000 {
		return 10000
	}
	return bp
}

func applySubscriptionDiscount(priceCents, discountBP int) int {
	priceCents = util.ToInt(priceCents, 0)
	if priceCents < 1 {
		return 1
	}
	discountBP = util.ToInt(discountBP, 0)
	if discountBP <= 0 {
		return priceCents
	}
	if discountBP >= 10000 {
		return 1
	}
	discounted := priceCents * (10000 - discountBP) / 10000
	if discounted < 1 {
		return 1
	}
	return discounted
}

func (a *App) subscriptionTiers() []subscriptionTierRuntime {
	return []subscriptionTierRuntime{
		{
			Key:         service.SubscriptionTierMonthly,
			Name:        firstNonEmpty(strings.TrimSpace(a.config.SubscriptionMonthlyName()), "包月套餐"),
			Description: strings.TrimSpace(a.config.SubscriptionMonthlyDesc()),
			Badge:       strings.TrimSpace(a.config.SubscriptionMonthlyBadge()),
			PriceCents:  a.config.SubscriptionMonthlyPriceCents(),
			PriceNote:   strings.TrimSpace(a.config.SubscriptionMonthlyPriceNote()),
			Features:    splitMultilineFeatures(a.config.SubscriptionMonthlyFeatures()),
			RoleName:    "套餐-包月",
		},
		{
			Key:         service.SubscriptionTierQuarterly,
			Name:        firstNonEmpty(strings.TrimSpace(a.config.SubscriptionQuarterlyName()), "包季套餐"),
			Description: strings.TrimSpace(a.config.SubscriptionQuarterlyDesc()),
			Badge:       strings.TrimSpace(a.config.SubscriptionQuarterlyBadge()),
			PriceCents:  a.config.SubscriptionQuarterlyPriceCents(),
			PriceNote:   strings.TrimSpace(a.config.SubscriptionQuarterlyPriceNote()),
			Features:    splitMultilineFeatures(a.config.SubscriptionQuarterlyFeatures()),
			RoleName:    "套餐-包季",
		},
		{
			Key:         service.SubscriptionTierYearly,
			Name:        firstNonEmpty(strings.TrimSpace(a.config.SubscriptionYearlyName()), "包年套餐"),
			Description: strings.TrimSpace(a.config.SubscriptionYearlyDesc()),
			Badge:       strings.TrimSpace(a.config.SubscriptionYearlyBadge()),
			PriceCents:  a.config.SubscriptionYearlyPriceCents(),
			PriceNote:   strings.TrimSpace(a.config.SubscriptionYearlyPriceNote()),
			Features:    splitMultilineFeatures(a.config.SubscriptionYearlyFeatures()),
			RoleName:    "套餐-包年",
		},
	}
}

func (a *App) subscriptionTierByKey(key string) (subscriptionTierRuntime, bool) {
	for _, tier := range a.subscriptionTiers() {
		if tier.Key == key {
			return tier, true
		}
	}
	return subscriptionTierRuntime{}, false
}

func subscriptionPlansPayload(items []subscriptionTierRuntime) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, map[string]any{
			"key":          item.Key,
			"name":         item.Name,
			"description":  item.Description,
			"badge":        item.Badge,
			"price_cents":  item.PriceCents,
			"price_yuan":   float64(item.PriceCents) / 100.0,
			"price_note":   item.PriceNote,
			"features":     item.Features,
			"role_name":    item.RoleName,
			"period_label": subscriptionPeriodLabel(item.Key),
		})
	}
	return out
}

func subscriptionPeriodLabel(tier string) string {
	switch strings.TrimSpace(tier) {
	case service.SubscriptionTierMonthly:
		return "每月"
	case service.SubscriptionTierQuarterly:
		return "每季"
	case service.SubscriptionTierYearly:
		return "每年"
	default:
		return ""
	}
}

func splitMultilineFeatures(value string) []string {
	lines := strings.Split(strings.ReplaceAll(value, "\r\n", "\n"), "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		out = append(out, line)
	}
	if len(out) == 0 {
		return []string{"无限生图", "套餐期内不扣余额"}
	}
	return out
}

func uniqueNonEmptyStrings(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func (a *App) ensureSubscriptionTierRoles() {
	currentRoles := a.auth.ListRoles()
	for _, tier := range a.subscriptionTiers() {
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
			"/subscription",
			"/agency",
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
			service.APIPermissionKey("GET", "/api/subscriptions/plans"),
			service.APIPermissionKey("POST", "/api/subscriptions/orders"),
			service.APIPermissionKey("GET", "/api/agency"),
			service.APIPermissionKey("GET", "/api/agency/commission"),
			service.APIPermissionKey("GET", "/api/agency/withdrawals"),
			service.APIPermissionKey("POST", "/api/agency/withdrawals"),
			service.APIPermissionKey("GET", "/api/agency/withdraw-profile"),
			service.APIPermissionKey("POST", "/api/agency/withdraw-profile"),
			service.APIPermissionKey("POST", "/api/agency/withdraw-profile/upload"),
			service.APIPermissionKey("POST", "/api/agency/join"),
			service.APIPermissionKey("POST", "/api/agency/upgrade"),
		}
		payload := map[string]any{
			"name":            tier.RoleName,
			"description":     fmt.Sprintf("%s权限组（套餐期内无限生图）", tier.Name),
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

func (a *App) subscriptionRoleIDByTier(tierKey string) (string, bool) {
	roleName := ""
	for _, tier := range a.subscriptionTiers() {
		if tier.Key == tierKey {
			roleName = tier.RoleName
			break
		}
	}
	if roleName == "" {
		return "", false
	}
	for _, role := range a.auth.ListRoles() {
		if strings.TrimSpace(util.Clean(role["name"])) == roleName {
			id := strings.TrimSpace(util.Clean(role["id"]))
			if id != "" {
				return id, true
			}
		}
	}
	return "", false
}
