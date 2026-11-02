package store_test

import (
	"bytes"
	"context"
	"errors"
	"icloud-api/internal/domain"
	"icloud-api/internal/store"
	"testing"
	"time"
)

func TestSafePoolScopesHistoryAndRevokesIssuedCredentials(t *testing.T) {
	db, _, account, aliases, _ := poolFixture(t, 1, "")
	ctx := context.Background()
	if err := db.ConfigureMailArchive(t.TempDir(), 1<<20); err != nil {
		t.Fatal(err)
	}
	alias := aliases[0]
	past := time.Now().Add(-time.Hour)
	applyArchiveV2Batch(t, ctx, db, account.ID, []domain.Alias{alias}, []domain.ArchivedMessage{{AccountID: account.ID, UIDValidity: 1, UID: 1, InternalDate: past, Subject: "old private mail", RawMIME: []byte("old mail"), OTP: "123456", AliasIDs: []int64{alias.ID}}}, 1, 1, true)
	client, _, err := db.CreateSafePoolClient(ctx, "safe-project")
	if err != nil {
		t.Fatal(err)
	}
	leases, err := db.ClaimPool(ctx, client.ID, store.PoolClaim{RequestID: "safe-history-claim", Count: 1, TTLSeconds: 60})
	if err != nil {
		t.Fatal(err)
	}
	public, err := db.ListAccessibleMailboxMessages(ctx, alias.ID)
	if err != nil || len(public) != 0 {
		t.Fatalf("prior allocation messages exposed: %#v %v", public, err)
	}
	otp, err := db.ListAliasOTPs(ctx, alias.ID, 10)
	if err != nil || len(otp) != 0 {
		t.Fatalf("prior OTP exposed: %v %v", otp, err)
	}
	admin, err := db.ListArchivedMailboxMessages(ctx, alias.ID)
	if err != nil || len(admin) != 1 {
		t.Fatalf("admin archive destroyed: %v %v", admin, err)
	}
	before, err := db.GetAlias(ctx, alias.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.UpdatePoolClient(ctx, client.ID, false, false); err != nil {
		t.Fatal(err)
	}
	after, err := db.GetAlias(ctx, alias.ID)
	if err != nil || after.CredentialVersion <= before.CredentialVersion || bytes.Equal(after.APIKeyHash, before.APIKeyHash) {
		t.Fatal("disabled project retained issued credentials")
	}
	if len(leases) != 1 {
		t.Fatal("lease lost")
	}
}
func TestSafePoolExpiryRequiresExplicitReview(t *testing.T) {
	db, _, _, aliases, _ := poolFixture(t, 1, "")
	ctx := context.Background()
	client, _, err := db.CreateSafePoolClient(ctx, "expiry-review")
	if err != nil {
		t.Fatal(err)
	}
	leases, err := db.ClaimPool(ctx, client.ID, store.PoolClaim{RequestID: "review-expiry-claim", Count: 1, TTLSeconds: 60})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB().ExecContext(ctx, `UPDATE pool_leases SET expires_at=? WHERE id=?`, time.Now().Add(-time.Second).UnixNano(), leases[0].ID); err != nil {
		t.Fatal(err)
	}
	if err := db.RefreshPool(ctx); err != nil {
		t.Fatal(err)
	}
	reviews, err := db.ListPoolReleaseReviews(ctx)
	if err != nil || len(reviews) != 1 {
		t.Fatalf("missing review: %#v %v", reviews, err)
	}
	if _, err := db.ClaimPool(ctx, client.ID, store.PoolClaim{RequestID: "cannot-reclaim-review", Count: 1, TTLSeconds: 60}); !errors.Is(err, store.ErrPoolEmpty) {
		t.Fatalf("uncertain address reassigned: %v", err)
	}
	if err := db.SetPoolMember(ctx, aliases[0].ID, "available"); !errors.Is(err, store.ErrPoolConflict) {
		t.Fatalf("review bypassed: %v", err)
	}
	if err := db.ResolvePoolReleaseReview(ctx, leases[0].ID, true); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ClaimPool(ctx, client.ID, store.PoolClaim{RequestID: "review-confirmed-unused", Count: 1, TTLSeconds: 60}); err != nil {
		t.Fatal(err)
	}
}
