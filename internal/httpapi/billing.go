package httpapi

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"chatgpt2api/internal/protocol"
	"chatgpt2api/internal/service"
	"chatgpt2api/internal/util"
	"chatgpt2api/internal/version"
)

func (a *App) handleEmailRegister(w http.ResponseWriter, r *http.Request) {
	body, err := readJSONMap(r)
	if err != nil {
		util.WriteError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	email := util.Clean(body["email"])
	password := util.Clean(body["password"])
	name := util.Clean(body["name"])
	verifyCode := firstNonEmpty(util.Clean(body["verification_code"]), util.Clean(body["code"]))
	inviteCode := firstNonEmpty(util.Clean(body["invite_code"]), util.Clean(body["invitation_code"]), util.Clean(body["referral_code"]))
	user, key, registerErr := a.billing.RegisterEmailUser(
		email,
		password,
		name,
		verifyCode,
		inviteCode,
		a.config.ImagePriceCents(),
		a.config.RegistrationBonusImageTimes(),
		a.config.EmailAllowedDomains(),
		a.emailSMTPConfig(),
	)
	if registerErr != nil {
		util.WriteError(w, http.StatusBadRequest, registerErr.Error())
		return
	}
	util.WriteJSON(w, http.StatusOK, map[string]any{
		"ok":              true,
		"version":         version.Get(),
		"role":            service.AuthRoleUser,
		"subject_id":      user["id"],
		"name":            user["name"],
		"provider":        service.AuthProviderEmail,
		"key":             key,
		"user":            user,
		"wallet":          walletFromUser(user),
		"image_price":     a.config.ImagePriceCents(),
		"image_prices":    a.imagePricesConfig(),
		"allowed_domains": a.config.EmailAllowedDomains(),
	})
}

func (a *App) handleEmailRegisterCode(w http.ResponseWriter, r *http.Request) {
	body, err := readJSONMap(r)
	if err != nil {
		util.WriteError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	email := util.Clean(body["email"])
	if sendErr := a.billing.SendRegisterCode(email, a.config.EmailAllowedDomains(), a.emailSMTPConfig()); sendErr != nil {
		util.WriteError(w, http.StatusBadRequest, sendErr.Error())
		return
	}
	util.WriteJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"message": "verification code sent",
	})
}

func (a *App) handleEmailLogin(w http.ResponseWriter, r *http.Request) {
	body, err := readJSONMap(r)
	if err != nil {
		util.WriteError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	email := util.Clean(body["email"])
	password := util.Clean(body["password"])
	user, key, loginErr := a.billing.AuthenticateEmailUser(email, password)
	if loginErr != nil {
		util.WriteError(w, http.StatusUnauthorized, loginErr.Error())
		return
	}
	util.WriteJSON(w, http.StatusOK, map[string]any{
		"ok":              true,
		"version":         version.Get(),
		"role":            service.AuthRoleUser,
		"subject_id":      user["id"],
		"name":            user["name"],
		"provider":        service.AuthProviderEmail,
		"key":             key,
		"user":            user,
		"wallet":          walletFromUser(user),
		"image_price":     a.config.ImagePriceCents(),
		"image_prices":    a.imagePricesConfig(),
		"allowed_domains": a.config.EmailAllowedDomains(),
	})
}

func (a *App) handleWallet(w http.ResponseWriter, r *http.Request) {
	identity, ok := a.requireIdentity(w, r, "")
	if !ok {
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	wallet := a.billing.GetWalletByIdentity(identity)
	if wallet == nil {
		util.WriteError(w, http.StatusForbidden, "email wallet permission required")
		return
	}
	util.WriteJSON(w, http.StatusOK, map[string]any{
		"wallet":       wallet,
		"image_price":  a.config.ImagePriceCents(),
		"image_prices": a.imagePricesConfig(),
		"pay_channels": a.availablePayChannels(),
	})
}

func (a *App) handleWalletRedeem(w http.ResponseWriter, r *http.Request) {
	identity, ok := a.requireIdentity(w, r, "")
	if !ok {
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
	code := firstNonEmpty(util.Clean(body["code"]), util.Clean(body["redeem_code"]))
	if code == "" {
		util.WriteError(w, http.StatusBadRequest, "redeem code is required")
		return
	}
	wallet, redeemErr := a.billing.RedeemCode(identity, code)
	if redeemErr != nil {
		util.WriteError(w, http.StatusBadRequest, redeemErr.Error())
		return
	}
	util.WriteJSON(w, http.StatusOK, map[string]any{
		"ok":     true,
		"wallet": wallet,
	})
}

func (a *App) handlePayOrders(w http.ResponseWriter, r *http.Request) {
	identity, ok := a.requireIdentity(w, r, "")
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		query := r.URL.Query()
		limit := util.ToInt(query.Get("limit"), 300)
		if limit < 1 {
			limit = 300
		}
		records := a.billing.ListOrdersByIdentity(identity, limit)
		recordTypeFilter := strings.ToLower(strings.TrimSpace(query.Get("record_type")))
		typeFilter := strings.ToLower(strings.TrimSpace(query.Get("type")))
		statusFilter := strings.ToLower(strings.TrimSpace(query.Get("status")))
		filtered := make([]map[string]any, 0, len(records))
		for _, item := range records {
			recordType := strings.ToLower(strings.TrimSpace(util.Clean(item["record_type"])))
			orderType := strings.ToLower(strings.TrimSpace(util.Clean(item["type"])))
			status := strings.ToLower(strings.TrimSpace(util.Clean(item["status"])))
			if recordTypeFilter != "" && recordTypeFilter != "all" && recordType != recordTypeFilter {
				continue
			}
			if typeFilter != "" && typeFilter != "all" && orderType != typeFilter {
				continue
			}
			if statusFilter != "" && statusFilter != "all" && status != statusFilter {
				continue
			}
			filtered = append(filtered, item)
		}
		pageSize := util.ToInt(query.Get("page_size"), 20)
		if pageSize < 1 {
			pageSize = 20
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
			"total":      total,
			"page":       page,
			"page_size":  pageSize,
			"total_page": totalPages,
		})
	case http.MethodPost:
		body, err := readJSONMap(r)
		if err != nil {
			util.WriteError(w, http.StatusBadRequest, "invalid json body")
			return
		}
		amountCents, amountErr := parseRechargeAmountCents(body)
		if amountErr != nil {
			util.WriteError(w, http.StatusBadRequest, amountErr.Error())
			return
		}
		payType := strings.ToLower(strings.TrimSpace(firstNonEmpty(util.Clean(body["pay_type"]), util.Clean(body["type"]), "alipay")))
		var (
			order    map[string]any
			orderErr error
		)
		order, orderErr = a.billing.CreateYiPayOrder(identity, amountCents, payType, a.yiPayGatewayConfigForPath(r, "/wallet"))
		if orderErr != nil {
			util.WriteError(w, http.StatusBadRequest, orderErr.Error())
			return
		}
		util.WriteJSON(w, http.StatusOK, map[string]any{"order": order})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (a *App) handleYiPayNotify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("fail"))
		return
	}
	gateway := a.yiPayGatewayConfig(r)
	ok, notifyResult, err := a.billing.HandleYiPayNotify(r.Form, gateway)
	if err != nil {
		a.logs.Add("YiPay notify handle failed", map[string]any{
			"status": "failed",
			"error":  err.Error(),
		})
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("fail"))
		return
	}
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("fail"))
		return
	}
	a.activateAgencyFromPayResult(notifyResult, "YiPay notify")
	a.activateSubscriptionFromPayResult(notifyResult, "YiPay notify")
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("success"))
}

func (a *App) handleYiPayReturn(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/wallet", http.StatusFound)
		return
	}
	redirectPath := yipayReturnRedirectPath(r.Form.Get("redirect"))
	verifyValues := cloneURLValuesWithoutKeys(r.Form, "redirect")
	gateway := a.yiPayGatewayConfig(r)
	ok, notifyResult, err := a.billing.HandleYiPayNotify(verifyValues, gateway)
	if err != nil || !ok {
		message := "invalid notify payload"
		if err != nil {
			message = err.Error()
		}
		a.logs.Add("YiPay return handle failed", map[string]any{
			"status":   "failed",
			"error":    message,
			"redirect": redirectPath,
		})
		http.Redirect(w, r, redirectPath, http.StatusFound)
		return
	}
	a.activateAgencyFromPayResult(notifyResult, "YiPay return")
	a.activateSubscriptionFromPayResult(notifyResult, "YiPay return")
	http.Redirect(w, r, redirectPath, http.StatusFound)
}

func (a *App) activateAgencyFromPayResult(result map[string]any, source string) {
	orderKind := strings.TrimSpace(util.Clean(result["order_kind"]))
	if orderKind != service.BillingOrderKindAgencyJoin && orderKind != service.BillingOrderKindAgencyUpgrade {
		return
	}
	tierKey := strings.TrimSpace(util.Clean(result["agency_tier"]))
	userID := strings.TrimSpace(util.Clean(result["user_id"]))
	tier, found := a.agencyTierByKey(tierKey)
	if !found || userID == "" {
		return
	}
	a.ensureAgencyTierRoles()
	if _, activateErr := a.billing.ActivateAgencyByUserID(userID, service.AgencyTierBenefit{
		Tier:         tier.Key,
		CommissionBP: tier.CommissionBP,
		DiscountBP:   tier.DiscountBP,
	}, true); activateErr == nil {
		if roleID, roleFound := a.agencyRoleIDByTier(tier.Key); roleFound {
			_ = a.auth.UpdateUser(userID, map[string]any{"role_id": roleID})
		}
		return
	}
	a.logs.Add(source+" agency activation failed", map[string]any{
		"status":  "failed",
		"user_id": userID,
		"tier":    tierKey,
	})
}

func (a *App) ensureImageBillingCredit(w http.ResponseWriter, identity service.Identity, payload map[string]any) bool {
	if !a.shouldBillImage(identity) {
		return true
	}
	if a.billing.HasActiveSubscription(identity) {
		return true
	}
	if err := a.billing.EnsureCanConsume(identity, a.imageBillingAmountFromPayload(payload)); err != nil {
		util.WriteError(w, http.StatusPaymentRequired, err.Error())
		return false
	}
	return true
}

func (a *App) chargeImageUsage(identity service.Identity, endpoint string, payload map[string]any) error {
	if !a.shouldBillImage(identity) {
		return nil
	}
	if a.billing.HasActiveSubscription(identity) {
		return nil
	}
	tier := a.imageBillingTierFromPayload(payload)
	_, err := a.billing.ConsumeImageUsage(identity, a.imageBillingAmountFromPayload(payload), "image usage "+strings.TrimSpace(endpoint)+" "+tier)
	return err
}

func (a *App) activateSubscriptionFromPayResult(result map[string]any, source string) {
	tier := strings.TrimSpace(util.Clean(result["subscription_tier"]))
	if tier == "" {
		return
	}
	userID := strings.TrimSpace(util.Clean(result["user_id"]))
	if userID == "" {
		return
	}
	a.switchSubscriptionRoleByTier(userID, tier, source)
}

func (a *App) switchSubscriptionRoleByTier(userID, tier, source string) {
	userID = strings.TrimSpace(userID)
	tier = strings.TrimSpace(tier)
	if userID == "" || tier == "" {
		return
	}
	if a.shouldKeepCurrentRoleOnSubscription(userID) {
		return
	}
	a.ensureSubscriptionTierRoles()
	roleID, ok := a.subscriptionRoleIDByTier(tier)
	if !ok {
		return
	}
	if updated := a.auth.UpdateUser(userID, map[string]any{"role_id": roleID}); updated != nil {
		return
	}
	a.logs.Add(source+" subscription role switch failed", map[string]any{
		"status":  "failed",
		"user_id": userID,
		"tier":    tier,
	})
}

func (a *App) shouldKeepCurrentRoleOnSubscription(userID string) bool {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return false
	}
	roleID := ""
	for _, user := range a.auth.ListUsers() {
		if strings.TrimSpace(util.Clean(user["id"])) != userID {
			continue
		}
		roleID = strings.TrimSpace(util.Clean(user["role_id"]))
		break
	}
	if roleID == "" {
		return false
	}
	if roleID == service.AuthRoleAdmin {
		return true
	}
	for _, role := range a.auth.ListRoles() {
		if strings.TrimSpace(util.Clean(role["id"])) != roleID {
			continue
		}
		name := strings.TrimSpace(util.Clean(role["name"]))
		if strings.Contains(name, "代理") {
			return true
		}
		for _, menu := range util.AsStringSlice(role["menu_paths"]) {
			if strings.TrimSpace(menu) == "/agency" || strings.TrimSpace(menu) == "/agency-commission" {
				return true
			}
		}
		break
	}
	return false
}

func (a *App) shouldBillImage(identity service.Identity) bool {
	return identity.Role == service.AuthRoleUser && identity.Provider != service.AuthProviderLinuxDo
}

func (a *App) emailSMTPConfig() service.EmailSMTPConfig {
	raw := a.config.EmailSMTP()
	return service.EmailSMTPConfig{
		Enabled:   raw.Enabled,
		Host:      raw.Host,
		Port:      raw.Port,
		UseSSL:    raw.UseSSL,
		Username:  raw.Username,
		AuthCode:  raw.Password,
		FromEmail: raw.FromEmail,
		FromName:  raw.FromName,
	}
}

func (a *App) yiPayGatewayConfig(r *http.Request) service.YiPayGatewayConfig {
	raw := a.config.YiPay()
	baseURL := a.resolveImageBaseURL(r)
	notifyURL := strings.TrimSpace(raw.NotifyURL)
	if notifyURL == "" {
		notifyURL = strings.TrimRight(baseURL, "/") + "/api/pay/yipay/notify"
	} else {
		trimmed := strings.TrimRight(notifyURL, "/")
		if strings.HasSuffix(trimmed, "/api/pay/yipay") {
			notifyURL = trimmed + "/notify"
		}
	}
	returnURL := strings.TrimSpace(raw.ReturnURL)
	return service.YiPayGatewayConfig{
		Enabled:   raw.Enabled,
		PID:       raw.PID,
		Key:       raw.Key,
		SubmitURL: raw.SubmitURL,
		NotifyURL: notifyURL,
		ReturnURL: returnURL,
		SiteName:  raw.SiteName,
	}
}

func (a *App) yiPayGatewayConfigForPath(r *http.Request, path string) service.YiPayGatewayConfig {
	cfg := a.yiPayGatewayConfig(r)
	baseURL := strings.TrimRight(a.resolveImageBaseURL(r), "/")
	targetPath := strings.TrimSpace(path)
	if targetPath == "" {
		return cfg
	}
	if strings.HasPrefix(targetPath, "http://") || strings.HasPrefix(targetPath, "https://") {
		cfg.ReturnURL = targetPath
		return cfg
	}
	redirectPath := yipayReturnRedirectPath(targetPath)
	q := url.Values{}
	q.Set("redirect", redirectPath)
	cfg.ReturnURL = baseURL + "/api/pay/yipay/return?" + q.Encode()
	return cfg
}

func yipayReturnRedirectPath(path string) string {
	redirectPath := sanitizeFrontendRedirectPath(path)
	if redirectPath == "" {
		return "/wallet"
	}
	return redirectPath
}

func cloneURLValuesWithoutKeys(values url.Values, keys ...string) url.Values {
	out := url.Values{}
	skip := map[string]struct{}{}
	for _, key := range keys {
		key = strings.TrimSpace(key)
		if key != "" {
			skip[key] = struct{}{}
		}
	}
	for key, items := range values {
		if _, ignored := skip[key]; ignored {
			continue
		}
		for _, item := range items {
			out.Add(key, item)
		}
	}
	return out
}

func (a *App) availablePayChannels() []string {
	items := []string{}
	yipay := a.config.YiPay()
	if yipay.Enabled {
		items = append(items, "alipay", "wxpay", "paypal", "usdt")
	}
	return items
}

func (a *App) imagePricesConfig() map[string]any {
	return map[string]any{
		"1k": a.config.ImagePrice1KCents(),
		"2k": a.config.ImagePrice2KCents(),
		"4k": a.config.ImagePrice4KCents(),
	}
}

func (a *App) imageBillingTierFromPayload(payload map[string]any) string {
	if payload == nil {
		return "1k"
	}
	resolution := protocol.NormalizeImageResolutionTier(firstNonEmpty(util.Clean(payload["resolution"]), util.Clean(payload["image_resolution"])))
	if resolution != "" {
		return resolution
	}
	size := protocol.ResolveImageSizeWithResolution(util.Clean(payload["size"]), resolution)
	if tier := protocol.ResolutionTierFromSize(size); tier != "" {
		return tier
	}
	return "1k"
}

func (a *App) imagePriceCentsByTier(tier string) int {
	switch protocol.NormalizeImageResolutionTier(tier) {
	case "2k":
		return a.config.ImagePrice2KCents()
	case "4k":
		return a.config.ImagePrice4KCents()
	default:
		return a.config.ImagePrice1KCents()
	}
}

func (a *App) imageBillingAmountFromPayload(payload map[string]any) int {
	count := util.ToInt(payload["n"], 1)
	if count < 1 {
		count = 1
	}
	if count > 4 {
		count = 4
	}
	return a.imagePriceCentsByTier(a.imageBillingTierFromPayload(payload)) * count
}

func parseRechargeAmountCents(body map[string]any) (int, error) {
	if amountCents := util.ToInt(body["amount_cents"], 0); amountCents > 0 {
		return amountCents, nil
	}
	amountYuan := strings.TrimSpace(util.Clean(body["amount"]))
	if amountYuan == "" {
		return 0, fmt.Errorf("amount is required")
	}
	parsed, err := strconv.ParseFloat(amountYuan, 64)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("amount is invalid")
	}
	return int(parsed*100 + 0.5), nil
}

func walletFromUser(user map[string]any) map[string]any {
	return map[string]any{
		"invite_code":            util.Clean(user["invite_code"]),
		"invited_by":             util.Clean(user["invited_by"]),
		"invited_by_email":       util.Clean(user["invited_by_email"]),
		"balance_cents":          util.ToInt(user["balance_cents"], 0),
		"total_recharge_cents":   util.ToInt(user["total_recharge_cents"], 0),
		"total_consume_cents":    util.ToInt(user["total_consume_cents"], 0),
		"subscription_tier":      util.Clean(user["subscription_tier"]),
		"subscription_active":    util.ToBool(user["subscription_active"]),
		"subscription_expire_at": util.Clean(user["subscription_expire_at"]),
	}
}

func isChargeableImageEndpoint(endpoint string) bool {
	switch strings.TrimSpace(endpoint) {
	case "/v1/images/generations", "/v1/images/edits", "/v1/chat/completions", "/v1/responses", "/api/image-tasks/generations", "/api/image-tasks/edits":
		return true
	default:
		return false
	}
}

func hasImageResult(result map[string]any) bool {
	return len(util.AsMapSlice(result["data"])) > 0
}

func isChargeableChatCompletionsRequest(body map[string]any) bool {
	if util.IsImageGenerationModel(strings.TrimSpace(util.Clean(body["model"]))) {
		return true
	}
	for _, item := range util.AsStringSlice(body["modalities"]) {
		if strings.EqualFold(strings.TrimSpace(item), "image") {
			return true
		}
	}
	return false
}

func isChargeableResponsesRequest(body map[string]any) bool {
	if util.IsImageGenerationModel(strings.TrimSpace(util.Clean(body["model"]))) {
		return true
	}
	tools, _ := body["tools"].([]any)
	for _, raw := range tools {
		tool, _ := raw.(map[string]any)
		if strings.EqualFold(strings.TrimSpace(util.Clean(tool["type"])), "image_generation") {
			return true
		}
	}
	return false
}
