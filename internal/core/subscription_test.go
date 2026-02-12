package core

import (
	"encoding/base64"
	"testing"
	"time"

	"github.com/s-ui/s-ui/internal/db"
	"gorm.io/datatypes"
)

func TestGenerateBase64(t *testing.T) {
	path := ":memory:"
	if err := db.Init(path); err != nil {
		t.Fatalf("Init: %v", err)
	}

	u := &db.User{Name: "sub-test"}
	if err := db.CreateUser(u); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	// Create inbound with tls.server_name
	cfgJSON := []byte(`{"tls":{"server_name":"example.com"}}`)
	ib := &db.Inbound{
		Tag:        "vless-test",
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

	got, err := db.GetUserByID(u.ID)
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}

	body, err := GenerateBase64(got)
	if err != nil {
		t.Fatalf("GenerateBase64: %v", err)
	}
	if len(body) == 0 {
		t.Fatal("GenerateBase64 returned empty")
	}
	decoded, err := base64.StdEncoding.DecodeString(string(body))
	if err != nil {
		t.Fatalf("Base64 decode: %v", err)
	}
	if len(decoded) == 0 {
		t.Fatal("decoded empty")
	}
}

func TestGenerateClash(t *testing.T) {
	path := ":memory:"
	if err := db.Init(path); err != nil {
		t.Fatalf("Init: %v", err)
	}

	u := &db.User{Name: "clash-test"}
	if err := db.CreateUser(u); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	cfgJSON := []byte(`{"tls":{"server_name":"clash.example.com"}}`)
	ib := &db.Inbound{
		Tag:        "vless-clash",
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

	got, err := db.GetUserByID(u.ID)
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}

	body, err := GenerateClash(got)
	if err != nil {
		t.Fatalf("GenerateClash: %v", err)
	}
	if len(body) == 0 {
		t.Fatal("GenerateClash returned empty")
	}
	if !contains(string(body), "proxies:") {
		t.Errorf("GenerateClash missing proxies: %s", string(body))
	}
	if !contains(string(body), "vless") {
		t.Errorf("GenerateClash missing vless: %s", string(body))
	}
}

func TestBuildUserinfoHeader(t *testing.T) {
	u := &db.User{
		TrafficUsed:  1000,
		TrafficLimit: 5000,
		ExpireAt:     ptrTime(time.Unix(2218532293, 0)),
	}
	h := BuildUserinfoHeader(u)
	if !contains(h, "upload=500") {
		t.Errorf("BuildUserinfoHeader missing upload: %s", h)
	}
	if !contains(h, "download=500") {
		t.Errorf("BuildUserinfoHeader missing download: %s", h)
	}
	if !contains(h, "total=5000") {
		t.Errorf("BuildUserinfoHeader missing total: %s", h)
	}
	if !contains(h, "expire=2218532293") {
		t.Errorf("BuildUserinfoHeader missing expire: %s", h)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || len(s) > 0 && findSub(s, sub) >= 0)
}

func findSub(s, sub string) int {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func ptrTime(t time.Time) *time.Time {
	return &t
}
