package util

import "strings"

const UpstreamConnectionFailureMessage = "upstream connection failed before TLS handshake completed; check proxy reachability to chatgpt.com or change proxy"

func SummarizeUpstreamConnectionError(message string) (string, bool) {
	text := strings.TrimSpace(message)
	if text == "" {
		return "", false
	}
	lower := strings.ToLower(text)
	if strings.Contains(lower, "utls.handshakecontext") ||
		strings.Contains(lower, "http/2 request failed") ||
		strings.Contains(lower, "http/1.1 fallback failed") ||
		strings.Contains(lower, "tls connect error") ||
		strings.Contains(lower, "openssl_internal") ||
		strings.Contains(lower, "curl: (35)") ||
		((strings.Contains(lower, "tls") || strings.Contains(lower, "handshake")) && strings.Contains(lower, "eof")) {
		return UpstreamConnectionFailureMessage, true
	}
	return "", false
}

func IsRetryableUpstreamError(message string) bool {
	text := strings.TrimSpace(message)
	if text == "" {
		return false
	}
	if _, ok := SummarizeUpstreamConnectionError(text); ok {
		return true
	}
	lower := strings.ToLower(text)
	if strings.HasPrefix(lower, "bootstrap failed") {
		return true
	}
	if strings.Contains(lower, "cf_chl") ||
		strings.Contains(lower, "challenge-platform") ||
		strings.Contains(lower, "enable javascript and cookies to continue") ||
		strings.Contains(lower, "cloudflare challenge") {
		return true
	}
	return strings.Contains(lower, "context deadline exceeded") ||
		strings.Contains(lower, "client.timeout exceeded") ||
		strings.Contains(lower, "i/o timeout") ||
		strings.Contains(lower, "timeout awaiting response headers")
}
