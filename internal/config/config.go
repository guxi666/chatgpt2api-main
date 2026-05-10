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

	"chatgpt2api/internal/storage"
	"chatgpt2api/internal/util"
)

var settingEnvKeys = map[string]string{
	"auth-key":                          "CHATGPT2API_AUTH_KEY",
	"app_title":                         "CHATGPT2API_APP_TITLE",
	"project_name":                      "CHATGPT2API_PROJECT_NAME",
	"app_logo_url":                      "CHATGPT2API_APP_LOGO_URL",
	"site_icon_url":                     "CHATGPT2API_SITE_ICON_URL",
	"base_url":                          "CHATGPT2API_BASE_URL",
	"proxy":                             "CHATGPT2API_PROXY",
	"refresh_account_interval_minute":   "CHATGPT2API_REFRESH_ACCOUNT_INTERVAL_MINUTE",
	"image_concurrent_limit":            "CHATGPT2API_IMAGE_CONCURRENT_LIMIT",
	"user_default_concurrent_limit":     "CHATGPT2API_USER_DEFAULT_CONCURRENT_LIMIT",
	"user_default_rpm_limit":            "CHATGPT2API_USER_DEFAULT_RPM_LIMIT",
	"image_retention_days":              "CHATGPT2API_IMAGE_RETENTION_DAYS",
	"auto_remove_invalid_accounts":      "CHATGPT2API_AUTO_REMOVE_INVALID_ACCOUNTS",
	"auto_remove_rate_limited_accounts": "CHATGPT2API_AUTO_REMOVE_RATE_LIMITED_ACCOUNTS",
	"log_levels":                        "CHATGPT2API_LOG_LEVELS",
	"linuxdo_enabled":                   "CHATGPT2API_LINUXDO_ENABLED",
	"linuxdo_client_id":                 "CHATGPT2API_LINUXDO_CLIENT_ID",
	"linuxdo_client_secret":             "CHATGPT2API_LINUXDO_CLIENT_SECRET",
	"linuxdo_redirect_url":              "CHATGPT2API_LINUXDO_REDIRECT_URL",
	"linuxdo_frontend_redirect_url":     "CHATGPT2API_LINUXDO_FRONTEND_REDIRECT_URL",
	"login_page_image_url":              "CHATGPT2API_LOGIN_PAGE_IMAGE_URL",
	"login_page_image_mode":             "CHATGPT2API_LOGIN_PAGE_IMAGE_MODE",
	"login_page_image_zoom":             "CHATGPT2API_LOGIN_PAGE_IMAGE_ZOOM",
	"login_page_image_position_x":       "CHATGPT2API_LOGIN_PAGE_IMAGE_POSITION_X",
	"login_page_image_position_y":       "CHATGPT2API_LOGIN_PAGE_IMAGE_POSITION_Y",
	"email_allowed_domains":             "CHATGPT2API_EMAIL_ALLOWED_DOMAINS",
	"email_smtp_enabled":                "CHATGPT2API_EMAIL_SMTP_ENABLED",
	"email_smtp_host":                   "CHATGPT2API_EMAIL_SMTP_HOST",
	"email_smtp_port":                   "CHATGPT2API_EMAIL_SMTP_PORT",
	"email_smtp_use_ssl":                "CHATGPT2API_EMAIL_SMTP_USE_SSL",
	"email_smtp_username":               "CHATGPT2API_EMAIL_SMTP_USERNAME",
	"email_smtp_auth_code":              "CHATGPT2API_EMAIL_SMTP_AUTH_CODE",
	"email_smtp_from_email":             "CHATGPT2API_EMAIL_SMTP_FROM_EMAIL",
	"email_smtp_from_name":              "CHATGPT2API_EMAIL_SMTP_FROM_NAME",
	"image_price_cents":                 "CHATGPT2API_IMAGE_PRICE_CENTS",
	"yipay_enabled":                     "CHATGPT2API_YIPAY_ENABLED",
	"yipay_pid":                         "CHATGPT2API_YIPAY_PID",
	"yipay_key":                         "CHATGPT2API_YIPAY_KEY",
	"yipay_submit_url":                  "CHATGPT2API_YIPAY_SUBMIT_URL",
	"yipay_notify_url":                  "CHATGPT2API_YIPAY_NOTIFY_URL",
	"yipay_return_url":                  "CHATGPT2API_YIPAY_RETURN_URL",
	"yipay_site_name":                   "CHATGPT2API_YIPAY_SITE_NAME",
	"paypal_enabled":                    "CHATGPT2API_PAYPAL_ENABLED",
	"paypal_checkout_url":               "CHATGPT2API_PAYPAL_CHECKOUT_URL",
	"usdt_enabled":                      "CHATGPT2API_USDT_ENABLED",
	"usdt_network":                      "CHATGPT2API_USDT_NETWORK",
	"usdt_address":                      "CHATGPT2API_USDT_ADDRESS",
	"usdt_payment_url":                  "CHATGPT2API_USDT_PAYMENT_URL",
}

var envKeyRE = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

const imageCleanupMinInterval = 10 * time.Minute

type Store struct {
	mu                 sync.RWMutex
	RootDir            string
	DataDir            string
	EnvFile            string
	data               map[string]any
	externalEnvKeys    map[string]struct{}
	storageBackend     storage.Backend
	lastImageCleanupAt time.Time
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

type EmailSMTPConfig struct {
	Enabled   bool
	Host      string
	Port      int
	UseSSL    bool
	Username  string
	AuthCode  string
	FromEmail string
	FromName  string
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
	if s.AuthKey() == "" {
		return nil, errors.New("auth-key 未设置，请设置 CHATGPT2API_AUTH_KEY 或在 .env 中填写 CHATGPT2API_AUTH_KEY")
	}
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

func (s *Store) AuthKey() string {
	return strings.TrimSpace(fmt.Sprint(s.settingValue("auth-key", "")))
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

func (s *Store) ImageConcurrentLimit() int {
	value := intSetting(s.settingValue("image_concurrent_limit", 4), 4)
	if value < 1 {
		return 1
	}
	return value
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

func (s *Store) ImagePriceCents() int {
	value := intSetting(s.settingValue("image_price_cents", 8), 8)
	if value < 1 {
		return 1
	}
	return value
}

func (s *Store) EmailAllowedDomains() []string {
	defaults := []string{
		"qq.com",
		"163.com",
		"126.com",
		"gmail.com",
		"outlook.com",
		"hotmail.com",
		"icloud.com",
		"yahoo.com",
		"foxmail.com",
		"sina.com",
	}
	raw := s.settingValue("email_allowed_domains", strings.Join(defaults, ","))
	rawText := ""
	switch value := raw.(type) {
	case []string:
		rawText = strings.Join(value, ",")
	case []any:
		items := make([]string, 0, len(value))
		for _, item := range value {
			items = append(items, strings.TrimSpace(fmt.Sprint(item)))
		}
		rawText = strings.Join(items, ",")
	default:
		rawText = fmt.Sprint(raw)
	}
	parts := strings.Split(strings.ToLower(strings.TrimSpace(rawText)), ",")
	out := make([]string, 0, len(parts))
	seen := map[string]struct{}{}
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	if len(out) == 0 {
		return defaults
	}
	return out
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

func (s *Store) AppTitle() string {
	value := strings.TrimSpace(fmt.Sprint(s.settingValue("app_title", "chatgpt2api")))
	if value == "" {
		return "chatgpt2api"
	}
	return value
}

func (s *Store) ProjectName() string {
	value := strings.TrimSpace(fmt.Sprint(s.settingValue("project_name", "chatgpt2api")))
	if value == "" {
		return "chatgpt2api"
	}
	return value
}

func (s *Store) AppLogoURL() string {
	return strings.TrimSpace(fmt.Sprint(s.settingValue("app_logo_url", "/logo-mark.svg")))
}

func (s *Store) SiteIconURL() string {
	return strings.TrimSpace(fmt.Sprint(s.settingValue("site_icon_url", "/logo-mark.svg")))
}

func (s *Store) Proxy() string {
	return strings.TrimSpace(fmt.Sprint(s.settingValue("proxy", "")))
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

func (s *Store) YiPay() YiPayConfig {
	s.mu.RLock()
	data := util.CopyMap(s.data)
	s.mu.RUnlock()
	return s.yiPayFromData(data)
}

func (s *Store) EmailSMTP() EmailSMTPConfig {
	s.mu.RLock()
	data := util.CopyMap(s.data)
	s.mu.RUnlock()
	return s.emailSMTPFromData(data)
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

func (s *Store) emailSMTPFromData(data map[string]any) EmailSMTPConfig {
	port := intSetting(s.settingValueFromData(data, "email_smtp_port", 465), 465)
	if port < 1 || port > 65535 {
		port = 465
	}
	return EmailSMTPConfig{
		Enabled:   util.ToBool(s.settingValueFromData(data, "email_smtp_enabled", false)),
		Host:      strings.TrimSpace(fmt.Sprint(s.settingValueFromData(data, "email_smtp_host", "smtp.qq.com"))),
		Port:      port,
		UseSSL:    util.ToBool(s.settingValueFromData(data, "email_smtp_use_ssl", true)),
		Username:  strings.TrimSpace(fmt.Sprint(s.settingValueFromData(data, "email_smtp_username", ""))),
		AuthCode:  strings.TrimSpace(fmt.Sprint(s.settingValueFromData(data, "email_smtp_auth_code", ""))),
		FromEmail: strings.TrimSpace(fmt.Sprint(s.settingValueFromData(data, "email_smtp_from_email", ""))),
		FromName:  strings.TrimSpace(fmt.Sprint(s.settingValueFromData(data, "email_smtp_from_name", "chatgpt2api"))),
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
	data["user_default_concurrent_limit"] = s.UserDefaultConcurrentLimit()
	data["user_default_rpm_limit"] = s.UserDefaultRPMLimit()
	data["image_retention_days"] = s.ImageRetentionDays()
	data["auto_remove_invalid_accounts"] = s.AutoRemoveInvalidAccounts()
	data["auto_remove_rate_limited_accounts"] = s.AutoRemoveRateLimitedAccounts()
	data["log_levels"] = s.LogLevels()
	data["app_title"] = s.AppTitle()
	data["project_name"] = s.ProjectName()
	data["app_logo_url"] = s.AppLogoURL()
	data["site_icon_url"] = s.SiteIconURL()
	data["proxy"] = s.Proxy()
	data["base_url"] = s.BaseURL()
	linuxdo := s.LinuxDoOAuth()
	data["linuxdo_enabled"] = linuxdo.Enabled
	data["linuxdo_client_id"] = linuxdo.ClientID
	data["linuxdo_client_secret_configured"] = linuxdo.ClientSecret != ""
	data["linuxdo_redirect_url"] = linuxdo.RedirectURL
	data["linuxdo_frontend_redirect_url"] = linuxdo.FrontendRedirectURL
	data["login_page_image_url"] = s.LoginPageImageURL()
	data["login_page_image_mode"] = s.LoginPageImageMode()
	data["login_page_image_zoom"] = s.LoginPageImageZoom()
	data["login_page_image_position_x"] = s.LoginPageImagePositionX()
	data["login_page_image_position_y"] = s.LoginPageImagePositionY()
	data["email_allowed_domains"] = s.EmailAllowedDomains()
	smtp := s.EmailSMTP()
	data["email_smtp_enabled"] = smtp.Enabled
	data["email_smtp_host"] = smtp.Host
	data["email_smtp_port"] = smtp.Port
	data["email_smtp_use_ssl"] = smtp.UseSSL
	data["email_smtp_username"] = smtp.Username
	data["email_smtp_from_email"] = smtp.FromEmail
	data["email_smtp_from_name"] = smtp.FromName
	data["email_smtp_auth_code_configured"] = smtp.AuthCode != ""
	data["image_price_cents"] = s.ImagePriceCents()
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
	delete(data, "auth-key")
	delete(data, "linuxdo_client_secret")
	delete(data, "email_smtp_auth_code")
	delete(data, "yipay_key")
	return data
}

func (s *Store) Update(data map[string]any) (map[string]any, error) {
	s.mu.Lock()
	next := util.CopyMap(s.data)
	for key, value := range data {
		if key == "linuxdo_client_secret_configured" || key == "email_smtp_auth_code_configured" || key == "yipay_key_configured" {
			continue
		}
		if key == "linuxdo_client_secret" && strings.TrimSpace(fmt.Sprint(value)) == "" {
			continue
		}
		if key == "email_smtp_auth_code" && strings.TrimSpace(fmt.Sprint(value)) == "" {
			continue
		}
		if key == "yipay_key" && strings.TrimSpace(fmt.Sprint(value)) == "" {
			continue
		}
		next[key] = value
	}
	if value, ok := next["login_page_image_mode"]; ok {
		next["login_page_image_mode"] = normalizeLoginPageImageMode(value)
	}
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
	now := time.Now()
	s.mu.Lock()
	if !s.lastImageCleanupAt.IsZero() && now.Sub(s.lastImageCleanupAt) < imageCleanupMinInterval {
		s.mu.Unlock()
		return 0
	}
	s.lastImageCleanupAt = now
	s.mu.Unlock()

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
	appLogoURL := strings.TrimSpace(util.Clean(data["app_logo_url"]))
	if err := validateBrandingAssetURL(appLogoURL); err != nil {
		return errors.New("app_logo_url must be an absolute http(s) URL, an absolute path, or data:image")
	}
	siteIconURL := strings.TrimSpace(util.Clean(data["site_icon_url"]))
	if err := validateBrandingAssetURL(siteIconURL); err != nil {
		return errors.New("site_icon_url must be an absolute http(s) URL, an absolute path, or data:image")
	}

	linuxdo := s.linuxDoOAuthFromData(data)
	if linuxdo.Enabled {
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
	}

	yipay := s.yiPayFromData(data)
	if yipay.Enabled {
		if yipay.PID == "" {
			return errors.New("YiPay PID is required when enabled")
		}
		if yipay.Key == "" {
			return errors.New("YiPay key is required when enabled")
		}
		if yipay.SubmitURL == "" {
			return errors.New("YiPay submit URL is required when enabled")
		}
		if err := validateAbsoluteHTTPURL(yipay.SubmitURL); err != nil {
			return errors.New("YiPay submit URL must be an absolute http(s) URL")
		}
		if yipay.NotifyURL != "" {
			if err := validateAbsoluteHTTPURL(yipay.NotifyURL); err != nil {
				return errors.New("YiPay notify URL must be an absolute http(s) URL")
			}
		}
		if yipay.ReturnURL != "" {
			if err := validateAbsoluteHTTPURL(yipay.ReturnURL); err != nil {
				return errors.New("YiPay return URL must be an absolute http(s) URL")
			}
		}
	}
	paypal := s.payPalFromData(data)
	if paypal.Enabled {
		if paypal.CheckoutURL == "" {
			return errors.New("PayPal checkout URL is required when enabled")
		}
		if err := validateAbsoluteHTTPURL(paypal.CheckoutURL); err != nil {
			return errors.New("PayPal checkout URL must be an absolute http(s) URL")
		}
	}
	usdt := s.usdtFromData(data)
	if usdt.Enabled {
		if usdt.Address == "" {
			return errors.New("USDT address is required when enabled")
		}
		if usdt.PaymentURL != "" {
			if err := validateAbsoluteHTTPURL(usdt.PaymentURL); err != nil {
				return errors.New("USDT payment URL must be an absolute http(s) URL")
			}
		}
	}

	price := intSetting(data["image_price_cents"], 8)
	if price < 1 {
		return errors.New("image_price_cents must be greater than 0")
	}
	if _, exists := data["email_allowed_domains"]; exists {
		domainsText := ""
		switch value := data["email_allowed_domains"].(type) {
		case []string:
			domainsText = strings.Join(value, ",")
		case []any:
			parts := make([]string, 0, len(value))
			for _, item := range value {
				parts = append(parts, strings.TrimSpace(fmt.Sprint(item)))
			}
			domainsText = strings.Join(parts, ",")
		default:
			domainsText = strings.TrimSpace(fmt.Sprint(data["email_allowed_domains"]))
		}
		domains := strings.Split(strings.ToLower(strings.TrimSpace(domainsText)), ",")
		for _, item := range domains {
			domain := strings.TrimSpace(item)
			if domain == "" {
				continue
			}
			if strings.Contains(domain, "@") || strings.Contains(domain, "/") || strings.Contains(domain, "\\") {
				return errors.New("email_allowed_domains contains invalid domain")
			}
		}
	}
	smtp := s.emailSMTPFromData(data)
	if smtp.Enabled {
		if smtp.Host == "" {
			return errors.New("email SMTP host is required when enabled")
		}
		if smtp.Port < 1 || smtp.Port > 65535 {
			return errors.New("email SMTP port must be between 1 and 65535")
		}
		if smtp.Username == "" {
			return errors.New("email SMTP username is required when enabled")
		}
		if smtp.AuthCode == "" {
			return errors.New("email SMTP auth code is required when enabled")
		}
		if smtp.FromEmail == "" {
			return errors.New("email SMTP from email is required when enabled")
		}
		if !strings.Contains(smtp.FromEmail, "@") {
			return errors.New("email SMTP from email is invalid")
		}
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

func validateBrandingAssetURL(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	if strings.HasPrefix(value, "data:image/") {
		return nil
	}
	if strings.HasPrefix(value, "/") && !strings.HasPrefix(value, "//") {
		return nil
	}
	return validateAbsoluteHTTPURL(value)
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
		if key != "auth-key" {
			keys = append(keys, key)
		}
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
