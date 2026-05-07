package service

import (
	"crypto/md5"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"chatgpt2api/internal/storage"
	"chatgpt2api/internal/util"

	"golang.org/x/crypto/bcrypt"
)

const (
	BillingProviderYiPay         = "yipay"
	BillingProviderPayPal        = "paypal"
	BillingProviderUSDT          = "usdt"
	BillingProviderRedeemCode    = "redeem_code"
	BillingProviderAdmin         = "admin_adjust"
	BillingProviderRegisterBonus = "register_bonus"
	BillingProviderInviteBonus   = "invite_bonus"

	BillingOrderStatusPending = "pending"
	BillingOrderStatusPaid    = "paid"
	BillingOrderStatusFailed  = "failed"

	BillingTxTypeRecharge = "recharge"
	BillingTxTypeConsume  = "consume"
	BillingTxTypeAdjust   = "adjust"

	RegisterCodeExpireMinutes = 10
	RegisterCodeResendSeconds = 60

	RegisterBonusImageTimes = 20
	InviteBonusImageTimes   = 10
)

var emailRE = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

type YiPayGatewayConfig struct {
	Enabled   bool
	PID       string
	Key       string
	SubmitURL string
	NotifyURL string
	ReturnURL string
	SiteName  string
}

type PayPalGatewayConfig struct {
	Enabled     bool
	CheckoutURL string
}

type USDTGatewayConfig struct {
	Enabled    bool
	Network    string
	Address    string
	PaymentURL string
}

type EmailBillingService struct {
	mu               sync.Mutex
	path             string
	store            storage.JSONDocumentBackend
	docName          string
	auth             *AuthService
	users            map[string]*billingUser
	userByEmail      map[string]string
	userByInviteCode map[string]string
	registerCodes    map[string]*billingRegisterCode
	redeemCodes      map[string]*billingRedeemCode
	orders           map[string]*billingOrder
	transactions     []map[string]any
}

type billingRegisterCode struct {
	Email      string
	CodeHash   string
	ExpiresAt  string
	LastSentAt string
	SendCount  int
}

type billingRedeemCode struct {
	Code        string
	AmountCents int
	Enabled     bool
	CreatedAt   string
	UpdatedAt   string
	ExpiresAt   string
	UsedBy      string
	UsedAt      string
	Note        string
}

type billingUser struct {
	ID                 string
	Email              string
	Provider           string
	Name               string
	InviteCode         string
	InvitedBy          string
	PasswordHash       string
	AuthKeyID          string
	Enabled            bool
	BalanceCents       int
	TotalRechargeCents int
	TotalConsumeCents  int
	CreatedAt          string
	UpdatedAt          string
	LastLoginAt        string
}

type billingOrder struct {
	ID          string
	OutTradeNo  string
	TradeNo     string
	Provider    string
	UserID      string
	UserEmail   string
	PayType     string
	AmountCents int
	Status      string
	CreatedAt   string
	UpdatedAt   string
	PaidAt      string
}

func NewEmailBillingService(dataDir string, backend storage.Backend, auth *AuthService) *EmailBillingService {
	s := &EmailBillingService{
		path:             filepath.Join(dataDir, "billing.json"),
		store:            jsonDocumentStoreFromBackend(backend),
		docName:          "billing.json",
		auth:             auth,
		users:            map[string]*billingUser{},
		userByEmail:      map[string]string{},
		userByInviteCode: map[string]string{},
		registerCodes:    map[string]*billingRegisterCode{},
		redeemCodes:      map[string]*billingRedeemCode{},
		orders:           map[string]*billingOrder{},
	}
	s.mu.Lock()
	s.loadLocked()
	s.mu.Unlock()
	return s
}

func (s *EmailBillingService) RegisterEmailUser(email, password, name, verifyCode, inviteCode string, imagePriceCents int, allowedDomains []string, smtpConfig EmailSMTPConfig) (map[string]any, string, error) {
	normalizedEmail, err := normalizeEmail(email)
	if err != nil {
		return nil, "", err
	}
	if err := validateEmailDomain(normalizedEmail, allowedDomains); err != nil {
		return nil, "", err
	}
	if err := validatePassword(password); err != nil {
		return nil, "", err
	}

	requireVerifyCode := smtpConfig.Ready()
	if requireVerifyCode {
		verifyCode = strings.TrimSpace(verifyCode)
		if verifyCode == "" {
			return nil, "", fmt.Errorf("verification code is required")
		}
	}
	displayName := strings.TrimSpace(name)
	if displayName == "" {
		displayName = normalizedEmail
	}
	passwordHashBytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, "", fmt.Errorf("failed to hash password")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.userByEmail[normalizedEmail]; exists {
		return nil, "", fmt.Errorf("email already registered")
	}
	normalizedInviteCode := normalizeInviteCode(inviteCode)
	var inviter *billingUser
	if normalizedInviteCode != "" {
		inviterID := strings.TrimSpace(s.userByInviteCode[normalizedInviteCode])
		if inviterID == "" {
			return nil, "", fmt.Errorf("invalid invite code")
		}
		inviter = s.users[inviterID]
		if inviter == nil || !inviter.Enabled {
			return nil, "", fmt.Errorf("invalid invite code")
		}
	}
	if requireVerifyCode {
		if verifyErr := s.verifyRegisterCodeLocked(normalizedEmail, verifyCode); verifyErr != nil {
			return nil, "", verifyErr
		}
	}

	userID := "email:" + util.NewHex(16)
	item, rawKey, err := s.auth.CreateAPIKey(
		AuthRoleUser,
		displayName,
		AuthOwner{
			ID:       userID,
			Name:     displayName,
			Provider: AuthProviderEmail,
		},
	)
	if err != nil {
		return nil, "", err
	}
	authKeyID := util.Clean(item["id"])
	if authKeyID == "" || rawKey == "" {
		return nil, "", fmt.Errorf("failed to create auth key")
	}

	now := util.NowISO()
	generatedInviteCode := s.newInviteCodeLocked(userID)
	registerBonusCents := maxBillingInt(0, imagePriceCents) * RegisterBonusImageTimes
	inviteBonusCents := maxBillingInt(0, imagePriceCents) * InviteBonusImageTimes
	user := &billingUser{
		ID:                 userID,
		Email:              normalizedEmail,
		Provider:           AuthProviderEmail,
		Name:               displayName,
		InviteCode:         generatedInviteCode,
		InvitedBy:          "",
		PasswordHash:       string(passwordHashBytes),
		AuthKeyID:          authKeyID,
		Enabled:            true,
		BalanceCents:       registerBonusCents,
		TotalRechargeCents: registerBonusCents,
		TotalConsumeCents:  0,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if inviter != nil {
		if inviter.ID == userID {
			return nil, "", fmt.Errorf("invalid invite code")
		}
		user.InvitedBy = inviter.InviteCode
	}
	s.users[userID] = user
	s.userByEmail[normalizedEmail] = userID
	s.userByInviteCode[user.InviteCode] = user.ID
	appendTxCount := 0
	if registerBonusCents > 0 {
		s.transactions = append(s.transactions, map[string]any{
			"id":                  "tx_" + util.NewHex(18),
			"user_id":             user.ID,
			"email":               user.Email,
			"type":                BillingTxTypeRecharge,
			"amount_cents":        registerBonusCents,
			"balance_after_cents": user.BalanceCents,
			"provider":            BillingProviderRegisterBonus,
			"note":                fmt.Sprintf("new user bonus (%d image credits)", RegisterBonusImageTimes),
			"created_at":          now,
		})
		appendTxCount++
	}
	if inviter != nil && inviteBonusCents > 0 {
		inviter.BalanceCents += inviteBonusCents
		inviter.TotalRechargeCents += inviteBonusCents
		inviter.UpdatedAt = now
		s.transactions = append(s.transactions, map[string]any{
			"id":                  "tx_" + util.NewHex(18),
			"user_id":             inviter.ID,
			"email":               inviter.Email,
			"type":                BillingTxTypeRecharge,
			"amount_cents":        inviteBonusCents,
			"balance_after_cents": inviter.BalanceCents,
			"provider":            BillingProviderInviteBonus,
			"note":                fmt.Sprintf("invite bonus from %s (%d image credits)", user.Email, InviteBonusImageTimes),
			"created_at":          now,
		})
		appendTxCount++
	}
	delete(s.registerCodes, normalizedEmail)
	if err := s.saveLocked(); err != nil {
		delete(s.users, userID)
		delete(s.userByEmail, normalizedEmail)
		delete(s.userByInviteCode, user.InviteCode)
		if inviter != nil && inviteBonusCents > 0 {
			inviter.BalanceCents -= inviteBonusCents
			inviter.TotalRechargeCents -= inviteBonusCents
			inviter.UpdatedAt = util.NowISO()
		}
		if appendTxCount > 0 && len(s.transactions) >= appendTxCount {
			s.transactions = s.transactions[:len(s.transactions)-appendTxCount]
		}
		_ = s.auth.DeleteKey(authKeyID, AuthKeyFilter{
			Role:    AuthRoleUser,
			Kind:    AuthKindAPIKey,
			OwnerID: userID,
		})
		return nil, "", err
	}
	return publicBillingUser(user), rawKey, nil
}

func (s *EmailBillingService) SendRegisterCode(email string, allowedDomains []string, smtpConfig EmailSMTPConfig) error {
	if !smtpConfig.Ready() {
		return fmt.Errorf("email smtp is not configured")
	}
	normalizedEmail, err := normalizeEmail(email)
	if err != nil {
		return err
	}
	if err := validateEmailDomain(normalizedEmail, allowedDomains); err != nil {
		return err
	}

	now := time.Now().UTC()
	s.mu.Lock()
	if _, exists := s.userByEmail[normalizedEmail]; exists {
		s.mu.Unlock()
		return fmt.Errorf("email already registered")
	}
	if item := s.registerCodes[normalizedEmail]; item != nil {
		lastSentAt, _ := time.Parse(time.RFC3339Nano, strings.TrimSpace(item.LastSentAt))
		if !lastSentAt.IsZero() {
			nextAllowedAt := lastSentAt.Add(RegisterCodeResendSeconds * time.Second)
			if now.Before(nextAllowedAt) {
				remaining := int(nextAllowedAt.Sub(now).Seconds())
				if remaining < 1 {
					remaining = 1
				}
				s.mu.Unlock()
				return fmt.Errorf("please wait %d seconds before requesting another code", remaining)
			}
		}
	}
	s.mu.Unlock()

	code, err := generateRegisterCode6()
	if err != nil {
		return err
	}
	subject := "chatgpt2api verification code"
	content := fmt.Sprintf("Your verification code is: %s\nValid for %d minutes.", code, RegisterCodeExpireMinutes)
	if mailErr := SendSMTPMail(smtpConfig, normalizedEmail, subject, content); mailErr != nil {
		return mailErr
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	item := s.registerCodes[normalizedEmail]
	if item == nil {
		item = &billingRegisterCode{
			Email: normalizedEmail,
		}
		s.registerCodes[normalizedEmail] = item
	}
	item.CodeHash = hashRegisterCode(code)
	item.ExpiresAt = now.Add(RegisterCodeExpireMinutes * time.Minute).Format(time.RFC3339Nano)
	item.LastSentAt = now.Format(time.RFC3339Nano)
	item.SendCount++
	return s.saveLocked()
}

func (s *EmailBillingService) AuthenticateEmailUser(email, password string) (map[string]any, string, error) {
	normalizedEmail, err := normalizeEmail(email)
	if err != nil {
		return nil, "", err
	}
	password = strings.TrimSpace(password)
	if password == "" {
		return nil, "", fmt.Errorf("password is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	user := s.userByEmailLocked(normalizedEmail)
	if user == nil {
		return nil, "", fmt.Errorf("email or password is invalid")
	}
	if !user.Enabled {
		return nil, "", fmt.Errorf("account is disabled")
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		return nil, "", fmt.Errorf("email or password is invalid")
	}
	key, found := s.auth.RevealKey(user.AuthKeyID, AuthKeyFilter{
		Role:    AuthRoleUser,
		Kind:    AuthKindAPIKey,
		OwnerID: user.ID,
	})
	if !found || key == "" {
		return nil, "", fmt.Errorf("auth key is unavailable")
	}
	user.LastLoginAt = util.NowISO()
	user.UpdatedAt = user.LastLoginAt
	if err := s.saveLocked(); err != nil {
		return nil, "", err
	}
	return publicBillingUser(user), key, nil
}

func (s *EmailBillingService) GetWalletByIdentity(identity Identity) map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	user := s.ensureUserByIdentityLocked(identity)
	if user == nil {
		return nil
	}
	invitees, inviteeCount := s.inviteesForUserLocked(user, 200)
	return map[string]any{
		"user_id":               user.ID,
		"email":                 user.Email,
		"name":                  user.Name,
		"invite_code":           user.InviteCode,
		"invited_by":            user.InvitedBy,
		"invitee_count":         inviteeCount,
		"invitees":              invitees,
		"balance_cents":         user.BalanceCents,
		"total_recharge_cents":  user.TotalRechargeCents,
		"total_consume_cents":   user.TotalConsumeCents,
		"image_price_note":      "amount is in cents",
		"last_login_at":         user.LastLoginAt,
		"updated_at":            user.UpdatedAt,
		"billing_provider_hint": BillingProviderYiPay,
	}
}

func (s *EmailBillingService) ListUsersForAdmin() []map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := make([]*billingUser, 0, len(s.users))
	for _, user := range s.users {
		items = append(items, user)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].CreatedAt != items[j].CreatedAt {
			return items[i].CreatedAt > items[j].CreatedAt
		}
		return items[i].Email < items[j].Email
	})
	out := make([]map[string]any, 0, len(items))
	for _, user := range items {
		out = append(out, publicBillingUser(user))
	}
	return out
}

func (s *EmailBillingService) GetWalletByUserID(userID string) map[string]any {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	user := s.users[userID]
	if user == nil {
		return nil
	}
	return map[string]any{
		"user_id":              user.ID,
		"email":                user.Email,
		"name":                 user.Name,
		"invite_code":          user.InviteCode,
		"invited_by":           user.InvitedBy,
		"balance_cents":        user.BalanceCents,
		"total_recharge_cents": user.TotalRechargeCents,
		"total_consume_cents":  user.TotalConsumeCents,
		"last_login_at":        user.LastLoginAt,
		"updated_at":           user.UpdatedAt,
	}
}

func (s *EmailBillingService) EnsureWalletUser(userID, name, provider string) map[string]any {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	user := s.ensureUserByIDLocked(userID, name, provider)
	if user == nil {
		return nil
	}
	return map[string]any{
		"user_id":              user.ID,
		"email":                user.Email,
		"name":                 user.Name,
		"invite_code":          user.InviteCode,
		"invited_by":           user.InvitedBy,
		"balance_cents":        user.BalanceCents,
		"total_recharge_cents": user.TotalRechargeCents,
		"total_consume_cents":  user.TotalConsumeCents,
		"last_login_at":        user.LastLoginAt,
		"updated_at":           user.UpdatedAt,
	}
}

func (s *EmailBillingService) AdminAdjustUserBalance(userID string, deltaCents int, note string) (map[string]any, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, fmt.Errorf("user id is required")
	}
	if deltaCents == 0 {
		return nil, fmt.Errorf("delta_cents cannot be 0")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	user := s.users[userID]
	if user == nil {
		return nil, fmt.Errorf("user not found")
	}
	nextBalance := user.BalanceCents + deltaCents
	if nextBalance < 0 {
		s.appendBillingFailureLocked(user, BillingTxTypeAdjust, BillingProviderAdmin, 0, "balance cannot be negative")
		return nil, fmt.Errorf("balance cannot be negative")
	}
	user.BalanceCents = nextBalance
	if deltaCents > 0 {
		user.TotalRechargeCents += deltaCents
	} else {
		user.TotalConsumeCents += -deltaCents
	}
	user.UpdatedAt = util.NowISO()
	tx := map[string]any{
		"id":                  "tx_" + util.NewHex(18),
		"user_id":             user.ID,
		"email":               user.Email,
		"type":                BillingTxTypeAdjust,
		"amount_cents":        deltaCents,
		"balance_after_cents": user.BalanceCents,
		"provider":            BillingProviderAdmin,
		"note":                strings.TrimSpace(note),
		"created_at":          util.NowISO(),
	}
	s.transactions = append(s.transactions, tx)
	if err := s.saveLocked(); err != nil {
		user.BalanceCents -= deltaCents
		if deltaCents > 0 {
			user.TotalRechargeCents -= deltaCents
		} else {
			user.TotalConsumeCents -= -deltaCents
		}
		user.UpdatedAt = util.NowISO()
		if len(s.transactions) > 0 {
			s.transactions = s.transactions[:len(s.transactions)-1]
		}
		return nil, err
	}
	return map[string]any{
		"user_id":              user.ID,
		"email":                user.Email,
		"name":                 user.Name,
		"balance_cents":        user.BalanceCents,
		"total_recharge_cents": user.TotalRechargeCents,
		"total_consume_cents":  user.TotalConsumeCents,
		"updated_at":           user.UpdatedAt,
	}, nil
}

func (s *EmailBillingService) AdminSetUserBalance(userID string, balanceCents int, note string) (map[string]any, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, fmt.Errorf("user id is required")
	}
	if balanceCents < 0 {
		return nil, fmt.Errorf("balance_cents cannot be negative")
	}
	s.mu.Lock()
	user := s.users[userID]
	currentBalance := 0
	if user != nil {
		currentBalance = user.BalanceCents
	}
	s.mu.Unlock()
	if user == nil {
		return nil, fmt.Errorf("user not found")
	}
	delta := balanceCents - currentBalance
	if delta == 0 {
		return s.GetWalletByUserID(userID), nil
	}
	return s.AdminAdjustUserBalance(userID, delta, note)
}

func (s *EmailBillingService) RedeemCode(identity Identity, code string) (map[string]any, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return nil, fmt.Errorf("redeem code is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	user := s.ensureUserByIdentityLocked(identity)
	if user == nil {
		return nil, fmt.Errorf("user identity required")
	}
	item := s.redeemCodes[code]
	if item == nil {
		s.appendBillingFailureLocked(user, BillingTxTypeRecharge, BillingProviderRedeemCode, 0, "redeem code is invalid")
		return nil, fmt.Errorf("redeem code is invalid")
	}
	if !item.Enabled {
		s.appendBillingFailureLocked(user, BillingTxTypeRecharge, BillingProviderRedeemCode, 0, "redeem code is disabled")
		return nil, fmt.Errorf("redeem code is disabled")
	}
	if item.UsedBy != "" {
		s.appendBillingFailureLocked(user, BillingTxTypeRecharge, BillingProviderRedeemCode, 0, "redeem code has already been used")
		return nil, fmt.Errorf("redeem code has already been used")
	}
	if item.ExpiresAt != "" {
		expiresAt, err := time.Parse(time.RFC3339Nano, item.ExpiresAt)
		if err != nil || time.Now().UTC().After(expiresAt) {
			s.appendBillingFailureLocked(user, BillingTxTypeRecharge, BillingProviderRedeemCode, 0, "redeem code has expired")
			return nil, fmt.Errorf("redeem code has expired")
		}
	}
	user.BalanceCents += item.AmountCents
	user.TotalRechargeCents += item.AmountCents
	user.UpdatedAt = util.NowISO()
	item.UsedBy = user.ID
	item.UsedAt = util.NowISO()
	item.UpdatedAt = item.UsedAt
	tx := map[string]any{
		"id":                  "tx_" + util.NewHex(18),
		"user_id":             user.ID,
		"email":               user.Email,
		"type":                BillingTxTypeRecharge,
		"amount_cents":        item.AmountCents,
		"balance_after_cents": user.BalanceCents,
		"provider":            BillingProviderRedeemCode,
		"note":                "redeem code " + item.Code,
		"created_at":          util.NowISO(),
	}
	s.transactions = append(s.transactions, tx)
	if err := s.saveLocked(); err != nil {
		user.BalanceCents -= item.AmountCents
		user.TotalRechargeCents -= item.AmountCents
		user.UpdatedAt = util.NowISO()
		item.UsedBy = ""
		item.UsedAt = ""
		item.UpdatedAt = util.NowISO()
		if len(s.transactions) > 0 {
			s.transactions = s.transactions[:len(s.transactions)-1]
		}
		return nil, err
	}
	return map[string]any{
		"user_id":              user.ID,
		"email":                user.Email,
		"name":                 user.Name,
		"balance_cents":        user.BalanceCents,
		"total_recharge_cents": user.TotalRechargeCents,
		"total_consume_cents":  user.TotalConsumeCents,
		"updated_at":           user.UpdatedAt,
		"redeem_code":          item.Code,
		"amount_cents":         item.AmountCents,
	}, nil
}

func (s *EmailBillingService) ListRedeemCodes(limit int) []map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := make([]*billingRedeemCode, 0, len(s.redeemCodes))
	for _, item := range s.redeemCodes {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].CreatedAt != items[j].CreatedAt {
			return items[i].CreatedAt > items[j].CreatedAt
		}
		return items[i].Code < items[j].Code
	})
	if limit < 1 || limit > 5000 {
		limit = 200
	}
	out := make([]map[string]any, 0, minBillingInt(limit, len(items)))
	for _, item := range items {
		out = append(out, publicBillingRedeemCode(item))
		if len(out) >= limit {
			break
		}
	}
	return out
}

func (s *EmailBillingService) CreateRedeemCodes(amountCents, count int, expiresAt, note string) ([]map[string]any, error) {
	if amountCents < 1 {
		return nil, fmt.Errorf("amount_cents must be greater than 0")
	}
	if count < 1 || count > 500 {
		return nil, fmt.Errorf("count must be between 1 and 500")
	}
	expiresAt = strings.TrimSpace(expiresAt)
	if expiresAt != "" {
		t, err := time.Parse(time.RFC3339Nano, expiresAt)
		if err != nil {
			return nil, fmt.Errorf("expires_at must be RFC3339 time")
		}
		if time.Now().UTC().After(t) {
			return nil, fmt.Errorf("expires_at must be in the future")
		}
	}
	now := util.NowISO()
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]map[string]any, 0, count)
	for i := 0; i < count; i++ {
		code := s.newRedeemCodeLocked()
		item := &billingRedeemCode{
			Code:        code,
			AmountCents: amountCents,
			Enabled:     true,
			CreatedAt:   now,
			UpdatedAt:   now,
			ExpiresAt:   expiresAt,
			UsedBy:      "",
			UsedAt:      "",
			Note:        strings.TrimSpace(note),
		}
		s.redeemCodes[code] = item
		out = append(out, publicBillingRedeemCode(item))
	}
	if err := s.saveLocked(); err != nil {
		for _, item := range out {
			delete(s.redeemCodes, strings.ToUpper(strings.TrimSpace(util.Clean(item["code"]))))
		}
		return nil, err
	}
	return out, nil
}

func (s *EmailBillingService) UpdateRedeemCode(code string, enabled *bool, note *string, expiresAt *string) (map[string]any, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return nil, fmt.Errorf("code is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	item := s.redeemCodes[code]
	if item == nil {
		return nil, fmt.Errorf("redeem code not found")
	}
	if enabled != nil {
		item.Enabled = *enabled
	}
	if note != nil {
		item.Note = strings.TrimSpace(*note)
	}
	if expiresAt != nil {
		value := strings.TrimSpace(*expiresAt)
		if value != "" {
			t, err := time.Parse(time.RFC3339Nano, value)
			if err != nil {
				return nil, fmt.Errorf("expires_at must be RFC3339 time")
			}
			if time.Now().UTC().After(t) {
				return nil, fmt.Errorf("expires_at must be in the future")
			}
		}
		item.ExpiresAt = value
	}
	item.UpdatedAt = util.NowISO()
	if err := s.saveLocked(); err != nil {
		return nil, err
	}
	return publicBillingRedeemCode(item), nil
}

func (s *EmailBillingService) DeleteRedeemCode(code string) bool {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.redeemCodes[code]; !exists {
		return false
	}
	delete(s.redeemCodes, code)
	_ = s.saveLocked()
	return true
}

func (s *EmailBillingService) EnsureCanConsume(identity Identity, amountCents int) error {
	if amountCents < 1 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	user := s.ensureUserByIdentityLocked(identity)
	if user == nil {
		return nil
	}
	if !user.Enabled {
		return fmt.Errorf("account is disabled")
	}
	if user.BalanceCents < amountCents {
		return fmt.Errorf("insufficient balance, please recharge first")
	}
	return nil
}

func (s *EmailBillingService) ConsumeImageUsage(identity Identity, amountCents int, note string) (map[string]any, error) {
	if amountCents < 1 {
		return nil, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	user := s.ensureUserByIdentityLocked(identity)
	if user == nil {
		return nil, nil
	}
	if !user.Enabled {
		return nil, fmt.Errorf("account is disabled")
	}
	if user.BalanceCents < amountCents {
		return nil, fmt.Errorf("insufficient balance, please recharge first")
	}
	user.BalanceCents -= amountCents
	user.TotalConsumeCents += amountCents
	user.UpdatedAt = util.NowISO()
	tx := map[string]any{
		"id":                  "tx_" + util.NewHex(18),
		"user_id":             user.ID,
		"email":               user.Email,
		"type":                BillingTxTypeConsume,
		"amount_cents":        -amountCents,
		"balance_after_cents": user.BalanceCents,
		"note":                strings.TrimSpace(note),
		"created_at":          util.NowISO(),
	}
	s.transactions = append(s.transactions, tx)
	if err := s.saveLocked(); err != nil {
		user.BalanceCents += amountCents
		user.TotalConsumeCents -= amountCents
		user.UpdatedAt = util.NowISO()
		if len(s.transactions) > 0 {
			s.transactions = s.transactions[:len(s.transactions)-1]
		}
		return nil, err
	}
	return tx, nil
}

func (s *EmailBillingService) CreateYiPayOrder(identity Identity, amountCents int, payType string, cfg YiPayGatewayConfig) (map[string]any, error) {
	if !cfg.Enabled || strings.TrimSpace(cfg.PID) == "" || strings.TrimSpace(cfg.Key) == "" || strings.TrimSpace(cfg.SubmitURL) == "" {
		return nil, fmt.Errorf("YiPay is not configured")
	}
	if amountCents < 1 {
		return nil, fmt.Errorf("amount must be greater than 0")
	}
	payType = strings.ToLower(strings.TrimSpace(payType))
	switch payType {
	case "alipay", "wxpay", "paypal", "usdt":
	default:
		return nil, fmt.Errorf("unsupported pay type")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	user := s.ensureUserByIdentityLocked(identity)
	if user == nil {
		return nil, fmt.Errorf("user identity required")
	}
	if !user.Enabled {
		return nil, fmt.Errorf("account is disabled")
	}
	now := util.NowISO()
	outTradeNo := s.newOutTradeNoLocked()
	order := &billingOrder{
		ID:          "ord_" + util.NewHex(18),
		OutTradeNo:  outTradeNo,
		Provider:    BillingProviderYiPay,
		UserID:      user.ID,
		UserEmail:   user.Email,
		PayType:     payType,
		AmountCents: amountCents,
		Status:      BillingOrderStatusPending,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	params := map[string]string{
		"pid":          strings.TrimSpace(cfg.PID),
		"type":         payType,
		"out_trade_no": outTradeNo,
		"notify_url":   strings.TrimSpace(cfg.NotifyURL),
		"return_url":   strings.TrimSpace(cfg.ReturnURL),
		"name":         "chatgpt2api wallet recharge",
		"money":        centsToYuan(amountCents),
		"sitename":     firstNonEmpty(strings.TrimSpace(cfg.SiteName), "chatgpt2api"),
		"param":        user.ID,
	}
	sign := yipaySign(params, cfg.Key)
	values := url.Values{}
	for key, value := range params {
		if strings.TrimSpace(value) != "" {
			values.Set(key, value)
		}
	}
	values.Set("sign", sign)
	values.Set("sign_type", "MD5")

	submitURL := strings.TrimRight(strings.TrimSpace(cfg.SubmitURL), "?")
	payURL := submitURL
	if strings.Contains(submitURL, "?") {
		payURL += "&" + values.Encode()
	} else {
		payURL += "?" + values.Encode()
	}

	s.orders[outTradeNo] = order
	if err := s.saveLocked(); err != nil {
		delete(s.orders, outTradeNo)
		return nil, err
	}
	return map[string]any{
		"id":           order.ID,
		"provider":     order.Provider,
		"status":       order.Status,
		"out_trade_no": order.OutTradeNo,
		"user_id":      order.UserID,
		"user_email":   order.UserEmail,
		"pay_type":     order.PayType,
		"amount_cents": order.AmountCents,
		"amount_yuan":  centsToYuan(order.AmountCents),
		"pay_url":      payURL,
		"created_at":   order.CreatedAt,
		"updated_at":   order.UpdatedAt,
	}, nil
}

func (s *EmailBillingService) CreatePayPalOrder(identity Identity, amountCents int, cfg PayPalGatewayConfig) (map[string]any, error) {
	if !cfg.Enabled || strings.TrimSpace(cfg.CheckoutURL) == "" {
		return nil, fmt.Errorf("PayPal is not configured")
	}
	if amountCents < 1 {
		return nil, fmt.Errorf("amount must be greater than 0")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	user := s.ensureUserByIdentityLocked(identity)
	if user == nil {
		return nil, fmt.Errorf("user identity required")
	}
	if !user.Enabled {
		return nil, fmt.Errorf("account is disabled")
	}
	order := s.newPendingOrderLocked(user, amountCents, "paypal", BillingProviderPayPal)
	values := url.Values{}
	values.Set("out_trade_no", order.OutTradeNo)
	values.Set("amount", centsToYuan(order.AmountCents))
	values.Set("user_id", user.ID)
	values.Set("email", user.Email)
	values.Set("name", user.Name)
	payURL := withQuery(cfg.CheckoutURL, values)
	s.orders[order.OutTradeNo] = order
	if err := s.saveLocked(); err != nil {
		delete(s.orders, order.OutTradeNo)
		return nil, err
	}
	return map[string]any{
		"id":           order.ID,
		"provider":     order.Provider,
		"status":       order.Status,
		"out_trade_no": order.OutTradeNo,
		"user_id":      order.UserID,
		"user_email":   order.UserEmail,
		"pay_type":     order.PayType,
		"amount_cents": order.AmountCents,
		"amount_yuan":  centsToYuan(order.AmountCents),
		"pay_url":      payURL,
		"created_at":   order.CreatedAt,
		"updated_at":   order.UpdatedAt,
	}, nil
}

func (s *EmailBillingService) CreateUSDTOrder(identity Identity, amountCents int, cfg USDTGatewayConfig) (map[string]any, error) {
	if !cfg.Enabled || strings.TrimSpace(cfg.Address) == "" {
		return nil, fmt.Errorf("USDT is not configured")
	}
	if amountCents < 1 {
		return nil, fmt.Errorf("amount must be greater than 0")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	user := s.ensureUserByIdentityLocked(identity)
	if user == nil {
		return nil, fmt.Errorf("user identity required")
	}
	if !user.Enabled {
		return nil, fmt.Errorf("account is disabled")
	}
	order := s.newPendingOrderLocked(user, amountCents, "usdt", BillingProviderUSDT)
	values := url.Values{}
	values.Set("out_trade_no", order.OutTradeNo)
	values.Set("amount", centsToYuan(order.AmountCents))
	values.Set("network", strings.TrimSpace(cfg.Network))
	values.Set("address", strings.TrimSpace(cfg.Address))
	values.Set("user_id", user.ID)
	values.Set("email", user.Email)
	values.Set("name", user.Name)
	payURL := ""
	if strings.TrimSpace(cfg.PaymentURL) != "" {
		payURL = withQuery(cfg.PaymentURL, values)
	}
	s.orders[order.OutTradeNo] = order
	if err := s.saveLocked(); err != nil {
		delete(s.orders, order.OutTradeNo)
		return nil, err
	}
	return map[string]any{
		"id":           order.ID,
		"provider":     order.Provider,
		"status":       order.Status,
		"out_trade_no": order.OutTradeNo,
		"user_id":      order.UserID,
		"user_email":   order.UserEmail,
		"pay_type":     order.PayType,
		"amount_cents": order.AmountCents,
		"amount_yuan":  centsToYuan(order.AmountCents),
		"pay_url":      payURL,
		"usdt_address": strings.TrimSpace(cfg.Address),
		"usdt_network": strings.TrimSpace(cfg.Network),
		"created_at":   order.CreatedAt,
		"updated_at":   order.UpdatedAt,
	}, nil
}

func (s *EmailBillingService) HandleYiPayNotify(values url.Values, cfg YiPayGatewayConfig) (bool, error) {
	if !cfg.Enabled || strings.TrimSpace(cfg.Key) == "" {
		return false, fmt.Errorf("YiPay is not configured")
	}
	flat := map[string]string{}
	for key, items := range values {
		if len(items) == 0 {
			continue
		}
		flat[key] = strings.TrimSpace(items[0])
	}
	sign := strings.TrimSpace(flat["sign"])
	if sign == "" {
		return false, fmt.Errorf("missing sign")
	}
	expected := yipaySign(flat, cfg.Key)
	if !strings.EqualFold(sign, expected) {
		return false, fmt.Errorf("invalid sign")
	}
	outTradeNo := strings.TrimSpace(flat["out_trade_no"])
	if outTradeNo == "" {
		return false, fmt.Errorf("missing out_trade_no")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	order := s.orders[outTradeNo]
	if order == nil {
		return false, fmt.Errorf("order not found")
	}
	tradeStatus := strings.ToUpper(strings.TrimSpace(flat["trade_status"]))
	if tradeStatus != "TRADE_SUCCESS" {
		order.Status = BillingOrderStatusFailed
		order.UpdatedAt = util.NowISO()
		_ = s.saveLocked()
		return true, nil
	}
	if order.Status == BillingOrderStatusPaid {
		return true, nil
	}
	moneyCents, err := yuanToCents(flat["money"])
	if err != nil {
		return false, fmt.Errorf("invalid money")
	}
	if moneyCents != order.AmountCents {
		return false, fmt.Errorf("money mismatch")
	}
	user := s.users[order.UserID]
	if user == nil {
		return false, fmt.Errorf("user not found")
	}
	user.BalanceCents += order.AmountCents
	user.TotalRechargeCents += order.AmountCents
	user.UpdatedAt = util.NowISO()

	now := util.NowISO()
	order.Status = BillingOrderStatusPaid
	order.TradeNo = strings.TrimSpace(flat["trade_no"])
	order.PaidAt = now
	order.UpdatedAt = now

	tx := map[string]any{
		"id":                  "tx_" + util.NewHex(18),
		"user_id":             user.ID,
		"email":               user.Email,
		"type":                BillingTxTypeRecharge,
		"amount_cents":        order.AmountCents,
		"balance_after_cents": user.BalanceCents,
		"order_id":            order.ID,
		"out_trade_no":        order.OutTradeNo,
		"trade_no":            order.TradeNo,
		"provider":            BillingProviderYiPay,
		"created_at":          now,
	}
	s.transactions = append(s.transactions, tx)

	if err := s.saveLocked(); err != nil {
		user.BalanceCents -= order.AmountCents
		user.TotalRechargeCents -= order.AmountCents
		user.UpdatedAt = util.NowISO()
		order.Status = BillingOrderStatusPending
		order.TradeNo = ""
		order.PaidAt = ""
		order.UpdatedAt = util.NowISO()
		if len(s.transactions) > 0 {
			s.transactions = s.transactions[:len(s.transactions)-1]
		}
		return false, err
	}
	return true, nil
}

func (s *EmailBillingService) ListOrdersByIdentity(identity Identity, limit int) []map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	user := s.ensureUserByIdentityLocked(identity)
	if user == nil {
		return []map[string]any{}
	}
	if limit < 1 {
		limit = 20
	}
	out := make([]map[string]any, 0)
	items := make([]*billingOrder, 0)
	for _, order := range s.orders {
		if order.UserID == user.ID {
			items = append(items, order)
		}
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].CreatedAt > items[j].CreatedAt
	})
	for _, item := range items {
		record := publicBillingOrder(item)
		record["source"] = "order"
		record["note"] = ""
		out = append(out, record)
	}
	for _, tx := range s.transactions {
		if util.Clean(tx["user_id"]) != user.ID {
			continue
		}
		txType := strings.TrimSpace(util.Clean(tx["type"]))
		if txType == BillingTxTypeConsume {
			continue
		}
		provider := strings.TrimSpace(util.Clean(tx["provider"]))
		if provider == BillingProviderYiPay || provider == BillingProviderPayPal || provider == BillingProviderUSDT {
			continue
		}
		createdAt := strings.TrimSpace(util.Clean(tx["created_at"]))
		status := strings.ToLower(strings.TrimSpace(util.Clean(tx["status"])))
		switch status {
		case "", "success", "paid":
			status = BillingOrderStatusPaid
		case "failed":
			status = BillingOrderStatusFailed
		case "pending":
			status = BillingOrderStatusPending
		default:
			status = BillingOrderStatusPaid
		}
		amountCents := util.ToInt(tx["amount_cents"], 0)
		id := strings.TrimSpace(util.Clean(tx["id"]))
		if id == "" {
			id = "tx_" + strings.ReplaceAll(strings.ReplaceAll(firstNonEmpty(createdAt, util.NowISO()), ":", ""), "-", "")
		}
		out = append(out, map[string]any{
			"id":           id,
			"provider":     firstNonEmpty(provider, BillingProviderAdmin),
			"pay_type":     firstNonEmpty(provider, BillingProviderAdmin),
			"amount_cents": amountCents,
			"amount_yuan":  centsToYuanSigned(amountCents),
			"status":       status,
			"out_trade_no": id,
			"trade_no":     "",
			"created_at":   createdAt,
			"updated_at":   createdAt,
			"paid_at":      "",
			"source":       "transaction",
			"tx_type":      txType,
			"note":         strings.TrimSpace(util.Clean(tx["note"])),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return util.Clean(out[i]["created_at"]) > util.Clean(out[j]["created_at"])
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func (s *EmailBillingService) inviteesForUserLocked(user *billingUser, limit int) ([]map[string]any, int) {
	if user == nil || strings.TrimSpace(user.InviteCode) == "" {
		return []map[string]any{}, 0
	}
	candidates := make([]*billingUser, 0)
	for _, item := range s.users {
		if strings.EqualFold(strings.TrimSpace(item.InvitedBy), strings.TrimSpace(user.InviteCode)) {
			candidates = append(candidates, item)
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].CreatedAt != candidates[j].CreatedAt {
			return candidates[i].CreatedAt > candidates[j].CreatedAt
		}
		return candidates[i].Email < candidates[j].Email
	})
	count := len(candidates)
	if limit < 1 {
		limit = 50
	}
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}
	out := make([]map[string]any, 0, len(candidates))
	for _, item := range candidates {
		out = append(out, map[string]any{
			"id":         item.ID,
			"email":      item.Email,
			"name":       item.Name,
			"created_at": item.CreatedAt,
		})
	}
	return out, count
}

func (s *EmailBillingService) appendBillingFailureLocked(user *billingUser, txType, provider string, amountCents int, note string) {
	if user == nil {
		return
	}
	s.transactions = append(s.transactions, map[string]any{
		"id":                  "tx_" + util.NewHex(18),
		"user_id":             user.ID,
		"email":               user.Email,
		"type":                firstNonEmpty(strings.TrimSpace(txType), BillingTxTypeRecharge),
		"amount_cents":        amountCents,
		"balance_after_cents": user.BalanceCents,
		"provider":            firstNonEmpty(strings.TrimSpace(provider), BillingProviderAdmin),
		"status":              BillingOrderStatusFailed,
		"note":                strings.TrimSpace(note),
		"created_at":          util.NowISO(),
	})
	_ = s.saveLocked()
}

func (s *EmailBillingService) loadLocked() {
	raw := loadStoredJSON(s.store, s.docName, s.path)
	obj, _ := raw.(map[string]any)
	users := util.AsMapSlice(obj["users"])
	registerCodes := util.AsMapSlice(obj["register_codes"])
	redeemCodes := util.AsMapSlice(obj["redeem_codes"])
	orders := util.AsMapSlice(obj["orders"])
	transactions := util.AsMapSlice(obj["transactions"])

	s.users = map[string]*billingUser{}
	s.userByEmail = map[string]string{}
	s.userByInviteCode = map[string]string{}
	s.registerCodes = map[string]*billingRegisterCode{}
	s.redeemCodes = map[string]*billingRedeemCode{}
	s.orders = map[string]*billingOrder{}
	s.transactions = make([]map[string]any, 0, len(transactions))

	for _, rawUser := range users {
		user := normalizeBillingUser(rawUser)
		if user == nil {
			continue
		}
		s.users[user.ID] = user
		s.userByEmail[user.Email] = user.ID
		if user.InviteCode != "" {
			if _, exists := s.userByInviteCode[user.InviteCode]; !exists {
				s.userByInviteCode[user.InviteCode] = user.ID
			}
		}
	}
	for _, rawOrder := range orders {
		order := normalizeBillingOrder(rawOrder)
		if order == nil {
			continue
		}
		s.orders[order.OutTradeNo] = order
	}
	for _, rawCode := range registerCodes {
		item := normalizeBillingRegisterCode(rawCode)
		if item == nil {
			continue
		}
		s.registerCodes[item.Email] = item
	}
	for _, rawCode := range redeemCodes {
		item := normalizeBillingRedeemCode(rawCode)
		if item == nil {
			continue
		}
		s.redeemCodes[item.Code] = item
	}
	for _, tx := range transactions {
		s.transactions = append(s.transactions, tx)
	}
}

func (s *EmailBillingService) saveLocked() error {
	users := make([]map[string]any, 0, len(s.users))
	for _, user := range s.users {
		users = append(users, billingUserToMap(user))
	}
	orders := make([]map[string]any, 0, len(s.orders))
	for _, order := range s.orders {
		orders = append(orders, billingOrderToMap(order))
	}
	registerCodes := make([]map[string]any, 0, len(s.registerCodes))
	for _, item := range s.registerCodes {
		registerCodes = append(registerCodes, billingRegisterCodeToMap(item))
	}
	redeemCodes := make([]map[string]any, 0, len(s.redeemCodes))
	for _, item := range s.redeemCodes {
		redeemCodes = append(redeemCodes, billingRedeemCodeToMap(item))
	}
	sort.Slice(users, func(i, j int) bool {
		return util.Clean(users[i]["created_at"]) > util.Clean(users[j]["created_at"])
	})
	sort.Slice(orders, func(i, j int) bool {
		return util.Clean(orders[i]["created_at"]) > util.Clean(orders[j]["created_at"])
	})
	sort.Slice(registerCodes, func(i, j int) bool {
		return util.Clean(registerCodes[i]["last_sent_at"]) > util.Clean(registerCodes[j]["last_sent_at"])
	})
	sort.Slice(redeemCodes, func(i, j int) bool {
		return util.Clean(redeemCodes[i]["created_at"]) > util.Clean(redeemCodes[j]["created_at"])
	})
	value := map[string]any{
		"users":          users,
		"register_codes": registerCodes,
		"redeem_codes":   redeemCodes,
		"orders":         orders,
		"transactions":   s.transactions,
	}
	return saveStoredJSON(s.store, s.docName, s.path, value)
}

func (s *EmailBillingService) userByEmailLocked(email string) *billingUser {
	id := s.userByEmail[email]
	if id == "" {
		return nil
	}
	return s.users[id]
}

func (s *EmailBillingService) userByIdentityLocked(identity Identity) *billingUser {
	if identity.Role != AuthRoleUser {
		return nil
	}
	userID := strings.TrimSpace(identity.OwnerID)
	if userID == "" {
		userID = strings.TrimSpace(identity.ID)
	}
	if userID == "" {
		return nil
	}
	return s.users[userID]
}

func (s *EmailBillingService) ensureUserByIdentityLocked(identity Identity) *billingUser {
	user := s.userByIdentityLocked(identity)
	if user != nil {
		return user
	}
	if identity.Role != AuthRoleUser {
		return nil
	}
	if identity.Provider != AuthProviderLocal {
		return nil
	}
	userID := strings.TrimSpace(identity.OwnerID)
	if userID == "" {
		userID = strings.TrimSpace(identity.ID)
	}
	if userID == "" {
		return nil
	}
	return s.ensureUserByIDLocked(userID, identity.Name, identity.Provider)
}

func (s *EmailBillingService) ensureUserByIDLocked(userID, name, provider string) *billingUser {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil
	}
	if user := s.users[userID]; user != nil {
		return user
	}
	normalizedProvider := strings.TrimSpace(provider)
	if normalizedProvider == "" {
		normalizedProvider = AuthProviderLocal
	}
	displayName := strings.TrimSpace(name)
	if displayName == "" {
		displayName = userID
	}
	safeID := strings.NewReplacer(":", "-", "/", "-", "\\", "-", "@", "-", " ", "-").Replace(strings.ToLower(userID))
	fallbackEmail := strings.TrimSpace(safeID) + "@local.invalid"
	now := util.NowISO()
	user := &billingUser{
		ID:                 userID,
		Email:              fallbackEmail,
		Provider:           normalizedProvider,
		Name:               displayName,
		InviteCode:         s.newInviteCodeLocked(userID),
		InvitedBy:          "",
		PasswordHash:       "local-user",
		AuthKeyID:          userID,
		Enabled:            true,
		BalanceCents:       0,
		TotalRechargeCents: 0,
		TotalConsumeCents:  0,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	s.users[userID] = user
	s.userByEmail[user.Email] = userID
	s.userByInviteCode[user.InviteCode] = user.ID
	if err := s.saveLocked(); err != nil {
		delete(s.users, userID)
		delete(s.userByEmail, user.Email)
		delete(s.userByInviteCode, user.InviteCode)
		return nil
	}
	return user
}

func (s *EmailBillingService) newOutTradeNoLocked() string {
	for {
		outTradeNo := "pay_" + time.Now().UTC().Format("20060102150405") + util.NewHex(8)
		if _, exists := s.orders[outTradeNo]; !exists {
			return outTradeNo
		}
	}
}

func (s *EmailBillingService) newPendingOrderLocked(user *billingUser, amountCents int, payType, provider string) *billingOrder {
	now := util.NowISO()
	return &billingOrder{
		ID:          "ord_" + util.NewHex(18),
		OutTradeNo:  s.newOutTradeNoLocked(),
		Provider:    strings.TrimSpace(provider),
		UserID:      user.ID,
		UserEmail:   user.Email,
		PayType:     strings.ToLower(strings.TrimSpace(payType)),
		AmountCents: amountCents,
		Status:      BillingOrderStatusPending,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func (s *EmailBillingService) newRedeemCodeLocked() string {
	for {
		code := "CG2A-" + strings.ToUpper(util.NewHex(4)) + "-" + strings.ToUpper(util.NewHex(4))
		if _, exists := s.redeemCodes[code]; !exists {
			return code
		}
	}
}

func (s *EmailBillingService) newInviteCodeLocked(userID string) string {
	base := inviteCodeFromUserID(userID)
	if ownerID, exists := s.userByInviteCode[base]; !exists || ownerID == userID {
		return base
	}
	for {
		next := base + "-" + strings.ToUpper(util.NewHex(2))
		if ownerID, exists := s.userByInviteCode[next]; !exists || ownerID == userID {
			return next
		}
	}
}

func normalizeBillingUser(raw map[string]any) *billingUser {
	id := strings.TrimSpace(util.Clean(raw["id"]))
	email := strings.ToLower(strings.TrimSpace(util.Clean(raw["email"])))
	passwordHash := strings.TrimSpace(util.Clean(raw["password_hash"]))
	authKeyID := strings.TrimSpace(util.Clean(raw["auth_key_id"]))
	if id == "" || email == "" || passwordHash == "" || authKeyID == "" {
		return nil
	}
	name := strings.TrimSpace(util.Clean(raw["name"]))
	if name == "" {
		name = email
	}
	return &billingUser{
		ID:                 id,
		Email:              email,
		Provider:           firstNonEmpty(strings.TrimSpace(util.Clean(raw["provider"])), AuthProviderEmail),
		Name:               name,
		InviteCode:         firstNonEmpty(normalizeInviteCode(raw["invite_code"]), inviteCodeFromUserID(id)),
		InvitedBy:          normalizeInviteCode(raw["invited_by"]),
		PasswordHash:       passwordHash,
		AuthKeyID:          authKeyID,
		Enabled:            util.ToBool(util.ValueOr(raw["enabled"], true)),
		BalanceCents:       maxBillingInt(0, util.ToInt(raw["balance_cents"], 0)),
		TotalRechargeCents: maxBillingInt(0, util.ToInt(raw["total_recharge_cents"], 0)),
		TotalConsumeCents:  maxBillingInt(0, util.ToInt(raw["total_consume_cents"], 0)),
		CreatedAt:          firstNonEmpty(util.Clean(raw["created_at"]), util.NowISO()),
		UpdatedAt:          firstNonEmpty(util.Clean(raw["updated_at"]), util.Clean(raw["created_at"]), util.NowISO()),
		LastLoginAt:        util.Clean(raw["last_login_at"]),
	}
}

func billingUserToMap(user *billingUser) map[string]any {
	return map[string]any{
		"id":                   user.ID,
		"email":                user.Email,
		"provider":             user.Provider,
		"name":                 user.Name,
		"invite_code":          user.InviteCode,
		"invited_by":           user.InvitedBy,
		"password_hash":        user.PasswordHash,
		"auth_key_id":          user.AuthKeyID,
		"enabled":              user.Enabled,
		"balance_cents":        user.BalanceCents,
		"total_recharge_cents": user.TotalRechargeCents,
		"total_consume_cents":  user.TotalConsumeCents,
		"created_at":           user.CreatedAt,
		"updated_at":           user.UpdatedAt,
		"last_login_at":        user.LastLoginAt,
	}
}

func publicBillingUser(user *billingUser) map[string]any {
	provider := strings.TrimSpace(user.Provider)
	if provider == "" {
		provider = AuthProviderEmail
	}
	return map[string]any{
		"id":                   user.ID,
		"email":                user.Email,
		"name":                 user.Name,
		"invite_code":          user.InviteCode,
		"invited_by":           user.InvitedBy,
		"enabled":              user.Enabled,
		"provider":             provider,
		"role":                 AuthRoleUser,
		"balance_cents":        user.BalanceCents,
		"total_recharge_cents": user.TotalRechargeCents,
		"total_consume_cents":  user.TotalConsumeCents,
		"created_at":           user.CreatedAt,
		"updated_at":           user.UpdatedAt,
		"last_login_at":        user.LastLoginAt,
	}
}

func normalizeBillingRegisterCode(raw map[string]any) *billingRegisterCode {
	email := strings.ToLower(strings.TrimSpace(util.Clean(raw["email"])))
	codeHash := strings.TrimSpace(util.Clean(raw["code_hash"]))
	expiresAt := strings.TrimSpace(util.Clean(raw["expires_at"]))
	if email == "" || codeHash == "" || expiresAt == "" {
		return nil
	}
	return &billingRegisterCode{
		Email:      email,
		CodeHash:   codeHash,
		ExpiresAt:  expiresAt,
		LastSentAt: strings.TrimSpace(util.Clean(raw["last_sent_at"])),
		SendCount:  maxBillingInt(0, util.ToInt(raw["send_count"], 0)),
	}
}

func billingRegisterCodeToMap(item *billingRegisterCode) map[string]any {
	return map[string]any{
		"email":        item.Email,
		"code_hash":    item.CodeHash,
		"expires_at":   item.ExpiresAt,
		"last_sent_at": item.LastSentAt,
		"send_count":   item.SendCount,
	}
}

func normalizeBillingRedeemCode(raw map[string]any) *billingRedeemCode {
	code := strings.ToUpper(strings.TrimSpace(util.Clean(raw["code"])))
	if code == "" {
		return nil
	}
	amountCents := maxBillingInt(0, util.ToInt(raw["amount_cents"], 0))
	if amountCents < 1 {
		return nil
	}
	return &billingRedeemCode{
		Code:        code,
		AmountCents: amountCents,
		Enabled:     util.ToBool(util.ValueOr(raw["enabled"], true)),
		CreatedAt:   firstNonEmpty(strings.TrimSpace(util.Clean(raw["created_at"])), util.NowISO()),
		UpdatedAt:   firstNonEmpty(strings.TrimSpace(util.Clean(raw["updated_at"])), util.Clean(raw["created_at"]), util.NowISO()),
		ExpiresAt:   strings.TrimSpace(util.Clean(raw["expires_at"])),
		UsedBy:      strings.TrimSpace(util.Clean(raw["used_by"])),
		UsedAt:      strings.TrimSpace(util.Clean(raw["used_at"])),
		Note:        strings.TrimSpace(util.Clean(raw["note"])),
	}
}

func billingRedeemCodeToMap(item *billingRedeemCode) map[string]any {
	return map[string]any{
		"code":         item.Code,
		"amount_cents": item.AmountCents,
		"enabled":      item.Enabled,
		"created_at":   item.CreatedAt,
		"updated_at":   item.UpdatedAt,
		"expires_at":   item.ExpiresAt,
		"used_by":      item.UsedBy,
		"used_at":      item.UsedAt,
		"note":         item.Note,
	}
}

func publicBillingRedeemCode(item *billingRedeemCode) map[string]any {
	return map[string]any{
		"code":         item.Code,
		"amount_cents": item.AmountCents,
		"amount_yuan":  centsToYuan(item.AmountCents),
		"enabled":      item.Enabled,
		"created_at":   item.CreatedAt,
		"updated_at":   item.UpdatedAt,
		"expires_at":   item.ExpiresAt,
		"used_by":      item.UsedBy,
		"used_at":      item.UsedAt,
		"note":         item.Note,
	}
}

func normalizeBillingOrder(raw map[string]any) *billingOrder {
	outTradeNo := strings.TrimSpace(util.Clean(raw["out_trade_no"]))
	userID := strings.TrimSpace(util.Clean(raw["user_id"]))
	if outTradeNo == "" || userID == "" {
		return nil
	}
	status := strings.TrimSpace(util.Clean(raw["status"]))
	switch status {
	case BillingOrderStatusPending, BillingOrderStatusPaid, BillingOrderStatusFailed:
	default:
		status = BillingOrderStatusPending
	}
	return &billingOrder{
		ID:          firstNonEmpty(strings.TrimSpace(util.Clean(raw["id"])), "ord_"+util.NewHex(18)),
		OutTradeNo:  outTradeNo,
		TradeNo:     strings.TrimSpace(util.Clean(raw["trade_no"])),
		Provider:    firstNonEmpty(strings.TrimSpace(util.Clean(raw["provider"])), BillingProviderYiPay),
		UserID:      userID,
		UserEmail:   strings.TrimSpace(util.Clean(raw["user_email"])),
		PayType:     strings.TrimSpace(util.Clean(raw["pay_type"])),
		AmountCents: maxBillingInt(0, util.ToInt(raw["amount_cents"], 0)),
		Status:      status,
		CreatedAt:   firstNonEmpty(strings.TrimSpace(util.Clean(raw["created_at"])), util.NowISO()),
		UpdatedAt:   firstNonEmpty(strings.TrimSpace(util.Clean(raw["updated_at"])), util.Clean(raw["created_at"]), util.NowISO()),
		PaidAt:      strings.TrimSpace(util.Clean(raw["paid_at"])),
	}
}

func billingOrderToMap(order *billingOrder) map[string]any {
	return map[string]any{
		"id":           order.ID,
		"out_trade_no": order.OutTradeNo,
		"trade_no":     order.TradeNo,
		"provider":     order.Provider,
		"user_id":      order.UserID,
		"user_email":   order.UserEmail,
		"pay_type":     order.PayType,
		"amount_cents": order.AmountCents,
		"status":       order.Status,
		"created_at":   order.CreatedAt,
		"updated_at":   order.UpdatedAt,
		"paid_at":      order.PaidAt,
	}
}

func publicBillingOrder(order *billingOrder) map[string]any {
	return map[string]any{
		"id":           order.ID,
		"out_trade_no": order.OutTradeNo,
		"trade_no":     order.TradeNo,
		"provider":     order.Provider,
		"pay_type":     order.PayType,
		"amount_cents": order.AmountCents,
		"amount_yuan":  centsToYuan(order.AmountCents),
		"status":       order.Status,
		"created_at":   order.CreatedAt,
		"updated_at":   order.UpdatedAt,
		"paid_at":      order.PaidAt,
	}
}

func normalizeEmail(email string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(email))
	if normalized == "" {
		return "", fmt.Errorf("email is required")
	}
	if !emailRE.MatchString(normalized) {
		return "", fmt.Errorf("invalid email format")
	}
	return normalized, nil
}

func validateEmailDomain(email string, allowedDomains []string) error {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return fmt.Errorf("invalid email format")
	}
	domain := strings.ToLower(strings.TrimSpace(parts[1]))
	allowed := map[string]struct{}{}
	for _, item := range allowedDomains {
		value := strings.ToLower(strings.TrimSpace(item))
		if value != "" {
			allowed[value] = struct{}{}
		}
	}
	if len(allowed) == 0 {
		return nil
	}
	if _, ok := allowed[domain]; !ok {
		return fmt.Errorf("email domain is not allowed")
	}
	return nil
}

func validatePassword(password string) error {
	password = strings.TrimSpace(password)
	if len(password) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}
	if len(password) > 128 {
		return fmt.Errorf("password is too long")
	}
	return nil
}

func yipaySign(params map[string]string, key string) string {
	keys := make([]string, 0, len(params))
	for k, v := range params {
		if k == "sign" || k == "sign_type" {
			continue
		}
		if strings.TrimSpace(v) == "" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, item := range keys {
		parts = append(parts, item+"="+strings.TrimSpace(params[item]))
	}
	signText := strings.Join(parts, "&") + strings.TrimSpace(key)
	sum := md5.Sum([]byte(signText))
	return hex.EncodeToString(sum[:])
}

func centsToYuan(value int) string {
	if value < 0 {
		value = 0
	}
	return fmt.Sprintf("%.2f", float64(value)/100)
}

func centsToYuanSigned(value int) string {
	sign := ""
	if value < 0 {
		sign = "-"
		value = -value
	}
	return sign + fmt.Sprintf("%.2f", float64(value)/100)
}

func yuanToCents(text string) (int, error) {
	value := strings.TrimSpace(text)
	if value == "" {
		return 0, fmt.Errorf("empty amount")
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, err
	}
	if parsed < 0 {
		return 0, fmt.Errorf("amount cannot be negative")
	}
	cents := int(parsed*100 + 0.5)
	return cents, nil
}

func (s *EmailBillingService) verifyRegisterCodeLocked(email, code string) error {
	item := s.registerCodes[email]
	if item == nil {
		return fmt.Errorf("verification code is required")
	}
	expiresAt, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(item.ExpiresAt))
	if err != nil || expiresAt.IsZero() {
		delete(s.registerCodes, email)
		return fmt.Errorf("verification code is invalid")
	}
	if time.Now().UTC().After(expiresAt) {
		delete(s.registerCodes, email)
		return fmt.Errorf("verification code has expired")
	}
	if hashRegisterCode(code) != strings.TrimSpace(item.CodeHash) {
		return fmt.Errorf("verification code is invalid")
	}
	return nil
}

func hashRegisterCode(code string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(code)))
	return hex.EncodeToString(sum[:])
}

func generateRegisterCode6() (string, error) {
	max := 1000000
	var buf [4]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	num := int(buf[0])<<24 | int(buf[1])<<16 | int(buf[2])<<8 | int(buf[3])
	if num < 0 {
		num = -num
	}
	num = num % max
	return fmt.Sprintf("%06d", num), nil
}

func normalizeInviteCode(value any) string {
	return strings.ToUpper(strings.TrimSpace(util.Clean(value)))
}

func inviteCodeFromUserID(userID string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(userID)))
	return "INV-" + strings.ToUpper(hex.EncodeToString(sum[:6]))
}

func withQuery(base string, values url.Values) string {
	base = strings.TrimSpace(base)
	if base == "" {
		return ""
	}
	encoded := values.Encode()
	if encoded == "" {
		return base
	}
	if strings.Contains(base, "?") {
		return strings.TrimRight(base, "&") + "&" + encoded
	}
	return strings.TrimRight(base, "?") + "?" + encoded
}

func minBillingInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxBillingInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
