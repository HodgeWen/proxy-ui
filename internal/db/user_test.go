package db

import (
	"testing"
)

func TestCreateUser_SubscriptionToken(t *testing.T) {
	// Use in-memory SQLite for test
	path := ":memory:"
	if err := Init(path); err != nil {
		t.Fatalf("Init: %v", err)
	}

	u := &User{
		Name: "test-sub-user",
	}
	if err := CreateUser(u); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if u.SubscriptionToken == "" {
		t.Error("CreateUser should set SubscriptionToken")
	}
	if len(u.SubscriptionToken) != 16 {
		t.Errorf("SubscriptionToken length want 16, got %d", len(u.SubscriptionToken))
	}

	found, err := GetUserBySubscriptionToken(u.SubscriptionToken)
	if err != nil {
		t.Fatalf("GetUserBySubscriptionToken: %v", err)
	}
	if found.ID != u.ID || found.Name != u.Name {
		t.Errorf("GetUserBySubscriptionToken: got user %v, want %v", found, u)
	}
}
