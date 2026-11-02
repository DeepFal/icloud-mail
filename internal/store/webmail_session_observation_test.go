package store_test

import (
	"context"
	"icloud-api/internal/domain"
	"testing"
	"time"
)

func TestObservedWebMailSessionCannotReplaceNewLoginOrLogout(t *testing.T) {
	db := openTestStore(t)
	ctx := context.Background()
	account := createAccount(t, ctx, db, "web source", "web-observe@icloud.com")
	validated := time.Now()
	old, err := db.UpsertAppleWebSession(ctx, domain.AppleWebSession{AccountID: account.ID, AppleID: account.Email, Region: "global", Ciphertext: "old", Authenticated: true, LastValidatedAt: &validated})
	if err != nil {
		t.Fatal(err)
	}
	next := old
	next.Ciphertext = "mail-update"
	if _, err := db.UpsertAppleWebSession(ctx, domain.AppleWebSession{AccountID: account.ID, AppleID: account.Email, Region: "global", Ciphertext: "new-login", Authenticated: true, LastValidatedAt: &validated}); err != nil {
		t.Fatal(err)
	}
	if saved, err := db.UpdateObservedAppleWebSession(ctx, account, old, next); err != nil || saved {
		t.Fatalf("old mail replaced newer login: %v %v", saved, err)
	}
	current, err := db.GetAppleWebSession(ctx, account.ID)
	if err != nil || current.Ciphertext != "new-login" {
		t.Fatal("new login not preserved")
	}
	if err := db.DeleteAppleWebSession(ctx, account.ID); err != nil {
		t.Fatal(err)
	}
	if saved, err := db.UpdateObservedAppleWebSession(ctx, account, current, next); err != nil || saved {
		t.Fatalf("mail resurrected logout: %v %v", saved, err)
	}
}
