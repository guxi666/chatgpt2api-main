package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"chatgpt2api/internal/objectstore"
	"chatgpt2api/internal/storage"
	"chatgpt2api/internal/util"
)

var settingEnvKeys = map[string]string{
	"base_url":                           "CHATGPT2API_BASE_URL",
	"brand_top_left_name":                "CHATGPT2API_BRAND_TOP_LEFT_NAME",
	"brand_site_name":                    "CHATGPT2API_BRAND_SITE_NAME",
	"brand_top_left_logo_url":            "CHATGPT2API_BRAND_TOP_LEFT_LOGO_URL",
	"brand_site_logo_url":                "CHATGPT2API_BRAND_SITE_LOGO_URL",
	"proxy":                              "CHATGPT2API_PROXY",
	"refresh_account_interval_minute":    "CHATGPT2API_REFRESH_ACCOUNT_INTERVAL_MINUTE",
	"image_concurrent_limit":             "CHATGPT2API_IMAGE_CONCURRENT_LIMIT",
	"image_single_count_limit":           "CHATGPT2API_IMAGE_SINGLE_COUNT_LIMIT",
	"image_task_timeout_seconds":         "CHATGPT2API_IMAGE_TASK_TIMEOUT_SECONDS",
	"user_default_concurrent_limit":      "CHATGPT2API_USER_DEFAULT_CONCURRENT_LIMIT",
	"user_default_rpm_limit":             "CHATGPT2API_USER_DEFAULT_RPM_LIMIT",
	"image_retention_days":               "CHATGPT2API_IMAGE_RETENTION_DAYS",
	"auto_remove_invalid_accounts":       "CHATGPT2API_AUTO_REMOVE_INVALID_ACCOUNTS",
	"auto_remove_rate_limited_accounts":  "CHATGPT2API_AUTO_REMOVE_RATE_LIMITED_ACCOUNTS",
	"log_retention_days":                 "CHATGPT2API_LOG_RETENTION_DAYS",
	"log_levels":                         "CHATGPT2API_LOG_LEVELS",
	"linuxdo_enabled":                    "CHATGPT2API_LINUXDO_ENABLED",
	"linuxdo_client_id":                  "CHATGPT2API_LINUXDO_CLIENT_ID",
	"linuxdo_client_secret":              "CHATGPT2API_LINUXDO_CLIENT_SECRET",
	"linuxdo_redirect_url":               "CHATGPT2API_LINUXDO_REDIRECT_URL",
	"linuxdo_frontend_redirect_url":      "CHATGPT2API_LINUXDO_FRONTEND_REDIRECT_URL",
	"update_repo":                        "CHATGPT2API_UPDATE_REPO",
	"update_github_token":                "CHATGPT2API_UPDATE_GITHUB_TOKEN",
	"registration_enabled":               "CHATGPT2API_REGISTRATION_ENABLED",
	"registration_allowed_email_domains": "CHATGPT2API_REGISTRATION_ALLOWED_EMAIL_DOMAINS",
	"registration_bonus_image_times":     "CHATGPT2API_REGISTRATION_BONUS_IMAGE_TIMES",
	"cf_turnstile_site_key":              "CHATGPT2API_CF_TURNSTILE_SITE_KEY",
	"cf_turnstile_secret_key":            "CHATGPT2API_CF_TURNSTILE_SECRET_KEY",
	"email_smtp_enabled":                 "CHATGPT2API_EMAIL_SMTP_ENABLED",
	"email_smtp_host":                    "CHATGPT2API_EMAIL_SMTP_HOST",
	"email_smtp_port":                    "CHATGPT2API_EMAIL_SMTP_PORT",
	"email_smtp_use_ssl":                 "CHATGPT2API_EMAIL_SMTP_USE_SSL",
	"email_smtp_username":                "CHATGPT2API_EMAIL_SMTP_USERNAME",
	"email_smtp_auth_code":               "CHATGPT2API_EMAIL_SMTP_AUTH_CODE",
	"email_smtp_from_email":              "CHATGPT2API_EMAIL_SMTP_FROM_EMAIL",
	"email_smtp_from_name":               "CHATGPT2API_EMAIL_SMTP_FROM_NAME",
	"image_price_cents":                  "CHATGPT2API_IMAGE_PRICE_CENTS",
	"image_price_1k_cents":               "CHATGPT2API_IMAGE_PRICE_1K_CENTS",
	"image_price_2k_cents":               "CHATGPT2API_IMAGE_PRICE_2K_CENTS",
	"image_price_4k_cents":               "CHATGPT2API_IMAGE_PRICE_4K_CENTS",
	"agency_tier_basic_cents":            "CHATGPT2API_AGENCY_TIER_BASIC_CENTS",
	"agency_tier_pro_cents":              "CHATGPT2API_AGENCY_TIER_PRO_CENTS",
	"agency_tier_premium_cents":          "CHATGPT2API_AGENCY_TIER_PREMIUM_CENTS",
	"agency_tier_basic_commission_bp":    "CHATGPT2API_AGENCY_TIER_BASIC_COMMISSION_BP",
	"agency_tier_pro_commission_bp":      "CHATGPT2API_AGENCY_TIER_PRO_COMMISSION_BP",
	"agency_tier_premium_commission_bp":  "CHATGPT2API_AGENCY_TIER_PREMIUM_COMMISSION_BP",
	"agency_tier_basic_discount_bp":      "CHATGPT2API_AGENCY_TIER_BASIC_DISCOUNT_BP",
	"agency_tier_pro_discount_bp":        "CHATGPT2API_AGENCY_TIER_PRO_DISCOUNT_BP",
	"agency_tier_premium_discount_bp":    "CHATGPT2API_AGENCY_TIER_PREMIUM_DISCOUNT_BP",
	"yipay_enabled":                      "CHATGPT2API_YIPAY_ENABLED",
	"yipay_pid":                          "CHATGPT2API_YIPAY_PID",
	"yipay_key":                          "CHATGPT2API_YIPAY_KEY",
	"yipay_submit_url":                   "CHATGPT2API_YIPAY_SUBMIT_URL",
	"yipay_notify_url":                   "CHATGPT2API_YIPAY_NOTIFY_URL",
	"yipay_return_url":                   "CHATGPT2API_YIPAY_RETURN_URL",
	"yipay_site_name":                    "CHATGPT2API_YIPAY_SITE_NAME",
	"paypal_enabled":                     "CHATGPT2API_PAYPAL_ENABLED",
	"paypal_checkout_url":                "CHATGPT2API_PAYPAL_CHECKOUT_URL",
	"usdt_enabled":                       "CHATGPT2API_USDT_ENABLED",
	"usdt_network":                       "CHATGPT2API_USDT_NETWORK",
	"usdt_address":                       "CHATGPT2API_USDT_ADDRESS",
	"usdt_payment_url":                   "CHATGPT2API_USDT_PAYMENT_URL",
	"login_page_image_url":               "CHATGPT2API_LOGIN_PAGE_IMAGE_URL",
	"login_page_image_mode":              "CHATGPT2API_LOGIN_PAGE_IMAGE_MODE",
	"login_page_image_zoom":              "CHATGPT2API_LOGIN_PAGE_IMAGE_ZOOM",
	"login_page_image_position_x":        "CHATGPT2API_LOGIN_PAGE_IMAGE_POSITION_X",
	"login_page_image_position_y":        "CHATGPT2API_LOGIN_PAGE_IMAGE_POSITION_Y",
	"image_r2_enabled":                   "CHATGPT2API_IMAGE_R2_ENABLED",
	"image_r2_endpoint":                  "CHATGPT2API_IMAGE_R2_ENDPOINT",
	"image_r2_bucket":                    "CHATGPT2API_IMAGE_R2_BUCKET",
	"image_r2_region":                    "CHATGPT2API_IMAGE_R2_REGION",
	"image_r2_access_key_id":             "CHATGPT2API_IMAGE_R2_ACCESS_KEY_ID",
	"image_r2_secret_access_key":         "CHATGPT2API_IMAGE_R2_SECRET_ACCESS_KEY",
	"image_r2_public_base_url":           "CHATGPT2API_IMAGE_R2_PUBLIC_BASE_URL",
	"image_r2_prefix":                    "CHATGPT2API_IMAGE_R2_PREFIX",
}

var envKeyRE = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

const (
	defaultImageTaskTimeoutSeconds = 300
	minImageTaskTimeoutSeconds     = 30
	maxImageTaskTimeoutSeconds     = 3600
)

type Store struct {
	mu              sync.RWMutex
	RootDir         string
	DataDir         string
	EnvFile         string
	data            map[string]any
	externalEnvKeys map[string]struct{}
	storageBackend  storage.Backend
}

type LinuxDoOAuthConfig struct {
	Enabled              bool
	ClientID             string
	ClientSecret         string
	AuthorizeURL         string
	TokenURL             string
	UserInfoURL          string
	Scopes               string
	RedirectURL          string
	FrontendRedirectURL  string
	TokenAuthMethod      string
	UsePKCE              bool
	UserInfoEmailPath    string
	UserInfoIDPath       string
	UserInfoUsernamePath string
}

type EmailSMTPConfig struct {
	Enabled   bool
	Host      string
	Port      int
	UseSSL    bool
	Username  string
	Password  string
	FromEmail string
	FromName  string
}

type YiPayConfig struct {
	Enabled   bool
	PID       string
	Key       string
	SubmitURL string
	NotifyURL string
	ReturnURL string
	SiteName  string
}

type PayPalConfig struct {
	Enabled     bool
	CheckoutURL string
}

type USDTConfig struct {
	Enabled    bool
	Network    string
	Address    string
	PaymentURL string
}

func NewStore() (*Store, error) {
	root, err := resolveRootDir()
	if err != nil {
		return nil, err
	}

	envFile := filepath.Join(root, ".env")
	envFileValues := readEnvObject(envFile)
	s := &Store{
		RootDir:         root,
		DataDir:         filepath.Join(root, "data"),
		EnvFile:         envFile,
		data:            map[string]any{},
		externalEnvKeys: map[string]struct{}{},
	}
	for _, item := range os.Environ() {
		key, value, _ := strings.Cut(item, "=")
		if fileValue, ok := envFileValues[key]; ok && value == fileValue {
			continue
		}
		s.externalEnvKeys[key] = struct{}{}
	}
	if err := os.MkdirAll(s.DataDir, 0o755); err != nil {
		return nil, err
	}
	s.loadEnvFile()
	s.data = settingsFromEnvValues(envFileValues)
	return s, nil
}

func resolveRootDir() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	if configured := strings.TrimSpace(os.Getenv("CHATGPT2API_ROOT")); configured != "" {
		return filepath.Abs(configured)
	}
	if root := findAncestorWithFile(cwd, ".env"); root != "" {
		return root, nil
	}
	if root := findAncestorWithProjectGoMod(cwd); root != "" {
		return root, nil
	}
	return filepath.Abs(cwd)
}

func findAncestorWithFile(start, name string) string {
	dir, err := filepath.Abs(start)
	if err != nil {
		return ""
	}
	for {
		info, statErr := os.Stat(filepath.Join(dir, name))
		if statErr == nil && !info.IsDir() {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func findAncestorWithProjectGoMod(start string) string {
	dir, err := filepath.Abs(start)
	if err != nil {
		return ""
	}
	for {
		data, readErr := os.ReadFile(filepath.Join(dir, "go.mod"))
		if readErr == nil && strings.Contains(string(data), "module chatgpt2api") {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func (s *Store) AdminUsername() string {
	value := strings.TrimSpace(os.Getenv("CHATGPT2API_ADMIN_USERNAME"))
	if value == "" {
		return "admin"
	}
	return value
}

func (s *Store) AdminPassword() string {
	return strings.TrimSpace(os.Getenv("CHATGPT2API_ADMIN_PASSWORD"))
}

func (s *Store) RegistrationEnabled() bool {
	return util.ToBool(s.settingValue("registration_enabled", false))
}

func (s *Store) CFTurnstileSiteKey() string {
	return strings.TrimSpace(fmt.Sprint(s.settingValue("cf_turnstile_site_key", "")))
}

func (s *Store) CFTurnstileSecretKey() string {
	return strings.TrimSpace(fmt.Sprint(s.settingValue("cf_turnstile_secret_key", "")))
}

func (s *Store) CFTurnstileEnabled() bool {
	return s.CFTurnstileSiteKey() != "" && s.CFTurnstileSecretKey() != ""
}

func (s *Store) RegistrationAllowedEmailDomains() []string {
	raw := strings.TrimSpace(fmt.Sprint(s.settingValue("registration_allowed_email_domains", "")))
	if raw == "" {
		raw = "qq.com,163.com,126.com,gmail.com,outlook.com,hotmail.com,icloud.com,yahoo.com,foxmail.com,sina.com"
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, 16)
	for _, part := range strings.Split(raw, ",") {
		domain := strings.ToLower(strings.TrimSpace(part))
		if domain == "" {
			continue
		}
		if _, ok := seen[domain]; ok {
			continue
		}
		seen[domain] = struct{}{}
		out = append(out, domain)
	}
	return out
}

func (s *Store) RegistrationBonusImageTimes() int {
	value := intSetting(s.settingValue("registration_bonus_image_times", 20), 20)
	if value < 0 {
		return 0
	}
	if value > 100000 {
		return 100000
	}
	return value
}

func (s *Store) EmailAllowedDomains() []string {
	return s.RegistrationAllowedEmailDomains()
}

func (s *Store) ImagePriceCents() int {
	return s.ImagePrice1KCents()
}

func (s *Store) ImagePrice1KCents() int {
	value := intSetting(s.settingValue("image_price_1k_cents", s.settingValue("image_price_cents", 8)), 8)
	if value < 1 {
		return 1
	}
	return value
}

func (s *Store) ImagePrice2KCents() int {
	base := s.ImagePrice1KCents()
	value := intSetting(s.settingValue("image_price_2k_cents", base*2), base*2)
	if value < 1 {
		return 1
	}
	return value
}

func (s *Store) ImagePrice4KCents() int {
	base := s.ImagePrice1KCents()
	value := intSetting(s.settingValue("image_price_4k_cents", base*4), base*4)
	if value < 1 {
		return 1
	}
	return value
}

func (s *Store) AgencyTierBasicCents() int {
	value := intSetting(s.settingValue("agency_tier_basic_cents", 19900), 19900)
	if value < 0 {
		return 0
	}
	return value
}

func (s *Store) AgencyTierProCents() int {
	value := intSetting(s.settingValue("agency_tier_pro_cents", 49900), 49900)
	if value < 0 {
		return 0
	}
	return value
}

func (s *Store) AgencyTierPremiumCents() int {
	value := intSetting(s.settingValue("agency_tier_premium_cents", 99900), 99900)
	if value < 0 {
		return 0
	}
	return value
}

func (s *Store) AgencyTierBasicCommissionBP() int {
	return clampAgencyBasisPoint(intSetting(s.settingValue("agency_tier_basic_commission_bp", 3000), 3000))
}

func (s *Store) AgencyTierProCommissionBP() int {
	return clampAgencyBasisPoint(intSetting(s.settingValue("agency_tier_pro_commission_bp", 4500), 4500))
}

func (s *Store) AgencyTierPremiumCommissionBP() int {
	return clampAgencyBasisPoint(intSetting(s.settingValue("agency_tier_premium_commission_bp", 6000), 6000))
}

func (s *Store) AgencyTierBasicDiscountBP() int {
	return clampAgencyBasisPoint(intSetting(s.settingValue("agency_tier_basic_discount_bp", 500), 500))
}

func (s *Store) AgencyTierProDiscountBP() int {
	return clampAgencyBasisPoint(intSetting(s.settingValue("agency_tier_pro_discount_bp", 1000), 1000))
}

func (s *Store) AgencyTierPremiumDiscountBP() int {
	return clampAgencyBasisPoint(intSetting(s.settingValue("agency_tier_premium_discount_bp", 1500), 1500))
}

func clampAgencyBasisPoint(value int) int {
	if value < 0 {
		return 0
	}
	if value > 10000 {
		return 10000
	}
	return value
}

func (s *Store) EmailSMTP() EmailSMTPConfig {
	port := intSetting(s.settingValue("email_smtp_port", 465), 465)
	if port <= 0 {
		port = 465
	}
	return EmailSMTPConfig{
		Enabled:   util.ToBool(s.settingValue("email_smtp_enabled", false)),
		Host:      strings.TrimSpace(fmt.Sprint(s.settingValue("email_smtp_host", ""))),
		Port:      port,
		UseSSL:    util.ToBool(s.settingValue("email_smtp_use_ssl", true)),
		Username:  strings.TrimSpace(fmt.Sprint(s.settingValue("email_smtp_username", ""))),
		Password:  strings.TrimSpace(fmt.Sprint(s.settingValue("email_smtp_auth_code", ""))),
		FromEmail: strings.TrimSpace(fmt.Sprint(s.settingValue("email_smtp_from_email", ""))),
		FromName:  strings.TrimSpace(fmt.Sprint(s.settingValue("email_smtp_from_name", "chatgpt2api"))),
	}
}

func (s *Store) YiPay() YiPayConfig {
	s.mu.RLock()
	data := util.CopyMap(s.data)
	s.mu.RUnlock()
	return s.yiPayFromData(data)
}

func (s *Store) PayPal() PayPalConfig {
	s.mu.RLock()
	data := util.CopyMap(s.data)
	s.mu.RUnlock()
	return s.payPalFromData(data)
}

func (s *Store) USDT() USDTConfig {
	s.mu.RLock()
	data := util.CopyMap(s.data)
	s.mu.RUnlock()
	return s.usdtFromData(data)
}

func (s *Store) RefreshAccountIntervalMinute() int {
	return intSetting(s.settingValue("refresh_account_interval_minute", 5), 5)
}

func (s *Store) ImageRetentionDays() int {
	value := intSetting(s.settingValue("image_retention_days", 30), 30)
	if value < 1 {
		return 1
	}
	return value
}

func (s *Store) LogRetentionDays() int {
	value := intSetting(s.settingValue("log_retention_days", 7), 7)
	if value < 1 {
		return 1
	}
	if value > 3650 {
		return 3650
	}
	return value
}

func (s *Store) ImageConcurrentLimit() int {
	value := intSetting(s.settingValue("image_concurrent_limit", 4), 4)
	if value < 1 {
		return 1
	}
	return value
}

func (s *Store) ImageSingleCountLimit() int {
	value := intSetting(s.settingValue("image_single_count_limit", 10), 10)
	if value < 1 {
		return 1
	}
	if value > 10 {
		return 10
	}
	return value
}

func (s *Store) ImageTaskTimeoutSeconds() int {
	return normalizeImageTaskTimeoutSeconds(s.settingValue("image_task_timeout_seconds", defaultImageTaskTimeoutSeconds))
}

func (s *Store) UserDefaultConcurrentLimit() int {
	value := intSetting(s.settingValue("user_default_concurrent_limit", 0), 0)
	if value < 0 {
		return 0
	}
	return value
}

func (s *Store) UserDefaultRPMLimit() int {
	value := intSetting(s.settingValue("user_default_rpm_limit", 0), 0)
	if value < 0 {
		return 0
	}
	return value
}

func (s *Store) AutoRemoveInvalidAccounts() bool {
	return util.ToBool(s.settingValue("auto_remove_invalid_accounts", false))
}

func (s *Store) AutoRemoveRateLimitedAccounts() bool {
	return util.ToBool(s.settingValue("auto_remove_rate_limited_accounts", false))
}

func (s *Store) BaseURL() string {
	return strings.TrimRight(strings.TrimSpace(fmt.Sprint(s.settingValue("base_url", ""))), "/")
}

func (s *Store) BrandTopLeftName() string {
	value := strings.TrimSpace(fmt.Sprint(s.settingValue("brand_top_left_name", "chatgpt2api")))
	if value == "" {
		return "chatgpt2api"
	}
	return value
}

func (s *Store) BrandSiteName() string {
	value := strings.TrimSpace(fmt.Sprint(s.settingValue("brand_site_name", "chatgpt2api")))
	if value == "" {
		return "chatgpt2api"
	}
	return value
}

func (s *Store) BrandTopLeftLogoURL() string {
	value := strings.TrimSpace(fmt.Sprint(s.settingValue("brand_top_left_logo_url", "/logo-mark.svg")))
	if value == "" {
		return "/logo-mark.svg"
	}
	return value
}

func (s *Store) BrandSiteLogoURL() string {
	value := strings.TrimSpace(fmt.Sprint(s.settingValue("brand_site_logo_url", "/logo-mark.svg")))
	if value == "" {
		return "/logo-mark.svg"
	}
	return value
}

func (s *Store) Proxy() string {
	return strings.TrimSpace(fmt.Sprint(s.settingValue("proxy", "")))
}

func (s *Store) ImageObjectStorage() objectstore.Config {
	accountEndpoint := strings.TrimSpace(fmt.Sprint(s.settingValue("image_r2_endpoint", "")))
	return objectstore.Config{
		Enabled:         util.ToBool(s.settingValue("image_r2_enabled", false)),
		Endpoint:        accountEndpoint,
		Region:          strings.TrimSpace(fmt.Sprint(s.settingValue("image_r2_region", "auto"))),
		Bucket:          strings.TrimSpace(fmt.Sprint(s.settingValue("image_r2_bucket", ""))),
		AccessKeyID:     strings.TrimSpace(fmt.Sprint(s.settingValue("image_r2_access_key_id", ""))),
		SecretAccessKey: strings.TrimSpace(fmt.Sprint(s.settingValue("image_r2_secret_access_key", ""))),
		PublicBaseURL:   strings.TrimRight(strings.TrimSpace(fmt.Sprint(s.settingValue("image_r2_public_base_url", ""))), "/"),
		Prefix:          strings.Trim(strings.TrimSpace(fmt.Sprint(s.settingValue("image_r2_prefix", "images"))), "/"),
	}.Normalize()
}

func (s *Store) UpdateProxyURL() string {
	if value := strings.TrimSpace(os.Getenv("CHATGPT2API_UPDATE_PROXY_URL")); value != "" {
		return value
	}
	return s.Proxy()
}

func (s *Store) UpdateRepo() string {
	return normalizeUpdateRepo(s.settingValue("update_repo", "ZyphrZero/chatgpt2api"))
}

func (s *Store) UpdateGitHubToken() string {
	return strings.TrimSpace(fmt.Sprint(s.settingValue("update_github_token", "")))
}

func (s *Store) LogLevels() []string {
	raw := s.settingValue("log_levels", "")
	var parts []string
	switch v := raw.(type) {
	case []string:
		parts = v
	case []any:
		for _, item := range v {
			parts = append(parts, fmt.Sprint(item))
		}
	default:
		parts = strings.Split(fmt.Sprint(raw), ",")
	}
	allowed := map[string]struct{}{"debug": {}, "info": {}, "warning": {}, "error": {}}
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		level := strings.ToLower(strings.TrimSpace(part))
		if _, ok := allowed[level]; ok {
			out = append(out, level)
		}
	}
	return out
}

func (s *Store) LinuxDoOAuth() LinuxDoOAuthConfig {
	s.mu.RLock()
	data := util.CopyMap(s.data)
	s.mu.RUnlock()
	return s.linuxDoOAuthFromData(data)
}

func (s *Store) linuxDoOAuthFromData(data map[string]any) LinuxDoOAuthConfig {
	redirectURL := strings.TrimSpace(fmt.Sprint(s.settingValueFromData(data, "linuxdo_redirect_url", "")))
	baseURL := strings.TrimRight(strings.TrimSpace(fmt.Sprint(s.settingValueFromData(data, "base_url", ""))), "/")
	if redirectURL == "" && baseURL != "" {
		redirectURL = baseURL + "/auth/linuxdo/oauth/callback"
	}
	return LinuxDoOAuthConfig{
		Enabled:              util.ToBool(s.settingValueFromData(data, "linuxdo_enabled", false)),
		ClientID:             strings.TrimSpace(fmt.Sprint(s.settingValueFromData(data, "linuxdo_client_id", ""))),
		ClientSecret:         strings.TrimSpace(fmt.Sprint(s.settingValueFromData(data, "linuxdo_client_secret", ""))),
		AuthorizeURL:         envString("CHATGPT2API_LINUXDO_AUTHORIZE_URL", "https://connect.linux.do/oauth2/authorize"),
		TokenURL:             envString("CHATGPT2API_LINUXDO_TOKEN_URL", "https://connect.linux.do/oauth2/token"),
		UserInfoURL:          envString("CHATGPT2API_LINUXDO_USERINFO_URL", "https://connect.linux.do/api/user"),
		Scopes:               envString("CHATGPT2API_LINUXDO_SCOPES", "user"),
		RedirectURL:          redirectURL,
		FrontendRedirectURL:  strings.TrimSpace(fmt.Sprint(s.settingValueFromData(data, "linuxdo_frontend_redirect_url", "/auth/linuxdo/callback"))),
		TokenAuthMethod:      strings.ToLower(envString("CHATGPT2API_LINUXDO_TOKEN_AUTH_METHOD", "client_secret_post")),
		UsePKCE:              envBool("CHATGPT2API_LINUXDO_USE_PKCE", false),
		UserInfoEmailPath:    envString("CHATGPT2API_LINUXDO_USERINFO_EMAIL_PATH", ""),
		UserInfoIDPath:       envString("CHATGPT2API_LINUXDO_USERINFO_ID_PATH", ""),
		UserInfoUsernamePath: envString("CHATGPT2API_LINUXDO_USERINFO_USERNAME_PATH", ""),
	}
}

func (s *Store) yiPayFromData(data map[string]any) YiPayConfig {
	return YiPayConfig{
		Enabled:   util.ToBool(s.settingValueFromData(data, "yipay_enabled", false)),
		PID:       strings.TrimSpace(fmt.Sprint(s.settingValueFromData(data, "yipay_pid", ""))),
		Key:       strings.TrimSpace(fmt.Sprint(s.settingValueFromData(data, "yipay_key", ""))),
		SubmitURL: strings.TrimSpace(fmt.Sprint(s.settingValueFromData(data, "yipay_submit_url", ""))),
		NotifyURL: strings.TrimSpace(fmt.Sprint(s.settingValueFromData(data, "yipay_notify_url", ""))),
		ReturnURL: strings.TrimSpace(fmt.Sprint(s.settingValueFromData(data, "yipay_return_url", ""))),
		SiteName:  strings.TrimSpace(fmt.Sprint(s.settingValueFromData(data, "yipay_site_name", "chatgpt2api"))),
	}
}

func (s *Store) payPalFromData(data map[string]any) PayPalConfig {
	return PayPalConfig{
		Enabled:     util.ToBool(s.settingValueFromData(data, "paypal_enabled", false)),
		CheckoutURL: strings.TrimSpace(fmt.Sprint(s.settingValueFromData(data, "paypal_checkout_url", ""))),
	}
}

func (s *Store) usdtFromData(data map[string]any) USDTConfig {
	network := strings.TrimSpace(fmt.Sprint(s.settingValueFromData(data, "usdt_network", "TRC20")))
	if network == "" {
		network = "TRC20"
	}
	return USDTConfig{
		Enabled:    util.ToBool(s.settingValueFromData(data, "usdt_enabled", false)),
		Network:    network,
		Address:    strings.TrimSpace(fmt.Sprint(s.settingValueFromData(data, "usdt_address", ""))),
		PaymentURL: strings.TrimSpace(fmt.Sprint(s.settingValueFromData(data, "usdt_payment_url", ""))),
	}
}

func (c LinuxDoOAuthConfig) Ready() bool {
	if !c.Enabled {
		return false
	}
	if c.ClientID == "" || c.AuthorizeURL == "" || c.TokenURL == "" || c.UserInfoURL == "" || c.RedirectURL == "" {
		return false
	}
	switch c.TokenAuthMethod {
	case "", "client_secret_post", "client_secret_basic":
		return c.ClientSecret != ""
	case "none":
		return c.UsePKCE
	default:
		return false
	}
}

func (c YiPayConfig) Ready() bool {
	if !c.Enabled {
		return false
	}
	return c.PID != "" && c.Key != "" && c.SubmitURL != ""
}

func (s *Store) ImagesDir() string {
	path := filepath.Join(s.DataDir, "images")
	_ = os.MkdirAll(path, 0o755)
	return path
}

func (s *Store) ImageThumbnailsDir() string {
	path := filepath.Join(s.DataDir, "image_thumbnails")
	_ = os.MkdirAll(path, 0o755)
	return path
}

func (s *Store) ImageMetadataDir() string {
	path := filepath.Join(s.DataDir, "image_metadata")
	_ = os.MkdirAll(path, 0o755)
	return path
}

func (s *Store) LoginPageImagesDir() string {
	path := filepath.Join(s.DataDir, "login_page_images")
	_ = os.MkdirAll(path, 0o755)
	return path
}

func (s *Store) LoginPageImageURL() string {
	return strings.TrimSpace(fmt.Sprint(s.settingValue("login_page_image_url", "")))
}

func (s *Store) LoginPageImageMode() string {
	return normalizeLoginPageImageMode(s.settingValue("login_page_image_mode", "contain"))
}

func (s *Store) LoginPageImageZoom() float64 {
	return clampFloat(floatSetting(s.settingValue("login_page_image_zoom", 1), 1), 1, 3)
}

func (s *Store) LoginPageImagePositionX() float64 {
	return clampFloat(floatSetting(s.settingValue("login_page_image_position_x", 50), 50), 0, 100)
}

func (s *Store) LoginPageImagePositionY() float64 {
	return clampFloat(floatSetting(s.settingValue("login_page_image_position_y", 50), 50), 0, 100)
}

func (s *Store) Get() map[string]any {
	s.mu.RLock()
	data := util.CopyMap(s.data)
	s.mu.RUnlock()
	data["refresh_account_interval_minute"] = s.RefreshAccountIntervalMinute()
	data["image_concurrent_limit"] = s.ImageConcurrentLimit()
	data["image_single_count_limit"] = s.ImageSingleCountLimit()
	data["image_task_timeout_seconds"] = s.ImageTaskTimeoutSeconds()
	data["user_default_concurrent_limit"] = s.UserDefaultConcurrentLimit()
	data["user_default_rpm_limit"] = s.UserDefaultRPMLimit()
	data["image_retention_days"] = s.ImageRetentionDays()
	data["log_retention_days"] = s.LogRetentionDays()
	data["auto_remove_invalid_accounts"] = s.AutoRemoveInvalidAccounts()
	data["auto_remove_rate_limited_accounts"] = s.AutoRemoveRateLimitedAccounts()
	data["log_levels"] = s.LogLevels()
	data["proxy"] = s.Proxy()
	data["base_url"] = s.BaseURL()
	data["brand_top_left_name"] = s.BrandTopLeftName()
	data["brand_site_name"] = s.BrandSiteName()
	data["brand_top_left_logo_url"] = s.BrandTopLeftLogoURL()
	data["brand_site_logo_url"] = s.BrandSiteLogoURL()
	data["registration_enabled"] = s.RegistrationEnabled()
	data["registration_allowed_email_domains"] = strings.Join(s.RegistrationAllowedEmailDomains(), ",")
	data["registration_bonus_image_times"] = s.RegistrationBonusImageTimes()
	data["cf_turnstile_site_key"] = s.CFTurnstileSiteKey()
	data["cf_turnstile_secret_key_configured"] = s.CFTurnstileSecretKey() != ""
	emailSMTP := s.EmailSMTP()
	data["email_smtp_enabled"] = emailSMTP.Enabled
	data["email_smtp_host"] = emailSMTP.Host
	data["email_smtp_port"] = emailSMTP.Port
	data["email_smtp_use_ssl"] = emailSMTP.UseSSL
	data["email_smtp_username"] = emailSMTP.Username
	data["email_smtp_from_email"] = emailSMTP.FromEmail
	data["email_smtp_from_name"] = emailSMTP.FromName
	data["email_smtp_auth_code_configured"] = strings.TrimSpace(emailSMTP.Password) != ""
	data["image_price_cents"] = s.ImagePriceCents()
	data["image_price_1k_cents"] = s.ImagePrice1KCents()
	data["image_price_2k_cents"] = s.ImagePrice2KCents()
	data["image_price_4k_cents"] = s.ImagePrice4KCents()
	data["agency_tier_basic_cents"] = s.AgencyTierBasicCents()
	data["agency_tier_pro_cents"] = s.AgencyTierProCents()
	data["agency_tier_premium_cents"] = s.AgencyTierPremiumCents()
	data["agency_tier_basic_commission_bp"] = s.AgencyTierBasicCommissionBP()
	data["agency_tier_pro_commission_bp"] = s.AgencyTierProCommissionBP()
	data["agency_tier_premium_commission_bp"] = s.AgencyTierPremiumCommissionBP()
	data["agency_tier_basic_discount_bp"] = s.AgencyTierBasicDiscountBP()
	data["agency_tier_pro_discount_bp"] = s.AgencyTierProDiscountBP()
	data["agency_tier_premium_discount_bp"] = s.AgencyTierPremiumDiscountBP()
	yipay := s.YiPay()
	data["yipay_enabled"] = yipay.Enabled
	data["yipay_pid"] = yipay.PID
	data["yipay_submit_url"] = yipay.SubmitURL
	data["yipay_notify_url"] = yipay.NotifyURL
	data["yipay_return_url"] = yipay.ReturnURL
	data["yipay_site_name"] = yipay.SiteName
	data["yipay_key_configured"] = yipay.Key != ""
	paypal := s.PayPal()
	data["paypal_enabled"] = paypal.Enabled
	data["paypal_checkout_url"] = paypal.CheckoutURL
	usdt := s.USDT()
	data["usdt_enabled"] = usdt.Enabled
	data["usdt_network"] = usdt.Network
	data["usdt_address"] = usdt.Address
	data["usdt_payment_url"] = usdt.PaymentURL
	linuxdo := s.LinuxDoOAuth()
	data["linuxdo_enabled"] = linuxdo.Enabled
	data["linuxdo_client_id"] = linuxdo.ClientID
	data["linuxdo_client_secret_configured"] = linuxdo.ClientSecret != ""
	data["linuxdo_redirect_url"] = linuxdo.RedirectURL
	data["linuxdo_frontend_redirect_url"] = linuxdo.FrontendRedirectURL
	data["update_repo"] = s.UpdateRepo()
	data["update_github_token_configured"] = s.UpdateGitHubToken() != ""
	data["login_page_image_url"] = s.LoginPageImageURL()
	data["login_page_image_mode"] = s.LoginPageImageMode()
	data["login_page_image_zoom"] = s.LoginPageImageZoom()
	data["login_page_image_position_x"] = s.LoginPageImagePositionX()
	data["login_page_image_position_y"] = s.LoginPageImagePositionY()
	imageR2 := s.ImageObjectStorage()
	data["image_r2_enabled"] = imageR2.Enabled
	data["image_r2_endpoint"] = imageR2.Endpoint
	data["image_r2_bucket"] = imageR2.Bucket
	data["image_r2_region"] = imageR2.Region
	data["image_r2_access_key_id"] = imageR2.AccessKeyID
	data["image_r2_secret_access_key_configured"] = imageR2.SecretAccessKey != ""
	data["image_r2_public_base_url"] = imageR2.PublicBaseURL
	data["image_r2_prefix"] = imageR2.Prefix
	delete(data, "linuxdo_client_secret")
	delete(data, "update_github_token")
	delete(data, "cf_turnstile_secret_key")
	delete(data, "email_smtp_auth_code")
	delete(data, "yipay_key")
	delete(data, "image_r2_secret_access_key")
	return data
}

func (s *Store) Update(data map[string]any) (map[string]any, error) {
	s.mu.Lock()
	next := util.CopyMap(s.data)
	for key, value := range data {
		if key == "linuxdo_client_secret_configured" {
			continue
		}
		if key == "update_github_token_configured" {
			continue
		}
		if key == "cf_turnstile_secret_key_configured" {
			continue
		}
		if key == "email_smtp_auth_code_configured" {
			continue
		}
		if key == "yipay_key_configured" {
			continue
		}
		if key == "image_r2_secret_access_key_configured" {
			continue
		}
		if key == "linuxdo_client_secret" && strings.TrimSpace(fmt.Sprint(value)) == "" {
			continue
		}
		if key == "update_github_token" && strings.TrimSpace(fmt.Sprint(value)) == "" {
			continue
		}
		if key == "cf_turnstile_secret_key" && strings.TrimSpace(fmt.Sprint(value)) == "" {
			continue
		}
		if key == "email_smtp_auth_code" && strings.TrimSpace(fmt.Sprint(value)) == "" {
			continue
		}
		if key == "yipay_key" && strings.TrimSpace(fmt.Sprint(value)) == "" {
			continue
		}
		if key == "image_r2_secret_access_key" && strings.TrimSpace(fmt.Sprint(value)) == "" {
			continue
		}
		next[key] = value
	}
	if value, ok := next["login_page_image_mode"]; ok {
		next["login_page_image_mode"] = normalizeLoginPageImageMode(value)
	}
	if value, ok := next["image_task_timeout_seconds"]; ok {
		next["image_task_timeout_seconds"] = normalizeImageTaskTimeoutSeconds(value)
	}
	next["update_repo"] = normalizeUpdateRepo(util.ValueOr(next["update_repo"], "ZyphrZero/chatgpt2api"))
	if err := s.validateSettingsUpdateLocked(next); err != nil {
		s.mu.Unlock()
		return nil, err
	}
	s.data = next
	err := s.saveLocked()
	s.mu.Unlock()
	if err != nil {
		return nil, err
	}
	return s.Get(), nil
}

func (s *Store) CleanupOldImages() int {
	cutoff := time.Now().Add(-time.Duration(s.ImageRetentionDays()) * 24 * time.Hour)
	removed := 0
	for _, dir := range []string{s.ImagesDir(), s.ImageThumbnailsDir(), s.ImageMetadataDir()} {
		_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			info, statErr := d.Info()
			if statErr == nil && info.ModTime().Before(cutoff) {
				if os.Remove(path) == nil {
					removed++
				}
			}
			return nil
		})
		removeEmptyDirs(dir)
	}
	return removed
}

func (s *Store) StorageBackend() (storage.Backend, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.storageBackend != nil {
		return s.storageBackend, nil
	}
	backend, err := storage.NewBackendFromEnv(s.DataDir)
	if err != nil {
		return nil, err
	}
	s.storageBackend = backend
	return backend, nil
}

func (s *Store) settingValue(key string, fallback any) any {
	envKey := settingEnvKeys[key]
	if value, ok := os.LookupEnv(envKey); ok {
		return value
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if value, ok := s.data[key]; ok {
		return value
	}
	return fallback
}

func (s *Store) settingValueFromData(data map[string]any, key string, fallback any) any {
	envKey := settingEnvKeys[key]
	if envKey != "" {
		if value, ok := os.LookupEnv(envKey); ok {
			if _, external := s.externalEnvKeys[envKey]; external {
				return value
			}
		}
	}
	if data != nil {
		if value, ok := data[key]; ok {
			return value
		}
	}
	if envKey != "" {
		if value, ok := os.LookupEnv(envKey); ok {
			return value
		}
	}
	return fallback
}

func (s *Store) validateSettingsUpdateLocked(data map[string]any) error {
	if err := validateUpdateRepo(util.Clean(util.ValueOr(data["update_repo"], "ZyphrZero/chatgpt2api"))); err != nil {
		return err
	}
	for key, value := range map[string]int{
		"image_price_cents":    intSetting(util.ValueOr(data["image_price_cents"], 8), 8),
		"image_price_1k_cents": intSetting(util.ValueOr(data["image_price_1k_cents"], util.ValueOr(data["image_price_cents"], 8)), 8),
		"image_price_2k_cents": intSetting(util.ValueOr(data["image_price_2k_cents"], 16), 16),
		"image_price_4k_cents": intSetting(util.ValueOr(data["image_price_4k_cents"], 32), 32),
	} {
		if value < 1 {
			return fmt.Errorf("%s must be >= 1", key)
		}
	}
	for key, value := range map[string]int{
		"agency_tier_basic_cents":           intSetting(util.ValueOr(data["agency_tier_basic_cents"], 19900), 19900),
		"agency_tier_pro_cents":             intSetting(util.ValueOr(data["agency_tier_pro_cents"], 49900), 49900),
		"agency_tier_premium_cents":         intSetting(util.ValueOr(data["agency_tier_premium_cents"], 99900), 99900),
		"agency_tier_basic_commission_bp":   clampAgencyBasisPoint(intSetting(util.ValueOr(data["agency_tier_basic_commission_bp"], 3000), 3000)),
		"agency_tier_pro_commission_bp":     clampAgencyBasisPoint(intSetting(util.ValueOr(data["agency_tier_pro_commission_bp"], 4500), 4500)),
		"agency_tier_premium_commission_bp": clampAgencyBasisPoint(intSetting(util.ValueOr(data["agency_tier_premium_commission_bp"], 6000), 6000)),
		"agency_tier_basic_discount_bp":     clampAgencyBasisPoint(intSetting(util.ValueOr(data["agency_tier_basic_discount_bp"], 500), 500)),
		"agency_tier_pro_discount_bp":       clampAgencyBasisPoint(intSetting(util.ValueOr(data["agency_tier_pro_discount_bp"], 1000), 1000)),
		"agency_tier_premium_discount_bp":   clampAgencyBasisPoint(intSetting(util.ValueOr(data["agency_tier_premium_discount_bp"], 1500), 1500)),
	} {
		if value < 0 {
			return fmt.Errorf("%s must be >= 0", key)
		}
	}
	registrationBonusImageTimes := intSetting(util.ValueOr(data["registration_bonus_image_times"], 20), 20)
	if registrationBonusImageTimes < 0 {
		return errors.New("registration_bonus_image_times must be >= 0")
	}
	yipay := s.yiPayFromData(data)
	if yipay.Enabled {
		if yipay.PID == "" {
			return errors.New("YiPay PID is required when enabled")
		}
		if yipay.Key == "" {
			return errors.New("YiPay KEY is required when enabled")
		}
		if yipay.SubmitURL == "" {
			return errors.New("YiPay Submit URL is required when enabled")
		}
		if err := validateAbsoluteHTTPURL(yipay.SubmitURL); err != nil {
			return errors.New("YiPay Submit URL must be an absolute http(s) URL")
		}
		if strings.TrimSpace(yipay.NotifyURL) != "" {
			if err := validateAbsoluteHTTPURL(yipay.NotifyURL); err != nil {
				return errors.New("YiPay Notify URL must be an absolute http(s) URL")
			}
		}
		if strings.TrimSpace(yipay.ReturnURL) != "" {
			if err := validateAbsoluteHTTPURL(yipay.ReturnURL); err != nil {
				return errors.New("YiPay Return URL must be an absolute http(s) URL")
			}
		}
	}
	paypal := s.payPalFromData(data)
	if paypal.Enabled {
		if strings.TrimSpace(paypal.CheckoutURL) == "" {
			return errors.New("PayPal Checkout URL is required when enabled")
		}
		if err := validateAbsoluteHTTPURL(paypal.CheckoutURL); err != nil {
			return errors.New("PayPal Checkout URL must be an absolute http(s) URL")
		}
	}
	usdt := s.usdtFromData(data)
	if usdt.Enabled {
		if strings.TrimSpace(usdt.Address) == "" {
			return errors.New("USDT Address is required when enabled")
		}
		if strings.TrimSpace(usdt.PaymentURL) != "" {
			if err := validateAbsoluteHTTPURL(usdt.PaymentURL); err != nil {
				return errors.New("USDT Payment URL must be an absolute http(s) URL")
			}
		}
	}
	imageR2 := objectstore.Config{
		Enabled:         util.ToBool(util.ValueOr(data["image_r2_enabled"], false)),
		Endpoint:        strings.TrimSpace(fmt.Sprint(util.ValueOr(data["image_r2_endpoint"], ""))),
		Region:          strings.TrimSpace(fmt.Sprint(util.ValueOr(data["image_r2_region"], "auto"))),
		Bucket:          strings.TrimSpace(fmt.Sprint(util.ValueOr(data["image_r2_bucket"], ""))),
		AccessKeyID:     strings.TrimSpace(fmt.Sprint(util.ValueOr(data["image_r2_access_key_id"], ""))),
		SecretAccessKey: strings.TrimSpace(fmt.Sprint(util.ValueOr(data["image_r2_secret_access_key"], ""))),
		PublicBaseURL:   strings.TrimSpace(fmt.Sprint(util.ValueOr(data["image_r2_public_base_url"], ""))),
		Prefix:          strings.TrimSpace(fmt.Sprint(util.ValueOr(data["image_r2_prefix"], "images"))),
	}.Normalize()
	if imageR2.Enabled {
		if imageR2.Endpoint == "" {
			return errors.New("R2 Endpoint is required when enabled")
		}
		if err := validateAbsoluteHTTPURL(imageR2.Endpoint); err != nil {
			return errors.New("R2 Endpoint must be an absolute http(s) URL")
		}
		if imageR2.Bucket == "" {
			return errors.New("R2 Bucket is required when enabled")
		}
		if imageR2.AccessKeyID == "" {
			return errors.New("R2 Access Key ID is required when enabled")
		}
		if imageR2.SecretAccessKey == "" {
			return errors.New("R2 Secret Access Key is required when enabled")
		}
		if imageR2.PublicBaseURL == "" {
			return errors.New("R2 Public Base URL is required when enabled")
		}
		if err := validateAbsoluteHTTPURL(imageR2.PublicBaseURL); err != nil {
			return errors.New("R2 Public Base URL must be an absolute http(s) URL")
		}
	}

	linuxdo := s.linuxDoOAuthFromData(data)
	if !linuxdo.Enabled {
		return nil
	}
	if linuxdo.ClientID == "" {
		return errors.New("Linuxdo Client ID is required when enabled")
	}
	if linuxdo.RedirectURL == "" {
		return errors.New("Linuxdo Redirect URL is required when enabled")
	}
	if linuxdo.FrontendRedirectURL == "" {
		return errors.New("Linuxdo Frontend Redirect URL is required when enabled")
	}
	if err := validateAbsoluteHTTPURL(linuxdo.RedirectURL); err != nil {
		return errors.New("Linuxdo Redirect URL must be an absolute http(s) URL")
	}
	if err := validateFrontendRedirectURL(linuxdo.FrontendRedirectURL); err != nil {
		return errors.New("Linuxdo Frontend Redirect URL must be an absolute http(s) URL or a relative path")
	}
	switch linuxdo.TokenAuthMethod {
	case "", "client_secret_post", "client_secret_basic":
		if linuxdo.ClientSecret == "" {
			return errors.New("Linuxdo Client Secret is required when enabled")
		}
	case "none":
		if !linuxdo.UsePKCE {
			return errors.New("Linuxdo PKCE must be enabled when token auth method is none")
		}
	default:
		return errors.New("Linuxdo token auth method must be one of client_secret_post, client_secret_basic, none")
	}
	return nil
}

func normalizeUpdateRepo(value any) string {
	repo := strings.Trim(strings.TrimSpace(fmt.Sprint(value)), "/")
	if repo == "" {
		return "ZyphrZero/chatgpt2api"
	}
	return repo
}

func validateUpdateRepo(value string) error {
	if !regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`).MatchString(value) {
		return errors.New("Update repository must use owner/repo format")
	}
	return nil
}

func validateAbsoluteHTTPURL(value string) error {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil {
		return err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return errors.New("scheme must be http or https")
	}
	if parsed.Host == "" {
		return errors.New("host is required")
	}
	return nil
}

func validateFrontendRedirectURL(value string) error {
	value = strings.TrimSpace(value)
	if strings.ContainsAny(value, "\r\n") {
		return errors.New("newlines are not allowed")
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return err
	}
	if parsed.Scheme != "" {
		if parsed.Scheme != "http" && parsed.Scheme != "https" {
			return errors.New("scheme must be http or https")
		}
		if parsed.Host == "" {
			return errors.New("host is required")
		}
		return nil
	}
	if !strings.HasPrefix(value, "/") || strings.HasPrefix(value, "//") {
		return errors.New("relative path must start with one slash")
	}
	return nil
}

func (s *Store) saveLocked() error {
	updates := map[string]string{}
	keys := make([]string, 0, len(settingEnvKeys))
	for key := range settingEnvKeys {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if value, ok := s.data[key]; ok {
			updates[settingEnvKeys[key]] = stringifyEnvValue(value)
		}
	}
	if err := writeEnvUpdates(s.EnvFile, updates); err != nil {
		return err
	}
	for key, value := range updates {
		if _, external := s.externalEnvKeys[key]; !external {
			_ = os.Setenv(key, value)
		}
	}
	return nil
}

func (s *Store) loadEnvFile() {
	for key, value := range readEnvObject(s.EnvFile) {
		if _, ok := os.LookupEnv(key); !ok {
			_ = os.Setenv(key, value)
		}
	}
}

func settingsFromEnvValues(values map[string]string) map[string]any {
	settings := map[string]any{}
	for settingKey, envKey := range settingEnvKeys {
		if value, ok := values[envKey]; ok {
			settings[settingKey] = value
		}
	}
	return settings
}

func intSetting(value any, fallback int) int {
	switch v := value.(type) {
	case int:
		return v
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(v))
		if err == nil {
			return n
		}
	}
	return fallback
}

func floatSetting(value any, fallback float64) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case string:
		n, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		if err == nil {
			return n
		}
	}
	return fallback
}

func normalizeLoginPageImageMode(value any) string {
	mode := strings.ToLower(strings.TrimSpace(fmt.Sprint(value)))
	switch mode {
	case "cover", "contain", "fill":
		return mode
	default:
		return "contain"
	}
}

func normalizeImageTaskTimeoutSeconds(value any) int {
	seconds := intSetting(value, defaultImageTaskTimeoutSeconds)
	if seconds < minImageTaskTimeoutSeconds {
		return minImageTaskTimeoutSeconds
	}
	if seconds > maxImageTaskTimeoutSeconds {
		return maxImageTaskTimeoutSeconds
	}
	return seconds
}

func clampFloat(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func envString(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return strings.TrimSpace(value)
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	if value, ok := os.LookupEnv(key); ok {
		value = strings.ToLower(strings.TrimSpace(value))
		return value == "1" || value == "true" || value == "yes" || value == "on"
	}
	return fallback
}

func readEnvObject(path string) map[string]string {
	data, err := os.ReadFile(path)
	if err != nil {
		if info, statErr := os.Stat(path); statErr == nil && info.IsDir() {
			fmt.Fprintf(os.Stderr, "Warning: .env at %q is a directory, ignoring it.\n", path)
		}
		return map[string]string{}
	}
	result := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		key, value, ok := parseEnvAssignment(line)
		if ok {
			result[key] = value
		}
	}
	return result
}

func parseEnvAssignment(line string) (string, string, bool) {
	stripped := strings.TrimSpace(line)
	if stripped == "" || strings.HasPrefix(stripped, "#") {
		return "", "", false
	}
	stripped = strings.TrimSpace(strings.TrimPrefix(stripped, "export "))
	key, value, ok := strings.Cut(stripped, "=")
	if !ok {
		return "", "", false
	}
	key = strings.TrimSpace(key)
	if !envKeyRE.MatchString(key) {
		return "", "", false
	}
	return key, unquoteEnvValue(value), true
}

func unquoteEnvValue(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 2 && value[0] == value[len(value)-1] && (value[0] == '"' || value[0] == '\'') {
		inner := value[1 : len(value)-1]
		if value[0] == '"' {
			inner = strings.ReplaceAll(inner, `\n`, "\n")
			inner = strings.ReplaceAll(inner, `\r`, "\r")
			inner = strings.ReplaceAll(inner, `\t`, "\t")
			inner = strings.ReplaceAll(inner, `\"`, `"`)
			inner = strings.ReplaceAll(inner, `\\`, `\`)
		}
		return inner
	}
	for index, char := range value {
		if char == '#' && (index == 0 || value[index-1] == ' ' || value[index-1] == '\t') {
			return strings.TrimRight(value[:index], " \t")
		}
	}
	return value
}

func stringifyEnvValue(value any) string {
	switch v := value.(type) {
	case bool:
		if v {
			return "true"
		}
		return "false"
	case []string:
		return strings.Join(v, ",")
	case []any:
		items := make([]string, 0, len(v))
		for _, item := range v {
			if s := strings.TrimSpace(fmt.Sprint(item)); s != "" {
				items = append(items, s)
			}
		}
		return strings.Join(items, ",")
	default:
		return strings.TrimSpace(fmt.Sprint(util.ValueOr(value, "")))
	}
}

func writeEnvUpdates(path string, updates map[string]string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var lines []string
	if data, err := os.ReadFile(path); err == nil {
		lines = strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
		if len(lines) > 0 && lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}
	}
	pending := map[string]string{}
	for key, value := range updates {
		pending[key] = value
	}
	next := make([]string, 0, len(lines)+len(updates)+1)
	for _, line := range lines {
		key, _, ok := parseEnvAssignment(line)
		if ok {
			if value, exists := pending[key]; exists {
				next = append(next, formatEnvAssignment(key, value))
				delete(pending, key)
				continue
			}
		}
		next = append(next, line)
	}
	if len(pending) > 0 {
		if len(next) > 0 && strings.TrimSpace(next[len(next)-1]) != "" {
			next = append(next, "")
		}
		keys := make([]string, 0, len(pending))
		for key := range pending {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			next = append(next, formatEnvAssignment(key, pending[key]))
		}
	}
	return os.WriteFile(path, []byte(strings.TrimRight(strings.Join(next, "\n"), "\n")+"\n"), 0o644)
}

func formatEnvAssignment(key, value string) string {
	return key + "=" + formatEnvValue(value)
}

func formatEnvValue(value string) string {
	if value == "" {
		return ""
	}
	if regexp.MustCompile(`^[A-Za-z0-9_./:@%+\-,]*$`).MatchString(value) {
		return value
	}
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	value = strings.ReplaceAll(value, "\n", `\n`)
	return `"` + value + `"`
}

func removeEmptyDirs(root string) {
	var dirs []string
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err == nil && d.IsDir() && path != root {
			dirs = append(dirs, path)
		}
		return nil
	})
	sort.Slice(dirs, func(i, j int) bool { return len(dirs[i]) > len(dirs[j]) })
	for _, dir := range dirs {
		_ = os.Remove(dir)
	}
}
