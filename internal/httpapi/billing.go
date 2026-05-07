package httpapi

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"chatgpt2api/internal/service"
	"chatgpt2api/internal/util"
	"chatgpt2api/internal/version"
)

const chatQuestionPriceCents = 10

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
	user, key, registerErr := a.billing.RegisterEmailUser(email, password, name, verifyCode, inviteCode, a.config.ImagePriceCents(), a.config.EmailAllowedDomains(), a.emailSMTPConfig())
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
		util.WriteJSON(w, http.StatusOK, map[string]any{
			"items": a.billing.ListOrdersByIdentity(identity, 30),
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
		order, orderErr = a.billing.CreateYiPayOrder(identity, amountCents, payType, a.yiPayGatewayConfig(r))
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
	ok, err := a.billing.HandleYiPayNotify(r.Form, gateway)
	if err != nil {
		a.logs.Add(service.LogTypeAccount, "YiPay notify handle failed", map[string]any{
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
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("success"))
}

func (a *App) ensureImageBillingCredit(w http.ResponseWriter, identity service.Identity) bool {
	if !a.shouldBillImage(identity) {
		return true
	}
	if err := a.billing.EnsureCanConsume(identity, a.config.ImagePriceCents()); err != nil {
		util.WriteError(w, http.StatusPaymentRequired, err.Error())
		return false
	}
	return true
}

func (a *App) ensureChatBillingCredit(w http.ResponseWriter, identity service.Identity) bool {
	if !a.shouldBillChatText(identity) {
		return true
	}
	if _, err := a.billing.ConsumeImageUsage(identity, chatQuestionPriceCents, "chat question usage /v1/chat/completions"); err != nil {
		util.WriteError(w, http.StatusPaymentRequired, err.Error())
		return false
	}
	return true
}

func (a *App) chargeImageUsage(identity service.Identity, endpoint string) error {
	if !a.shouldBillImage(identity) {
		return nil
	}
	_, err := a.billing.ConsumeImageUsage(identity, a.config.ImagePriceCents(), "image usage "+strings.TrimSpace(endpoint))
	return err
}

func (a *App) shouldBillImage(identity service.Identity) bool {
	return identity.Role == service.AuthRoleUser && identity.Provider == service.AuthProviderEmail
}

func (a *App) shouldBillChatText(identity service.Identity) bool {
	return identity.Role == service.AuthRoleUser && identity.Provider == service.AuthProviderEmail
}

func (a *App) emailSMTPConfig() service.EmailSMTPConfig {
	raw := a.config.EmailSMTP()
	return service.EmailSMTPConfig{
		Enabled:   raw.Enabled,
		Host:      raw.Host,
		Port:      raw.Port,
		UseSSL:    raw.UseSSL,
		Username:  raw.Username,
		AuthCode:  raw.AuthCode,
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

func (a *App) availablePayChannels() []string {
	items := []string{}
	yipay := a.config.YiPay()
	if yipay.Enabled {
		items = append(items, "alipay", "wxpay", "paypal", "usdt")
	}
	return items
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
		"invite_code":          util.Clean(user["invite_code"]),
		"invited_by":           util.Clean(user["invited_by"]),
		"balance_cents":        util.ToInt(user["balance_cents"], 0),
		"total_recharge_cents": util.ToInt(user["total_recharge_cents"], 0),
		"total_consume_cents":  util.ToInt(user["total_consume_cents"], 0),
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
