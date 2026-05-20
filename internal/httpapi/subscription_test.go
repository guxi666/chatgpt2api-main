package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"chatgpt2api/internal/service"
)

func TestSubscriptionsRespectAdminSwitch(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	if _, err := app.config.Update(map[string]any{"subscription_enabled": false}); err != nil {
		t.Fatalf("disable subscription: %v", err)
	}
	_, rawKey, err := app.auth.CreateAPIKey(service.AuthRoleUser, "subscriber", service.AuthOwner{})
	if err != nil {
		t.Fatalf("CreateAPIKey() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/subscriptions/plans", nil)
	req.Header.Set("Authorization", "Bearer "+rawKey)
	res := httptest.NewRecorder()
	app.Handler().ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("plans status = %d body = %s", res.Code, res.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatalf("plans json: %v", err)
	}
	if body["enabled"] != false {
		t.Fatalf("plans enabled = %#v, want false", body["enabled"])
	}
	plans, ok := body["plans"].([]any)
	if !ok || len(plans) != 3 {
		t.Fatalf("plans = %#v, want 3 configured tiers", body["plans"])
	}

	req = httptest.NewRequest(http.MethodPost, "/api/subscriptions/orders", bytes.NewBufferString(`{"tier":"monthly","pay_type":"balance"}`))
	req.Header.Set("Authorization", "Bearer "+rawKey)
	res = httptest.NewRecorder()
	app.Handler().ServeHTTP(res, req)
	if res.Code != http.StatusForbidden {
		t.Fatalf("order status = %d body = %s", res.Code, res.Body.String())
	}
}
