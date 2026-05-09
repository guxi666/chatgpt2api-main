package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/mail"
	"net/url"
	"strings"
	"time"

	"chatgpt2api/internal/util"
)

type cfTurnstileVerifyResponse struct {
	Success    bool     `json:"success"`
	ErrorCodes []string `json:"error-codes"`
}

func loginBodyTurnstileToken(body map[string]any) string {
	return firstNonEmpty(
		util.Clean(body["cf_turnstile_token"]),
		util.Clean(body["turnstile_token"]),
		util.Clean(body["captcha_token"]),
	)
}

func normalizeRegisterEmail(email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return "", errors.New("email is required")
	}
	parsed, err := mail.ParseAddress(email)
	if err != nil {
		return "", errors.New("invalid email")
	}
	normalized := strings.ToLower(strings.TrimSpace(parsed.Address))
	if normalized != email || !strings.Contains(email, "@") {
		return "", errors.New("invalid email")
	}
	return normalized, nil
}

func emailDomain(email string) string {
	email = strings.TrimSpace(email)
	_, domain, ok := strings.Cut(email, "@")
	if !ok {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(domain))
}

func (a *App) isRegistrationEmailAllowed(email string) bool {
	domain := emailDomain(email)
	if domain == "" {
		return false
	}
	allowed := a.config.RegistrationAllowedEmailDomains()
	for _, item := range allowed {
		if strings.EqualFold(strings.TrimSpace(item), domain) {
			return true
		}
	}
	return false
}

func (a *App) requireCFTurnstile(r *http.Request, token string) error {
	if !a.config.CFTurnstileEnabled() {
		return nil
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return errors.New("请完成 Cloudflare 验证")
	}
	ok, reason, err := verifyCFTurnstileToken(r.Context(), a.config.CFTurnstileSecretKey(), token, clientIP(r))
	if err != nil {
		return errors.New("Cloudflare 验证失败，请稍后重试")
	}
	if !ok {
		if reason != "" {
			return fmt.Errorf("Cloudflare 验证未通过: %s", reason)
		}
		return errors.New("Cloudflare 验证未通过")
	}
	return nil
}

func verifyCFTurnstileToken(ctx context.Context, secret, token, remoteIP string) (bool, string, error) {
	payload := url.Values{}
	payload.Set("secret", strings.TrimSpace(secret))
	payload.Set("response", strings.TrimSpace(token))
	if ip := strings.TrimSpace(remoteIP); ip != "" {
		payload.Set("remoteip", ip)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://challenges.cloudflare.com/turnstile/v0/siteverify", strings.NewReader(payload.Encode()))
	if err != nil {
		return false, "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false, "", err
	}
	defer resp.Body.Close()
	var result cfTurnstileVerifyResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, "", err
	}
	if result.Success {
		return true, "", nil
	}
	return false, strings.Join(result.ErrorCodes, ","), nil
}
