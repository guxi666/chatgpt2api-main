// Package version exposes the application version injected at build time.
package version

import "strings"

// Version is overridden by builds with:
//
//	go build -ldflags "-X chatgpt2api/internal/version.Version=1.2.3"
var Version = "0.0.0-dev"

// Get returns the normalized application version.
func Get() string {
	if value := strings.TrimSpace(Version); value != "" {
		return value
	}
	return "0.0.0-dev"
}
