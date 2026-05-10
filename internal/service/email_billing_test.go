package service

import (
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"chatgpt2api/internal/storage"
)

func TestEmailBillingRegisterLoginConsumeAndRecharge(t *testing.T) {
	dataDir := t.TempDir()
	backend := storage.NewJSONBackend(
		filepath.Join(dataDir, "accounts.json"),
		filepath.Join(dataDir, "auth_keys.json"),
	)
	auth := NewAuthService(backend)
	billing := NewEmailBillingService(dataDir, backend, auth)

	user, key, err := billing.RegisterEmailUser("demo@qq.com", "password123", "demo", "", "", 8, []string{"qq.com", "gmail.com"}, EmailSMTPConfig{})
	if err != nil {
		t.Fatalf("RegisterEmailUser() error = %v", err)
	}
	if key == "" {
		t.Fatal("RegisterEmailUser() returned empty key")
	}
	if user["provider"] != AuthProviderEmail || user["role"] != AuthRoleUser {
		t.Fatalf("registered user = %#v", user)
	}

	if _, _, err := billing.RegisterEmailUser("demo@qq.com", "password123", "demo", "", "", 8, []string{"qq.com"}, EmailSMTPConfig{}); err == nil {
		t.Fatal("duplicate email register should fail")
	}
	if _, _, err := billing.RegisterEmailUser("bad@unknown.test", "password123", "demo", "", "", 8, []string{"qq.com"}, EmailSMTPConfig{}); err == nil {
		t.Fatal("unexpected success for disallowed email domain")
	}

	loginUser, loginKey, err := billing.AuthenticateEmailUser("demo@qq.com", "password123")
	if err != nil {
		t.Fatalf("AuthenticateEmailUser() error = %v", err)
	}
	if loginKey != key {
		t.Fatalf("loginKey = %q, want %q", loginKey, key)
	}
	if loginUser["id"] != user["id"] {
		t.Fatalf("login user = %#v, want id=%v", loginUser, user["id"])
	}

	identity := auth.Authenticate(key)
	if identity == nil || identity.Provider != AuthProviderEmail {
		t.Fatalf("Authenticate(key) identity = %#v", identity)
	}
	wallet := billing.GetWalletByIdentity(*identity)
	if wallet == nil || wallet["balance_cents"] != 160 {
		t.Fatalf("initial wallet = %#v", wallet)
	}

	gateway := YiPayGatewayConfig{
		Enabled:   true,
		PID:       "1001",
		Key:       "secret",
		SubmitURL: "https://pay.example.com/submit.php",
		NotifyURL: "https://api.example.com/api/pay/yipay/notify",
		ReturnURL: "https://api.example.com/image",
		SiteName:  "chatgpt2api",
	}
	order, err := billing.CreateYiPayOrder(*identity, 500, "alipay", gateway)
	if err != nil {
		t.Fatalf("CreateYiPayOrder() error = %v", err)
	}
	outTradeNo, _ := order["out_trade_no"].(string)
	if outTradeNo == "" {
		t.Fatalf("CreateYiPayOrder() returned invalid order: %#v", order)
	}

	notify := url.Values{}
	notify.Set("pid", gateway.PID)
	notify.Set("type", "alipay")
	notify.Set("out_trade_no", outTradeNo)
	notify.Set("trade_no", "trade_123")
	notify.Set("trade_status", "TRADE_SUCCESS")
	notify.Set("money", "5.00")
	params := map[string]string{}
	for key, values := range notify {
		params[key] = values[0]
	}
	notify.Set("sign_type", "MD5")
	notify.Set("sign", yipaySign(params, gateway.Key))

	ok, err := billing.HandleYiPayNotify(notify, gateway)
	if err != nil || !ok {
		t.Fatalf("HandleYiPayNotify() ok=%v err=%v", ok, err)
	}
	ok, err = billing.HandleYiPayNotify(notify, gateway)
	if err != nil || !ok {
		t.Fatalf("HandleYiPayNotify() idempotent ok=%v err=%v", ok, err)
	}

	wallet = billing.GetWalletByIdentity(*identity)
	if wallet == nil || wallet["balance_cents"] != 660 {
		t.Fatalf("wallet after recharge = %#v", wallet)
	}
	if err := billing.EnsureCanConsume(*identity, 8); err != nil {
		t.Fatalf("EnsureCanConsume() after recharge error = %v", err)
	}
	if _, err := billing.ConsumeImageUsage(*identity, 8, "/v1/images/generations"); err != nil {
		t.Fatalf("ConsumeImageUsage() error = %v", err)
	}
	wallet = billing.GetWalletByIdentity(*identity)
	if wallet == nil || wallet["balance_cents"] != 652 {
		t.Fatalf("wallet after consume = %#v", wallet)
	}
}

func TestEmailBillingRegisterRequiresVerificationCodeWhenSMTPEnabled(t *testing.T) {
	dataDir := t.TempDir()
	backend := storage.NewJSONBackend(
		filepath.Join(dataDir, "accounts.json"),
		filepath.Join(dataDir, "auth_keys.json"),
	)
	auth := NewAuthService(backend)
	billing := NewEmailBillingService(dataDir, backend, auth)

	smtp := EmailSMTPConfig{
		Enabled:   true,
		Host:      "smtp.qq.com",
		Port:      465,
		UseSSL:    true,
		Username:  "sender@qq.com",
		AuthCode:  "auth-code",
		FromEmail: "sender@qq.com",
		FromName:  "chatgpt2api",
	}
	if _, _, err := billing.RegisterEmailUser("new@qq.com", "password123", "new", "", "", 8, []string{"qq.com"}, smtp); err == nil {
		t.Fatal("register should require verification code when smtp enabled")
	}

	code := "123456"
	billing.registerCodes["new@qq.com"] = &billingRegisterCode{
		Email:      "new@qq.com",
		CodeHash:   hashRegisterCode(code),
		ExpiresAt:  time.Now().UTC().Add(10 * time.Minute).Format(time.RFC3339Nano),
		LastSentAt: time.Now().UTC().Format(time.RFC3339Nano),
		SendCount:  1,
	}
	user, key, err := billing.RegisterEmailUser("new@qq.com", "password123", "new", code, "", 8, []string{"qq.com"}, smtp)
	if err != nil {
		t.Fatalf("RegisterEmailUser() with code error = %v", err)
	}
	if user["email"] != "new@qq.com" || key == "" {
		t.Fatalf("register result = %#v key=%q", user, key)
	}
}

func TestEmailBillingInviteBonus(t *testing.T) {
	dataDir := t.TempDir()
	backend := storage.NewJSONBackend(
		filepath.Join(dataDir, "accounts.json"),
		filepath.Join(dataDir, "auth_keys.json"),
	)
	auth := NewAuthService(backend)
	billing := NewEmailBillingService(dataDir, backend, auth)

	inviter, inviterKey, err := billing.RegisterEmailUser("inviter@qq.com", "password123", "inviter", "", "", 8, []string{"qq.com"}, EmailSMTPConfig{})
	if err != nil {
		t.Fatalf("register inviter error = %v", err)
	}
	inviteCode, _ := inviter["invite_code"].(string)
	if inviteCode == "" {
		t.Fatalf("invite code missing: %#v", inviter)
	}
	if _, _, err := billing.RegisterEmailUser("newuser@qq.com", "password123", "new", "", inviteCode, 8, []string{"qq.com"}, EmailSMTPConfig{}); err != nil {
		t.Fatalf("register invitee error = %v", err)
	}
	identity := auth.Authenticate(inviterKey)
	if identity == nil {
		t.Fatal("inviter identity should not be nil")
	}
	wallet := billing.GetWalletByIdentity(*identity)
	if wallet == nil {
		t.Fatal("inviter wallet should not be nil")
	}
	if wallet["balance_cents"] != 240 {
		t.Fatalf("inviter wallet balance = %#v, want 240", wallet)
	}
}
