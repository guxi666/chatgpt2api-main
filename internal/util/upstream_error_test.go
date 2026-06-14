package util

import "testing"

func TestSummarizeUpstreamConnectionError(t *testing.T) {
	cases := []string{
		`Get "https://chatgpt.com/": surf: HTTP/2 request failed: uTLS.HandshakeContext() error: EOF; HTTP/1.1 fallback failed: uTLS.HandshakeContext() error: EOF`,
		"curl: (35) OpenSSL SSL_connect: SSL_ERROR_SYSCALL",
		"TLS connect error: connection reset by peer",
		"error: OPENSSL_INTERNAL:WRONG_VERSION_NUMBER",
	}
	for _, input := range cases {
		got, ok := SummarizeUpstreamConnectionError(input)
		if !ok {
			t.Fatalf("SummarizeUpstreamConnectionError(%q) did not match", input)
		}
		if got != UpstreamConnectionFailureMessage {
			t.Fatalf("summary = %q, want %q", got, UpstreamConnectionFailureMessage)
		}
	}

	if got, ok := SummarizeUpstreamConnectionError("upstream returned 500"); ok || got != "" {
		t.Fatalf("non-connection summary = %q, %v", got, ok)
	}
}

func TestIsRetryableUpstreamError(t *testing.T) {
	tests := []struct {
		name    string
		message string
		want    bool
	}{
		{
			name:    "tls handshake eof",
			message: `Get "https://chatgpt.com/": surf: HTTP/2 request failed: uTLS.HandshakeContext() error: EOF; HTTP/1.1 fallback failed: uTLS.HandshakeContext() error: EOF`,
			want:    true,
		},
		{
			name:    "bootstrap html challenge",
			message: "bootstrap failed: status=403, body=<html><script>window._cf_chl_opt={}</script>Enable JavaScript and cookies to continue</html>",
			want:    true,
		},
		{
			name:    "context deadline",
			message: "Post https://chatgpt.com/backend-api/conversation: context deadline exceeded",
			want:    true,
		},
		{
			name:    "io timeout",
			message: "Get https://chatgpt.com/: i/o timeout",
			want:    true,
		},
		{
			name:    "invalid token",
			message: `auth_chat_requirements failed: status=401, body={"detail":"token_invalidated"}`,
			want:    false,
		},
		{
			name:    "empty",
			message: "",
			want:    false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := IsRetryableUpstreamError(test.message); got != test.want {
				t.Fatalf("IsRetryableUpstreamError() = %v, want %v", got, test.want)
			}
		})
	}
}
