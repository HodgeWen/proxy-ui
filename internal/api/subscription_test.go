package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/s-ui/s-ui/internal/db"
	"gorm.io/datatypes"
)

func TestSubscriptionHandler(t *testing.T) {
	if err := db.Init(":memory:"); err != nil {
		t.Fatalf("Init: %v", err)
	}

	u := &db.User{Name: "sub-handler-test"}
	if err := db.CreateUser(u); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	cfgJSON := []byte(`{"tls":{"server_name":"example.com"}}`)
	ib := &db.Inbound{
		Tag:        "vless-handler",
		Protocol:   "vless",
		ListenPort: 443,
		ConfigJSON: datatypes.JSON(cfgJSON),
	}
	if err := db.DB.Create(ib).Error; err != nil {
		t.Fatalf("Create inbound: %v", err)
	}
	if err := db.ReplaceUserInbounds(u.ID, []uint{ib.ID}); err != nil {
		t.Fatalf("ReplaceUserInbounds: %v", err)
	}

	router := chi.NewRouter()
	router.Get("/sub/{token}", SubscriptionHandler)

	// Invalid token -> 404
	req := httptest.NewRequest("GET", "/sub/INVALID_TOKEN", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("invalid token: want 404, got %d", rec.Code)
	}

	// Valid token -> 200, Base64
	req = httptest.NewRequest("GET", "/sub/"+u.SubscriptionToken, nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("valid token: want 200, got %d", rec.Code)
	}
	if rec.Header().Get("subscription-userinfo") == "" {
		t.Error("subscription-userinfo header missing")
	}
	if rec.Body.Len() == 0 {
		t.Error("body empty")
	}

	// Valid token + Clash UA -> YAML
	req = httptest.NewRequest("GET", "/sub/"+u.SubscriptionToken, nil)
	req.Header.Set("User-Agent", "ClashMeta/1.0")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("clash UA: want 200, got %d", rec.Code)
	}
	if rec.Header().Get("subscription-userinfo") == "" {
		t.Error("subscription-userinfo header missing")
	}
	body := rec.Body.String()
	if body == "" || body[0] != 'p' && body[0] != '-' {
		t.Errorf("expected YAML, got: %s", body[:min(50, len(body))])
	}

	// Disabled user -> 403
	u.Enabled = false
	db.UpdateUser(u)
	req = httptest.NewRequest("GET", "/sub/"+u.SubscriptionToken, nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("disabled user: want 403, got %d", rec.Code)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
