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

	BillingWithdrawStatusPending  = "pending"
	BillingWithdrawStatusApproved = "approved"
	BillingWithdrawStatusRejected = "rejected"
	BillingWithdrawStatusPaid     = "paid"

	BillingOrderKindRecharge      = "recharge"
	BillingOrderKindAgencyJoin    = "agency_join"
	BillingOrderKindAgencyUpgrade = "agency_upgrade"

	BillingTxTypeRecharge = "recharge"
	BillingTxTypeConsume  = "consume"
	BillingTxTypeAdjust   = "adjust"

	RegisterCodeExpireMinutes = 10
	RegisterCodeResendSeconds = 60

	RegisterBonusImageTimes = 20
	InviteBonusImageTimes   = 10

	AgencyTierBasic   = "basic"
	AgencyTierPro     = "pro"
	AgencyTierPremium = "premium"
)

var emailRE = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

var billingBeijingLocation = func() *time.Location {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.FixedZone("CST", 8*3600)
	}
	return loc
}()

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
	withdrawals      []*billingWithdrawalRequest
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
	AgencyTier         string
	AgencyEnabled      bool
	AgencyCommissionBP int
	AgencyDiscountBP   int
	AgencyJoinedAt     string
	AgencyAlipayQRCode string
	AgencyWeChatQRCode string
	AgencyPhone        string
	AgencyWeChatID     string
	RegisterIP         string
	RegisterDeviceID   string
	CreatedAt          string
	UpdatedAt          string
	LastLoginAt        string
}

type billingOrder struct {
	ID          string
	OutTradeNo  string
	TradeNo     string
	Provider    string
	Kind        string
	UserID      string
	UserEmail   string
	PayType     string
	AmountCents int
	BonusCents  int
	AgencyTier  string
	Note        string
	Status      string
	CreatedAt   string
	UpdatedAt   string
	PaidAt      string
}

type billingWithdrawalRequest struct {
	ID           string
	UserID       string
	UserEmail    string
	AmountCents  int
	AlipayQRCode string
	WeChatQRCode string
	Phone        string
	WeChatID     string
	Status       string
	AdminNote    string
	CreatedAt    string
	UpdatedAt    string
	ProcessedAt  string
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
		withdrawals:      make([]*billingWithdrawalRequest, 0),
	}
	s.mu.Lock()
	s.loadLocked()
	s.mu.Unlock()
	return s
}

func (s *EmailBillingService) RegisterEmailUser(email, password, name, verifyCode, inviteCode string, imagePriceCents int, registerBonusTimes int, allowedDomains []string, smtpConfig EmailSMTPConfig) (map[string]any, string, error) {
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
	if registerBonusTimes < 0 {
		registerBonusTimes = 0
	}
	registerBonusCents := maxBillingInt(0, imagePriceCents) * registerBonusTimes
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
			"note":                fmt.Sprintf("new user bonus (%d image credits)", registerBonusTimes),
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
	invitedUsers := s.invitedUsersByCodeLocked(user.InviteCode)
	invitedByEmail := ""
	if invitedByCode := normalizeInviteCode(user.InvitedBy); invitedByCode != "" {
		if inviterID := strings.TrimSpace(s.userByInviteCode[invitedByCode]); inviterID != "" {
			if inviter := s.users[inviterID]; inviter != nil {
				invitedByEmail = strings.TrimSpace(inviter.Email)
			}
		}
	}
	return map[string]any{
		"user_id":               user.ID,
		"email":                 user.Email,
		"name":                  user.Name,
		"invite_code":           user.InviteCode,
		"invited_by":            user.InvitedBy,
		"invited_by_email":      invitedByEmail,
		"invited_count":         len(invitedUsers),
		"invited_users":         invitedUsers,
		"balance_cents":         user.BalanceCents,
		"total_recharge_cents":  user.TotalRechargeCents,
		"total_consume_cents":   user.TotalConsumeCents,
		"agency_tier":           user.AgencyTier,
		"agency_enabled":        user.AgencyEnabled,
		"agency_commission_bp":  user.AgencyCommissionBP,
		"agency_discount_bp":    user.AgencyDiscountBP,
		"agency_joined_at":      user.AgencyJoinedAt,
		"image_price_note":      "amount is in cents",
		"last_login_at":         user.LastLoginAt,
		"updated_at":            user.UpdatedAt,
		"billing_provider_hint": BillingProviderYiPay,
	}
}

func (s *EmailBillingService) ApplyRegisterBonusForUser(userID string, imagePriceCents int, registerBonusTimes int) error {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return fmt.Errorf("user id is required")
	}
	if registerBonusTimes <= 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	user := s.users[userID]
	if user == nil {
		return fmt.Errorf("user not found")
	}
	for _, tx := range s.transactions {
		if strings.TrimSpace(util.Clean(tx["user_id"])) != userID {
			continue
		}
		if strings.TrimSpace(util.Clean(tx["provider"])) == BillingProviderRegisterBonus {
			return nil
		}
	}

	bonusCents := maxBillingInt(0, imagePriceCents) * registerBonusTimes
	if bonusCents <= 0 {
		return nil
	}
	now := util.NowISO()
	user.BalanceCents += bonusCents
	user.TotalRechargeCents += bonusCents
	user.UpdatedAt = now
	s.transactions = append(s.transactions, map[string]any{
		"id":                  "tx_" + util.NewHex(18),
		"user_id":             user.ID,
		"email":               user.Email,
		"type":                BillingTxTypeRecharge,
		"amount_cents":        bonusCents,
		"balance_after_cents": user.BalanceCents,
		"provider":            BillingProviderRegisterBonus,
		"note":                fmt.Sprintf("new user bonus (%d image credits)", registerBonusTimes),
		"created_at":          now,
	})
	if err := s.saveLocked(); err != nil {
		user.BalanceCents -= bonusCents
		user.TotalRechargeCents -= bonusCents
		user.UpdatedAt = util.NowISO()
		if len(s.transactions) > 0 {
			s.transactions = s.transactions[:len(s.transactions)-1]
		}
		return err
	}
	return nil
}

type AgencyTierBenefit struct {
	Tier         string
	CommissionBP int
	DiscountBP   int
}

func (s *EmailBillingService) ActivateAgencyByIdentity(identity Identity, benefit AgencyTierBenefit, allowUpgradeOnly bool) (map[string]any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	user := s.ensureUserByIdentityLocked(identity)
	if user == nil {
		return nil, fmt.Errorf("user not found")
	}
	return s.activateAgencyLocked(user, benefit, allowUpgradeOnly)
}

func (s *EmailBillingService) ActivateAgencyByUserID(userID string, benefit AgencyTierBenefit, allowUpgradeOnly bool) (map[string]any, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, fmt.Errorf("user id is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	user := s.users[userID]
	if user == nil {
		return nil, fmt.Errorf("user not found")
	}
	return s.activateAgencyLocked(user, benefit, allowUpgradeOnly)
}

func (s *EmailBillingService) DeactivateAgencyByUserID(userID string) (map[string]any, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, fmt.Errorf("user id is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	user := s.users[userID]
	if user == nil {
		return nil, fmt.Errorf("user not found")
	}
	if !user.AgencyEnabled && user.AgencyTier == "" && user.AgencyCommissionBP == 0 && user.AgencyDiscountBP == 0 {
		return publicBillingUser(user), nil
	}
	user.AgencyEnabled = false
	user.AgencyTier = ""
	user.AgencyCommissionBP = 0
	user.AgencyDiscountBP = 0
	user.AgencyJoinedAt = ""
	user.UpdatedAt = util.NowISO()
	if err := s.saveLocked(); err != nil {
		return nil, err
	}
	return publicBillingUser(user), nil
}

func (s *EmailBillingService) activateAgencyLocked(user *billingUser, benefit AgencyTierBenefit, allowUpgradeOnly bool) (map[string]any, error) {
	nextTier := normalizeAgencyTier(benefit.Tier)
	if nextTier == "" {
		return nil, fmt.Errorf("invalid agency tier")
	}
	if allowUpgradeOnly && agencyTierRank(nextTier) < agencyTierRank(user.AgencyTier) {
		return nil, fmt.Errorf("cannot downgrade agency tier")
	}
	if user.AgencyTier == nextTier && user.AgencyEnabled &&
		user.AgencyCommissionBP == normalizeAgencyBasisPoint(benefit.CommissionBP) &&
		user.AgencyDiscountBP == normalizeAgencyBasisPoint(benefit.DiscountBP) {
		return publicBillingUser(user), nil
	}
	now := util.NowISO()
	user.AgencyTier = nextTier
	user.AgencyEnabled = true
	user.AgencyCommissionBP = normalizeAgencyBasisPoint(benefit.CommissionBP)
	user.AgencyDiscountBP = normalizeAgencyBasisPoint(benefit.DiscountBP)
	if strings.TrimSpace(user.AgencyJoinedAt) == "" {
		user.AgencyJoinedAt = now
	}
	user.UpdatedAt = now
	if err := s.saveLocked(); err != nil {
		return nil, err
	}
	return publicBillingUser(user), nil
}

func (s *EmailBillingService) AgencyDashboardByIdentity(identity Identity, registerURLBase string) (map[string]any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	user := s.ensureUserByIdentityLocked(identity)
	if user == nil {
		return nil, fmt.Errorf("user not found")
	}
	return s.agencyDashboardLocked(user, registerURLBase), nil
}

func (s *EmailBillingService) agencyDashboardLocked(user *billingUser, registerURLBase string) map[string]any {
	inviteCode := strings.TrimSpace(user.InviteCode)
	invitedUsers := s.invitedUsersByCodeLocked(inviteCode)
	todayCommission, monthCommission, totalCommission, orderRows := s.agencyCommissionStatsLocked(user, invitedUsers)
	withdrawReservedCents := s.agencyWithdrawReservedCentsLocked(user.ID)
	availableCents := totalCommission - withdrawReservedCents
	if availableCents < 0 {
		availableCents = 0
	}
	withdrawRequests := s.listAgencyWithdrawalsByUserIDLocked(user.ID, 200)

	return map[string]any{
		"agent": map[string]any{
			"user_id":        user.ID,
			"email":          user.Email,
			"name":           user.Name,
			"tier":           user.AgencyTier,
			"enabled":        user.AgencyEnabled,
			"commission_bp":  user.AgencyCommissionBP,
			"discount_bp":    user.AgencyDiscountBP,
			"joined_at":      user.AgencyJoinedAt,
			"invite_code":    user.InviteCode,
			"channel_link":   buildAgencyInviteLink(registerURLBase, user.InviteCode),
			"invited_count":  len(invitedUsers),
			"invited_users":  invitedUsers,
			"wallet_balance": user.BalanceCents,
		},
		"summary": map[string]any{
			"today_commission_cents": todayCommission,
			"today_commission_yuan":  centsToYuan(todayCommission),
			"month_commission_cents": monthCommission,
			"month_commission_yuan":  centsToYuan(monthCommission),
			"total_commission_cents": totalCommission,
			"total_commission_yuan":  centsToYuan(totalCommission),
			"available_cents":        availableCents,
			"available_yuan":         centsToYuan(availableCents),
		},
		"orders":      orderRows,
		"withdrawals": withdrawRequests,
	}
}

func (s *EmailBillingService) AgencyWithdrawProfileByIdentity(identity Identity) (map[string]any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	user := s.ensureUserByIdentityLocked(identity)
	if user == nil {
		return nil, fmt.Errorf("user not found")
	}
	return publicBillingAgencyWithdrawProfile(user), nil
}

func (s *EmailBillingService) UpdateAgencyWithdrawProfile(identity Identity, alipayQRCode, weChatQRCode, phone, weChatID string) (map[string]any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	user := s.ensureUserByIdentityLocked(identity)
	if user == nil {
		return nil, fmt.Errorf("user not found")
	}
	user.AgencyAlipayQRCode = strings.TrimSpace(alipayQRCode)
	user.AgencyWeChatQRCode = strings.TrimSpace(weChatQRCode)
	user.AgencyPhone = strings.TrimSpace(phone)
	user.AgencyWeChatID = strings.TrimSpace(weChatID)
	user.UpdatedAt = util.NowISO()
	if err := s.saveLocked(); err != nil {
		return nil, err
	}
	return publicBillingAgencyWithdrawProfile(user), nil
}

func (s *EmailBillingService) CreateAgencyWithdrawalRequest(identity Identity, amountCents int, alipayQRCode, weChatQRCode, phone, weChatID string) (map[string]any, error) {
	if amountCents <= 0 {
		return nil, fmt.Errorf("withdraw amount must be greater than 0")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	user := s.ensureUserByIdentityLocked(identity)
	if user == nil {
		return nil, fmt.Errorf("user not found")
	}
	if !user.AgencyEnabled || normalizeAgencyTier(user.AgencyTier) == "" {
		return nil, fmt.Errorf("agency permission required")
	}

	_, _, totalCommission, _ := s.agencyCommissionStatsLocked(user, s.invitedUsersByCodeLocked(user.InviteCode))
	reserved := s.agencyWithdrawReservedCentsLocked(user.ID)
	available := totalCommission - reserved
	if available < 0 {
		available = 0
	}
	if amountCents > available {
		return nil, fmt.Errorf("withdraw amount exceeds available balance")
	}
	alipayQRCode = strings.TrimSpace(firstNonEmpty(alipayQRCode, user.AgencyAlipayQRCode))
	weChatQRCode = strings.TrimSpace(firstNonEmpty(weChatQRCode, user.AgencyWeChatQRCode))
	phone = strings.TrimSpace(firstNonEmpty(phone, user.AgencyPhone))
	weChatID = strings.TrimSpace(firstNonEmpty(weChatID, user.AgencyWeChatID))

	item := &billingWithdrawalRequest{
		ID:           "wd_" + util.NewHex(18),
		UserID:       user.ID,
		UserEmail:    user.Email,
		AmountCents:  amountCents,
		AlipayQRCode: strings.TrimSpace(alipayQRCode),
		WeChatQRCode: strings.TrimSpace(weChatQRCode),
		Phone:        strings.TrimSpace(phone),
		WeChatID:     strings.TrimSpace(weChatID),
		Status:       BillingWithdrawStatusPending,
		AdminNote:    "",
		CreatedAt:    util.NowISO(),
		UpdatedAt:    util.NowISO(),
		ProcessedAt:  "",
	}
	if item.AlipayQRCode == "" && item.WeChatQRCode == "" && item.Phone == "" && item.WeChatID == "" {
		return nil, fmt.Errorf("at least one payout contact is required")
	}
	user.AgencyAlipayQRCode = item.AlipayQRCode
	user.AgencyWeChatQRCode = item.WeChatQRCode
	user.AgencyPhone = item.Phone
	user.AgencyWeChatID = item.WeChatID
	user.UpdatedAt = item.UpdatedAt
	s.withdrawals = append(s.withdrawals, item)
	if err := s.saveLocked(); err != nil {
		if len(s.withdrawals) > 0 {
			s.withdrawals = s.withdrawals[:len(s.withdrawals)-1]
		}
		return nil, err
	}
	return publicBillingWithdrawalRequest(item), nil
}

func (s *EmailBillingService) ListAgencyWithdrawalRequestsByIdentity(identity Identity, limit int) ([]map[string]any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	user := s.ensureUserByIdentityLocked(identity)
	if user == nil {
		return nil, fmt.Errorf("user not found")
	}
	return s.listAgencyWithdrawalsByUserIDLocked(user.ID, limit), nil
}

func (s *EmailBillingService) ListAgencyWithdrawalRequestsForAdmin(limit int) []map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]map[string]any, 0, len(s.withdrawals))
	for _, item := range s.withdrawals {
		if item == nil {
			continue
		}
		out = append(out, publicBillingWithdrawalRequest(item))
	}
	sort.Slice(out, func(i, j int) bool {
		left := parseBillingTimestamp(strings.TrimSpace(util.Clean(out[i]["created_at"])))
		right := parseBillingTimestamp(strings.TrimSpace(util.Clean(out[j]["created_at"])))
		if !left.Equal(right) {
			return left.After(right)
		}
		return strings.TrimSpace(util.Clean(out[i]["id"])) > strings.TrimSpace(util.Clean(out[j]["id"]))
	})
	if limit > 0 && len(out) > limit {
		return out[:limit]
	}
	return out
}

func (s *EmailBillingService) listAgencyWithdrawalsByUserIDLocked(userID string, limit int) []map[string]any {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return []map[string]any{}
	}
	out := make([]map[string]any, 0)
	for _, item := range s.withdrawals {
		if item == nil || strings.TrimSpace(item.UserID) != userID {
			continue
		}
		out = append(out, publicBillingWithdrawalRequest(item))
	}
	sort.Slice(out, func(i, j int) bool {
		left := parseBillingTimestamp(strings.TrimSpace(util.Clean(out[i]["created_at"])))
		right := parseBillingTimestamp(strings.TrimSpace(util.Clean(out[j]["created_at"])))
		if !left.Equal(right) {
			return left.After(right)
		}
		return strings.TrimSpace(util.Clean(out[i]["id"])) > strings.TrimSpace(util.Clean(out[j]["id"]))
	})
	if limit > 0 && len(out) > limit {
		return out[:limit]
	}
	return out
}

func (s *EmailBillingService) agencyWithdrawReservedCentsLocked(userID string) int {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return 0
	}
	total := 0
	for _, item := range s.withdrawals {
		if item == nil || strings.TrimSpace(item.UserID) != userID {
			continue
		}
		switch normalizeAgencyWithdrawStatus(item.Status) {
		case BillingWithdrawStatusRejected:
			continue
		default:
			total += maxBillingInt(0, item.AmountCents)
		}
	}
	return total
}

func (s *EmailBillingService) agencyCommissionStatsLocked(user *billingUser, invitedUsers []map[string]any) (int, int, int, []map[string]any) {
	invitedUserSet := map[string]struct{}{}
	for _, item := range invitedUsers {
		id := strings.TrimSpace(util.Clean(item["id"]))
		if id != "" {
			invitedUserSet[id] = struct{}{}
		}
	}

	orderRows := make([]map[string]any, 0)
	totalCommission := 0
	monthCommission := 0
	todayCommission := 0
	now := time.Now().In(billingBeijingLocation)
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, billingBeijingLocation)
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, billingBeijingLocation)

	for _, order := range s.orders {
		if order.Status != BillingOrderStatusPaid {
			continue
		}
		if _, ok := invitedUserSet[order.UserID]; !ok {
			continue
		}
		commissionCents := order.AmountCents * user.AgencyCommissionBP / 10000
		createdAt := firstNonEmpty(order.PaidAt, order.UpdatedAt, order.CreatedAt)
		createdTime := parseBillingTimestamp(createdAt).In(billingBeijingLocation)
		totalCommission += commissionCents
		if !createdTime.Before(monthStart) {
			monthCommission += commissionCents
		}
		if !createdTime.Before(dayStart) {
			todayCommission += commissionCents
		}
		orderRows = append(orderRows, map[string]any{
			"id":                order.ID,
			"user_id":           order.UserID,
			"user_email":        order.UserEmail,
			"amount_cents":      order.AmountCents,
			"amount_yuan":       centsToYuan(order.AmountCents),
			"commission_cents":  commissionCents,
			"commission_yuan":   centsToYuan(commissionCents),
			"status":            order.Status,
			"created_at":        createdAt,
			"out_trade_no":      order.OutTradeNo,
			"agent_tier":        user.AgencyTier,
			"commission_bp":     user.AgencyCommissionBP,
			"discount_bp":       user.AgencyDiscountBP,
			"recharge_discount": fmt.Sprintf("%.2f%%", float64(user.AgencyDiscountBP)/100),
		})
	}

	sort.Slice(orderRows, func(i, j int) bool {
		left := parseBillingTimestamp(strings.TrimSpace(util.Clean(orderRows[i]["created_at"])))
		right := parseBillingTimestamp(strings.TrimSpace(util.Clean(orderRows[j]["created_at"])))
		return left.After(right)
	})
	return todayCommission, monthCommission, totalCommission, orderRows
}

func buildAgencyInviteLink(base, inviteCode string) string {
	base = strings.TrimSpace(base)
	inviteCode = strings.TrimSpace(inviteCode)
	if base == "" || inviteCode == "" {
		return ""
	}
	u, err := url.Parse(base)
	if err != nil {
		return ""
	}
	query := u.Query()
	query.Set("invite_code", inviteCode)
	u.RawQuery = query.Encode()
	return u.String()
}

func billingRechargeNote(order *billingOrder) string {
	if order == nil {
		return ""
	}
	if order.BonusCents <= 0 {
		return ""
	}
	return fmt.Sprintf("agency recharge bonus +%s", centsToYuan(order.BonusCents))
}

func agencyTierRank(tier string) int {
	switch normalizeAgencyTier(tier) {
	case AgencyTierBasic:
		return 1
	case AgencyTierPro:
		return 2
	case AgencyTierPremium:
		return 3
	default:
		return 0
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

func (s *EmailBillingService) EnsureWalletUserWithEmail(userID, email, name, provider string) map[string]any {
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

	normalizedEmail, err := normalizeEmail(email)
	if err == nil {
		if ownerID := strings.TrimSpace(s.userByEmail[normalizedEmail]); ownerID != "" && ownerID != user.ID {
			normalizedEmail = ""
		}
	}

	changed := false
	if normalizedEmail != "" && user.Email != normalizedEmail {
		if previousOwner := strings.TrimSpace(s.userByEmail[user.Email]); previousOwner == user.ID {
			delete(s.userByEmail, user.Email)
		}
		user.Email = normalizedEmail
		s.userByEmail[normalizedEmail] = user.ID
		changed = true
	}
	if displayName := strings.TrimSpace(name); displayName != "" && user.Name != displayName {
		user.Name = displayName
		changed = true
	}
	if normalizedProvider := strings.TrimSpace(provider); normalizedProvider != "" && user.Provider != normalizedProvider {
		user.Provider = normalizedProvider
		changed = true
	}
	if changed {
		user.UpdatedAt = util.NowISO()
		_ = s.saveLocked()
	}
	invitedUsers := s.invitedUsersByCodeLocked(user.InviteCode)
	return map[string]any{
		"user_id":              user.ID,
		"email":                user.Email,
		"name":                 user.Name,
		"invite_code":          user.InviteCode,
		"invited_by":           user.InvitedBy,
		"invited_count":        len(invitedUsers),
		"invited_users":        invitedUsers,
		"balance_cents":        user.BalanceCents,
		"total_recharge_cents": user.TotalRechargeCents,
		"total_consume_cents":  user.TotalConsumeCents,
		"last_login_at":        user.LastLoginAt,
		"updated_at":           user.UpdatedAt,
	}
}

func (s *EmailBillingService) ApplyInviteCodeForUser(userID, inviteCode string, imagePriceCents int) error {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return fmt.Errorf("user id is required")
	}
	normalizedInviteCode := normalizeInviteCode(inviteCode)
	if normalizedInviteCode == "" {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	user := s.users[userID]
	if user == nil {
		return fmt.Errorf("user not found")
	}
	if user.InvitedBy != "" {
		return nil
	}
	inviterID := strings.TrimSpace(s.userByInviteCode[normalizedInviteCode])
	if inviterID == "" {
		return fmt.Errorf("invalid invite code")
	}
	if inviterID == user.ID {
		return fmt.Errorf("invalid invite code")
	}
	inviter := s.users[inviterID]
	if inviter == nil || !inviter.Enabled {
		return fmt.Errorf("invalid invite code")
	}

	now := util.NowISO()
	user.InvitedBy = inviter.InviteCode
	user.UpdatedAt = now

	bonusCents := maxBillingInt(0, imagePriceCents) * InviteBonusImageTimes
	var tx map[string]any
	if bonusCents > 0 {
		inviter.BalanceCents += bonusCents
		inviter.TotalRechargeCents += bonusCents
		inviter.UpdatedAt = now
		tx = map[string]any{
			"id":                  "tx_" + util.NewHex(18),
			"user_id":             inviter.ID,
			"email":               inviter.Email,
			"type":                BillingTxTypeRecharge,
			"amount_cents":        bonusCents,
			"balance_after_cents": inviter.BalanceCents,
			"provider":            BillingProviderInviteBonus,
			"note":                fmt.Sprintf("invite bonus from %s (%d image credits)", user.Email, InviteBonusImageTimes),
			"created_at":          now,
		}
		s.transactions = append(s.transactions, tx)
	}

	if err := s.saveLocked(); err != nil {
		user.InvitedBy = ""
		user.UpdatedAt = util.NowISO()
		if bonusCents > 0 {
			inviter.BalanceCents -= bonusCents
			inviter.TotalRechargeCents -= bonusCents
			inviter.UpdatedAt = util.NowISO()
			if len(s.transactions) > 0 {
				s.transactions = s.transactions[:len(s.transactions)-1]
			}
		}
		return err
	}
	return nil
}

func (s *EmailBillingService) ValidateInviteCode(inviteCode string) error {
	normalizedInviteCode := normalizeInviteCode(inviteCode)
	if normalizedInviteCode == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	inviterID := strings.TrimSpace(s.userByInviteCode[normalizedInviteCode])
	if inviterID == "" {
		return fmt.Errorf("invalid invite code")
	}
	inviter := s.users[inviterID]
	if inviter == nil || !inviter.Enabled {
		return fmt.Errorf("invalid invite code")
	}
	return nil
}

func (s *EmailBillingService) ValidateRegisterFingerprint(registerIP, registerDeviceID string, maxPerIP, maxPerDevice int) error {
	registerIP = strings.TrimSpace(registerIP)
	registerDeviceID = strings.TrimSpace(registerDeviceID)
	s.mu.Lock()
	defer s.mu.Unlock()

	if maxPerIP > 0 && registerIP != "" {
		used := 0
		for _, user := range s.users {
			if !user.Enabled || strings.TrimSpace(user.RegisterIP) != registerIP {
				continue
			}
			used++
			if used >= maxPerIP {
				return fmt.Errorf("同 IP 注册次数已达上限")
			}
		}
	}
	if maxPerDevice > 0 && registerDeviceID != "" {
		used := 0
		for _, user := range s.users {
			if !user.Enabled || strings.TrimSpace(user.RegisterDeviceID) != registerDeviceID {
				continue
			}
			used++
			if used >= maxPerDevice {
				return fmt.Errorf("同设备注册次数已达上限")
			}
		}
	}
	return nil
}

func (s *EmailBillingService) BindRegisterFingerprint(userID, registerIP, registerDeviceID string) error {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return fmt.Errorf("user id is required")
	}
	registerIP = strings.TrimSpace(registerIP)
	registerDeviceID = strings.TrimSpace(registerDeviceID)
	if registerIP == "" && registerDeviceID == "" {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	user := s.users[userID]
	if user == nil {
		return fmt.Errorf("user not found")
	}
	changed := false
	if registerIP != "" && user.RegisterIP != registerIP {
		user.RegisterIP = registerIP
		changed = true
	}
	if registerDeviceID != "" && user.RegisterDeviceID != registerDeviceID {
		user.RegisterDeviceID = registerDeviceID
		changed = true
	}
	if !changed {
		return nil
	}
	user.UpdatedAt = util.NowISO()
	return s.saveLocked()
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
		return nil, fmt.Errorf("redeem code is invalid")
	}
	if !item.Enabled {
		return nil, fmt.Errorf("redeem code is disabled")
	}
	if item.UsedBy != "" {
		return nil, fmt.Errorf("redeem code has already been used")
	}
	if item.ExpiresAt != "" {
		expiresAt, err := time.Parse(time.RFC3339Nano, item.ExpiresAt)
		if err != nil || time.Now().UTC().After(expiresAt) {
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
	normalizedExpiresAt, err := normalizeRedeemExpiresAt(expiresAt)
	if err != nil {
		return nil, err
	}
	expiresAt = normalizedExpiresAt
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
		value, err := normalizeRedeemExpiresAt(*expiresAt)
		if err != nil {
			return nil, err
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
	return s.createYiPayOrderLocked(user, amountCents, payType, cfg, "chatgpt2api wallet recharge", BillingOrderKindRecharge, "", "")
}

func (s *EmailBillingService) CreateAgencyYiPayOrder(identity Identity, tier string, amountCents int, payType string, cfg YiPayGatewayConfig, allowUpgradeOnly bool) (map[string]any, error) {
	if !cfg.Enabled || strings.TrimSpace(cfg.PID) == "" || strings.TrimSpace(cfg.Key) == "" || strings.TrimSpace(cfg.SubmitURL) == "" {
		return nil, fmt.Errorf("YiPay is not configured")
	}
	payType = strings.ToLower(strings.TrimSpace(payType))
	switch payType {
	case "alipay", "wxpay", "paypal", "usdt":
	default:
		return nil, fmt.Errorf("unsupported pay type")
	}
	tier = normalizeAgencyTier(tier)
	if tier == "" {
		return nil, fmt.Errorf("invalid agency tier")
	}
	if amountCents < 1 {
		return nil, fmt.Errorf("agency tier price is invalid")
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
	if allowUpgradeOnly && agencyTierRank(tier) < agencyTierRank(user.AgencyTier) {
		return nil, fmt.Errorf("cannot downgrade agency tier")
	}
	if user.AgencyEnabled && user.AgencyTier == tier {
		return nil, fmt.Errorf("agency tier already activated")
	}

	kind := BillingOrderKindAgencyJoin
	if user.AgencyEnabled && agencyTierRank(tier) > agencyTierRank(user.AgencyTier) {
		kind = BillingOrderKindAgencyUpgrade
	}
	note := "chatgpt2api agency tier " + tier
	return s.createYiPayOrderLocked(user, amountCents, payType, cfg, note, kind, tier, note)
}

func (s *EmailBillingService) createYiPayOrderLocked(user *billingUser, amountCents int, payType string, cfg YiPayGatewayConfig, orderName, orderKind, agencyTier, note string) (map[string]any, error) {
	if user == nil {
		return nil, fmt.Errorf("user identity required")
	}
	now := util.NowISO()
	outTradeNo := s.newOutTradeNoLocked()
	order := &billingOrder{
		ID:          "ord_" + util.NewHex(18),
		OutTradeNo:  outTradeNo,
		Provider:    BillingProviderYiPay,
		Kind:        normalizeBillingOrderKind(orderKind),
		UserID:      user.ID,
		UserEmail:   user.Email,
		PayType:     payType,
		AmountCents: amountCents,
		AgencyTier:  normalizeAgencyTier(agencyTier),
		Note:        strings.TrimSpace(note),
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
		"name":         firstNonEmpty(strings.TrimSpace(orderName), "chatgpt2api wallet recharge"),
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
		"order_kind":   order.Kind,
		"agency_tier":  order.AgencyTier,
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

func (s *EmailBillingService) HandleYiPayNotify(values url.Values, cfg YiPayGatewayConfig) (bool, map[string]any, error) {
	if !cfg.Enabled || strings.TrimSpace(cfg.Key) == "" {
		return false, nil, fmt.Errorf("YiPay is not configured")
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
		return false, nil, fmt.Errorf("missing sign")
	}
	expected := yipaySign(flat, cfg.Key)
	if !strings.EqualFold(sign, expected) {
		return false, nil, fmt.Errorf("invalid sign")
	}
	outTradeNo := strings.TrimSpace(flat["out_trade_no"])
	if outTradeNo == "" {
		return false, nil, fmt.Errorf("missing out_trade_no")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	order := s.orders[outTradeNo]
	if order == nil {
		return false, nil, fmt.Errorf("order not found")
	}
	tradeStatus := strings.ToUpper(strings.TrimSpace(flat["trade_status"]))
	if tradeStatus != "TRADE_SUCCESS" {
		order.Status = BillingOrderStatusFailed
		order.UpdatedAt = util.NowISO()
		_ = s.saveLocked()
		return true, yipayNotifyResult(order), nil
	}
	if order.Status == BillingOrderStatusPaid {
		return true, yipayNotifyResult(order), nil
	}
	moneyCents, err := yuanToCents(flat["money"])
	if err != nil {
		return false, nil, fmt.Errorf("invalid money")
	}
	if moneyCents != order.AmountCents {
		return false, nil, fmt.Errorf("money mismatch")
	}
	user := s.users[order.UserID]
	if user == nil {
		return false, nil, fmt.Errorf("user not found")
	}

	now := util.NowISO()
	order.Status = BillingOrderStatusPaid
	order.TradeNo = strings.TrimSpace(flat["trade_no"])
	order.PaidAt = now
	order.UpdatedAt = now

	restoreUser := *user
	hadTx := false
	switch normalizeBillingOrderKind(order.Kind) {
	case BillingOrderKindAgencyJoin, BillingOrderKindAgencyUpgrade:
		// Agency orders only unlock agency tier; no wallet credit is added.
	default:
		creditedCents := order.AmountCents + maxBillingInt(0, order.BonusCents)
		user.BalanceCents += creditedCents
		user.TotalRechargeCents += creditedCents
		user.UpdatedAt = util.NowISO()
		tx := map[string]any{
			"id":                  "tx_" + util.NewHex(18),
			"user_id":             user.ID,
			"email":               user.Email,
			"type":                BillingTxTypeRecharge,
			"amount_cents":        creditedCents,
			"balance_after_cents": user.BalanceCents,
			"order_id":            order.ID,
			"out_trade_no":        order.OutTradeNo,
			"trade_no":            order.TradeNo,
			"provider":            BillingProviderYiPay,
			"note":                billingRechargeNote(order),
			"created_at":          now,
		}
		s.transactions = append(s.transactions, tx)
		hadTx = true
	}

	if err := s.saveLocked(); err != nil {
		*user = restoreUser
		order.Status = BillingOrderStatusPending
		order.TradeNo = ""
		order.PaidAt = ""
		order.UpdatedAt = util.NowISO()
		if hadTx && len(s.transactions) > 0 {
			s.transactions = s.transactions[:len(s.transactions)-1]
		}
		return false, nil, err
	}
	return true, yipayNotifyResult(order), nil
}

func yipayNotifyResult(order *billingOrder) map[string]any {
	if order == nil {
		return map[string]any{}
	}
	return map[string]any{
		"order_id":     order.ID,
		"out_trade_no": order.OutTradeNo,
		"user_id":      order.UserID,
		"order_kind":   normalizeBillingOrderKind(order.Kind),
		"agency_tier":  normalizeAgencyTier(order.AgencyTier),
		"status":       order.Status,
	}
}

func (s *EmailBillingService) ListOrdersByIdentity(identity Identity, limit int) []map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	user := s.ensureUserByIdentityLocked(identity)
	if user == nil {
		return []map[string]any{}
	}
	if limit < 1 {
		limit = 50
	}
	records := make([]map[string]any, 0, len(s.transactions)+len(s.orders))

	for _, tx := range s.transactions {
		if strings.TrimSpace(util.Clean(tx["user_id"])) != user.ID {
			continue
		}
		amountCents := util.ToInt(tx["amount_cents"], 0)
		record := map[string]any{
			"id":                  firstNonEmpty(strings.TrimSpace(util.Clean(tx["id"])), "tx_"+util.NewHex(12)),
			"record_type":         "transaction",
			"type":                strings.TrimSpace(util.Clean(tx["type"])),
			"provider":            strings.TrimSpace(util.Clean(tx["provider"])),
			"status":              BillingOrderStatusPaid,
			"amount_cents":        amountCents,
			"amount_yuan":         centsToYuan(absBillingInt(amountCents)),
			"balance_after_cents": util.ToInt(tx["balance_after_cents"], 0),
			"out_trade_no":        strings.TrimSpace(util.Clean(tx["out_trade_no"])),
			"trade_no":            strings.TrimSpace(util.Clean(tx["trade_no"])),
			"pay_type":            strings.TrimSpace(util.Clean(tx["pay_type"])),
			"note":                strings.TrimSpace(util.Clean(tx["note"])),
			"created_at":          strings.TrimSpace(util.Clean(tx["created_at"])),
		}
		records = append(records, record)
	}

	for _, order := range s.orders {
		if order.UserID != user.ID {
			continue
		}
		if order.Status == BillingOrderStatusPaid {
			continue
		}
		record := publicBillingOrder(order)
		record["record_type"] = "order"
		records = append(records, record)
	}

	sort.Slice(records, func(i, j int) bool {
		left := parseBillingTimestamp(strings.TrimSpace(util.Clean(records[i]["created_at"])))
		right := parseBillingTimestamp(strings.TrimSpace(util.Clean(records[j]["created_at"])))
		if !left.Equal(right) {
			return left.After(right)
		}
		return strings.TrimSpace(util.Clean(records[i]["id"])) > strings.TrimSpace(util.Clean(records[j]["id"]))
	})

	if len(records) > limit {
		records = records[:limit]
	}
	return records
}

func (s *EmailBillingService) ListOrdersForAdmin(limit int) ([]map[string]any, map[string]any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := make([]*billingOrder, 0, len(s.orders))
	for _, order := range s.orders {
		items = append(items, order)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].CreatedAt > items[j].CreatedAt
	})

	now := time.Now().In(billingBeijingLocation)
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, billingBeijingLocation)
	todayEnd := todayStart.Add(24 * time.Hour)

	totalPaidCount := 0
	totalRevenueCents := 0
	todayPaidCount := 0
	todayRevenueCents := 0
	pendingCount := 0
	failedCount := 0

	for _, order := range items {
		switch order.Status {
		case BillingOrderStatusPending:
			pendingCount++
		case BillingOrderStatusFailed:
			failedCount++
		case BillingOrderStatusPaid:
			totalPaidCount++
			totalRevenueCents += order.AmountCents

			paidAt := parseBillingTimestamp(firstNonEmpty(order.PaidAt, order.UpdatedAt, order.CreatedAt))
			if !paidAt.IsZero() {
				localPaidAt := paidAt.In(billingBeijingLocation)
				if !localPaidAt.Before(todayStart) && localPaidAt.Before(todayEnd) {
					todayPaidCount++
					todayRevenueCents += order.AmountCents
				}
			}
		}
	}

	out := make([]map[string]any, 0, len(items))
	for _, order := range items {
		out = append(out, adminBillingOrder(order))
		if limit > 0 && len(out) >= limit {
			break
		}
	}

	return out, map[string]any{
		"today_revenue_cents": todayRevenueCents,
		"today_revenue_yuan":  centsToYuan(todayRevenueCents),
		"today_paid_count":    todayPaidCount,
		"total_revenue_cents": totalRevenueCents,
		"total_revenue_yuan":  centsToYuan(totalRevenueCents),
		"total_paid_count":    totalPaidCount,
		"pending_count":       pendingCount,
		"failed_count":        failedCount,
		"record_count":        len(items),
		"updated_at":          util.NowISO(),
	}
}

func (s *EmailBillingService) loadLocked() {
	raw := loadStoredJSON(s.store, s.docName, s.path)
	obj, _ := raw.(map[string]any)
	users := util.AsMapSlice(obj["users"])
	registerCodes := util.AsMapSlice(obj["register_codes"])
	redeemCodes := util.AsMapSlice(obj["redeem_codes"])
	orders := util.AsMapSlice(obj["orders"])
	withdrawals := util.AsMapSlice(obj["withdrawals"])
	transactions := util.AsMapSlice(obj["transactions"])

	s.users = map[string]*billingUser{}
	s.userByEmail = map[string]string{}
	s.userByInviteCode = map[string]string{}
	s.registerCodes = map[string]*billingRegisterCode{}
	s.redeemCodes = map[string]*billingRedeemCode{}
	s.orders = map[string]*billingOrder{}
	s.withdrawals = make([]*billingWithdrawalRequest, 0, len(withdrawals))
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
	for _, rawWithdraw := range withdrawals {
		item := normalizeBillingWithdrawalRequest(rawWithdraw)
		if item == nil {
			continue
		}
		s.withdrawals = append(s.withdrawals, item)
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
	withdrawals := make([]map[string]any, 0, len(s.withdrawals))
	for _, item := range s.withdrawals {
		if item == nil {
			continue
		}
		withdrawals = append(withdrawals, billingWithdrawalRequestToMap(item))
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
	sort.Slice(withdrawals, func(i, j int) bool {
		return util.Clean(withdrawals[i]["created_at"]) > util.Clean(withdrawals[j]["created_at"])
	})
	value := map[string]any{
		"users":          users,
		"register_codes": registerCodes,
		"redeem_codes":   redeemCodes,
		"orders":         orders,
		"withdrawals":    withdrawals,
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

func (s *EmailBillingService) invitedUsersByCodeLocked(inviteCode string) []map[string]any {
	inviteCode = normalizeInviteCode(inviteCode)
	if inviteCode == "" {
		return []map[string]any{}
	}
	out := make([]map[string]any, 0)
	for _, user := range s.users {
		if normalizeInviteCode(user.InvitedBy) != inviteCode {
			continue
		}
		out = append(out, map[string]any{
			"id":         user.ID,
			"email":      user.Email,
			"name":       user.Name,
			"created_at": user.CreatedAt,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		left := strings.TrimSpace(util.Clean(out[i]["created_at"]))
		right := strings.TrimSpace(util.Clean(out[j]["created_at"]))
		if left != right {
			return left > right
		}
		return strings.TrimSpace(util.Clean(out[i]["email"])) < strings.TrimSpace(util.Clean(out[j]["email"]))
	})
	return out
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
	bonusCents := 0
	if user != nil && user.AgencyEnabled && user.AgencyDiscountBP > 0 {
		bonusCents = amountCents * user.AgencyDiscountBP / 10000
	}
	return &billingOrder{
		ID:          "ord_" + util.NewHex(18),
		OutTradeNo:  s.newOutTradeNoLocked(),
		Provider:    strings.TrimSpace(provider),
		Kind:        BillingOrderKindRecharge,
		UserID:      user.ID,
		UserEmail:   user.Email,
		PayType:     strings.ToLower(strings.TrimSpace(payType)),
		AmountCents: amountCents,
		BonusCents:  bonusCents,
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
		AgencyTier:         normalizeAgencyTier(raw["agency_tier"]),
		AgencyEnabled:      util.ToBool(raw["agency_enabled"]),
		AgencyCommissionBP: normalizeAgencyBasisPoint(raw["agency_commission_bp"]),
		AgencyDiscountBP:   normalizeAgencyBasisPoint(raw["agency_discount_bp"]),
		AgencyJoinedAt:     util.Clean(raw["agency_joined_at"]),
		AgencyAlipayQRCode: strings.TrimSpace(util.Clean(raw["agency_alipay_qr_code"])),
		AgencyWeChatQRCode: strings.TrimSpace(util.Clean(raw["agency_wechat_qr_code"])),
		AgencyPhone:        strings.TrimSpace(util.Clean(raw["agency_phone"])),
		AgencyWeChatID:     strings.TrimSpace(util.Clean(raw["agency_wechat_id"])),
		RegisterIP:         strings.TrimSpace(util.Clean(raw["register_ip"])),
		RegisterDeviceID:   strings.TrimSpace(util.Clean(raw["register_device_id"])),
		CreatedAt:          firstNonEmpty(util.Clean(raw["created_at"]), util.NowISO()),
		UpdatedAt:          firstNonEmpty(util.Clean(raw["updated_at"]), util.Clean(raw["created_at"]), util.NowISO()),
		LastLoginAt:        util.Clean(raw["last_login_at"]),
	}
}

func billingUserToMap(user *billingUser) map[string]any {
	return map[string]any{
		"id":                    user.ID,
		"email":                 user.Email,
		"provider":              user.Provider,
		"name":                  user.Name,
		"invite_code":           user.InviteCode,
		"invited_by":            user.InvitedBy,
		"password_hash":         user.PasswordHash,
		"auth_key_id":           user.AuthKeyID,
		"enabled":               user.Enabled,
		"balance_cents":         user.BalanceCents,
		"total_recharge_cents":  user.TotalRechargeCents,
		"total_consume_cents":   user.TotalConsumeCents,
		"agency_tier":           user.AgencyTier,
		"agency_enabled":        user.AgencyEnabled,
		"agency_commission_bp":  user.AgencyCommissionBP,
		"agency_discount_bp":    user.AgencyDiscountBP,
		"agency_joined_at":      user.AgencyJoinedAt,
		"agency_alipay_qr_code": user.AgencyAlipayQRCode,
		"agency_wechat_qr_code": user.AgencyWeChatQRCode,
		"agency_phone":          user.AgencyPhone,
		"agency_wechat_id":      user.AgencyWeChatID,
		"register_ip":           user.RegisterIP,
		"register_device_id":    user.RegisterDeviceID,
		"created_at":            user.CreatedAt,
		"updated_at":            user.UpdatedAt,
		"last_login_at":         user.LastLoginAt,
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
		"agency_tier":          user.AgencyTier,
		"agency_enabled":       user.AgencyEnabled,
		"agency_commission_bp": user.AgencyCommissionBP,
		"agency_discount_bp":   user.AgencyDiscountBP,
		"agency_joined_at":     user.AgencyJoinedAt,
		"created_at":           user.CreatedAt,
		"updated_at":           user.UpdatedAt,
		"last_login_at":        user.LastLoginAt,
	}
}

func publicBillingAgencyWithdrawProfile(user *billingUser) map[string]any {
	if user == nil {
		return map[string]any{}
	}
	return map[string]any{
		"alipay_qr_code": user.AgencyAlipayQRCode,
		"wechat_qr_code": user.AgencyWeChatQRCode,
		"phone":          user.AgencyPhone,
		"wechat_id":      user.AgencyWeChatID,
	}
}

func normalizeAgencyTier(value any) string {
	switch strings.ToLower(strings.TrimSpace(util.Clean(value))) {
	case AgencyTierBasic:
		return AgencyTierBasic
	case AgencyTierPro:
		return AgencyTierPro
	case AgencyTierPremium:
		return AgencyTierPremium
	default:
		return ""
	}
}

func normalizeAgencyBasisPoint(value any) int {
	bp := util.ToInt(value, 0)
	if bp < 0 {
		return 0
	}
	if bp > 10000 {
		return 10000
	}
	return bp
}

func normalizeBillingOrderKind(value any) string {
	switch strings.ToLower(strings.TrimSpace(util.Clean(value))) {
	case BillingOrderKindAgencyJoin:
		return BillingOrderKindAgencyJoin
	case BillingOrderKindAgencyUpgrade:
		return BillingOrderKindAgencyUpgrade
	default:
		return BillingOrderKindRecharge
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

func normalizeAgencyWithdrawStatus(value any) string {
	switch strings.ToLower(strings.TrimSpace(util.Clean(value))) {
	case BillingWithdrawStatusApproved:
		return BillingWithdrawStatusApproved
	case BillingWithdrawStatusRejected:
		return BillingWithdrawStatusRejected
	case BillingWithdrawStatusPaid:
		return BillingWithdrawStatusPaid
	default:
		return BillingWithdrawStatusPending
	}
}

func normalizeBillingWithdrawalRequest(raw map[string]any) *billingWithdrawalRequest {
	id := strings.TrimSpace(util.Clean(raw["id"]))
	userID := strings.TrimSpace(util.Clean(raw["user_id"]))
	if id == "" || userID == "" {
		return nil
	}
	amountCents := maxBillingInt(0, util.ToInt(raw["amount_cents"], 0))
	if amountCents <= 0 {
		return nil
	}
	createdAt := firstNonEmpty(strings.TrimSpace(util.Clean(raw["created_at"])), util.NowISO())
	updatedAt := firstNonEmpty(strings.TrimSpace(util.Clean(raw["updated_at"])), createdAt)
	return &billingWithdrawalRequest{
		ID:           id,
		UserID:       userID,
		UserEmail:    strings.TrimSpace(util.Clean(raw["user_email"])),
		AmountCents:  amountCents,
		AlipayQRCode: strings.TrimSpace(util.Clean(raw["alipay_qr_code"])),
		WeChatQRCode: strings.TrimSpace(util.Clean(raw["wechat_qr_code"])),
		Phone:        strings.TrimSpace(util.Clean(raw["phone"])),
		WeChatID:     strings.TrimSpace(util.Clean(raw["wechat_id"])),
		Status:       normalizeAgencyWithdrawStatus(raw["status"]),
		AdminNote:    strings.TrimSpace(util.Clean(raw["admin_note"])),
		CreatedAt:    createdAt,
		UpdatedAt:    updatedAt,
		ProcessedAt:  strings.TrimSpace(util.Clean(raw["processed_at"])),
	}
}

func billingWithdrawalRequestToMap(item *billingWithdrawalRequest) map[string]any {
	return map[string]any{
		"id":             item.ID,
		"user_id":        item.UserID,
		"user_email":     item.UserEmail,
		"amount_cents":   item.AmountCents,
		"alipay_qr_code": item.AlipayQRCode,
		"wechat_qr_code": item.WeChatQRCode,
		"phone":          item.Phone,
		"wechat_id":      item.WeChatID,
		"status":         normalizeAgencyWithdrawStatus(item.Status),
		"admin_note":     item.AdminNote,
		"created_at":     item.CreatedAt,
		"updated_at":     item.UpdatedAt,
		"processed_at":   item.ProcessedAt,
	}
}

func publicBillingWithdrawalRequest(item *billingWithdrawalRequest) map[string]any {
	return map[string]any{
		"id":             item.ID,
		"user_id":        item.UserID,
		"user_email":     item.UserEmail,
		"amount_cents":   item.AmountCents,
		"amount_yuan":    centsToYuan(item.AmountCents),
		"alipay_qr_code": item.AlipayQRCode,
		"wechat_qr_code": item.WeChatQRCode,
		"phone":          item.Phone,
		"wechat_id":      item.WeChatID,
		"status":         normalizeAgencyWithdrawStatus(item.Status),
		"admin_note":     item.AdminNote,
		"created_at":     item.CreatedAt,
		"updated_at":     item.UpdatedAt,
		"processed_at":   item.ProcessedAt,
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
		Kind:        normalizeBillingOrderKind(raw["order_kind"]),
		UserID:      userID,
		UserEmail:   strings.TrimSpace(util.Clean(raw["user_email"])),
		PayType:     strings.TrimSpace(util.Clean(raw["pay_type"])),
		AmountCents: maxBillingInt(0, util.ToInt(raw["amount_cents"], 0)),
		BonusCents:  maxBillingInt(0, util.ToInt(raw["bonus_cents"], 0)),
		AgencyTier:  normalizeAgencyTier(raw["agency_tier"]),
		Note:        strings.TrimSpace(util.Clean(raw["note"])),
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
		"order_kind":   normalizeBillingOrderKind(order.Kind),
		"user_id":      order.UserID,
		"user_email":   order.UserEmail,
		"pay_type":     order.PayType,
		"amount_cents": order.AmountCents,
		"bonus_cents":  order.BonusCents,
		"agency_tier":  normalizeAgencyTier(order.AgencyTier),
		"note":         strings.TrimSpace(order.Note),
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
		"order_kind":   normalizeBillingOrderKind(order.Kind),
		"pay_type":     order.PayType,
		"amount_cents": order.AmountCents,
		"amount_yuan":  centsToYuan(order.AmountCents),
		"bonus_cents":  order.BonusCents,
		"bonus_yuan":   centsToYuan(order.BonusCents),
		"credit_cents": order.AmountCents + maxBillingInt(0, order.BonusCents),
		"credit_yuan":  centsToYuan(order.AmountCents + maxBillingInt(0, order.BonusCents)),
		"agency_tier":  normalizeAgencyTier(order.AgencyTier),
		"note":         strings.TrimSpace(order.Note),
		"status":       order.Status,
		"created_at":   order.CreatedAt,
		"updated_at":   order.UpdatedAt,
		"paid_at":      order.PaidAt,
	}
}

func adminBillingOrder(order *billingOrder) map[string]any {
	item := publicBillingOrder(order)
	item["user_id"] = order.UserID
	item["user_email"] = order.UserEmail
	return item
}

func parseBillingTimestamp(value string) time.Time {
	text := strings.TrimSpace(value)
	if text == "" {
		return time.Time{}
	}
	if parsed, err := time.Parse(time.RFC3339Nano, text); err == nil {
		return parsed
	}
	if parsed, err := time.Parse(time.RFC3339, text); err == nil {
		return parsed
	}
	if parsed, err := time.ParseInLocation("2006-01-02 15:04:05", text, time.UTC); err == nil {
		return parsed
	}
	return time.Time{}
}

func normalizeRedeemExpiresAt(value string) (string, error) {
	text := strings.TrimSpace(value)
	if text == "" {
		return "", nil
	}
	parseLayouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
		"2006-01-02T15:04",
	}
	var parsed time.Time
	for _, layout := range parseLayouts {
		var t time.Time
		var err error
		if layout == time.RFC3339Nano || layout == time.RFC3339 {
			t, err = time.Parse(layout, text)
		} else {
			t, err = time.ParseInLocation(layout, text, billingBeijingLocation)
		}
		if err == nil {
			parsed = t
			break
		}
	}
	if parsed.IsZero() {
		return "", fmt.Errorf("expires_at 时间格式无效")
	}
	if time.Now().UTC().After(parsed.UTC()) {
		return "", fmt.Errorf("过期时间必须晚于当前时间")
	}
	return parsed.UTC().Format(time.RFC3339Nano), nil
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

func absBillingInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
