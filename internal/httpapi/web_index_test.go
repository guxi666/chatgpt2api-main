package httpapi

import (
	"strings"
	"testing"
)

func TestRewriteWebIndexBrand(t *testing.T) {
	raw := `<!doctype html><html><head><link rel="icon" href="/favicon.ico" /><title>Old</title></head><body></body></html>`

	got := rewriteWebIndexBrand(raw, `Site <A>`, `/brand.svg?x=1&y=2`)

	if want := `<title>Site &lt;A&gt;</title>`; !strings.Contains(got, want) {
		t.Fatalf("rewritten title missing %q in %q", want, got)
	}
	if want := `<link rel="icon" href="/brand.svg?x=1&amp;y=2" />`; !strings.Contains(got, want) {
		t.Fatalf("rewritten icon missing %q in %q", want, got)
	}
	if want := `"site_logo_url":"/brand.svg?x=1\u0026y=2"`; !strings.Contains(got, want) {
		t.Fatalf("injected app meta missing %q in %q", want, got)
	}
}

func TestIsWebIndexRequest(t *testing.T) {
	for _, path := range []string{"/", "/settings", "/accounts/pool"} {
		if !isWebIndexRequest(path) {
			t.Fatalf("%s should be served as SPA index", path)
		}
	}
	for _, path := range []string{"/assets/index.js", "/favicon.ico", "/logo-mark.svg"} {
		if isWebIndexRequest(path) {
			t.Fatalf("%s should be served as static asset", path)
		}
	}
}
