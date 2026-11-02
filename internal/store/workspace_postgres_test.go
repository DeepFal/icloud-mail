package store_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"icloud-api/internal/domain"
	"icloud-api/internal/store"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

func TestWorkspacePostgresSafePoolAndObservedSession(t *testing.T) {
	dsn := os.Getenv("ICLOUD_TEST_POSTGRES_URL")
	if dsn == "" {
		t.Skip("ICLOUD_TEST_POSTGRES_URL is not set")
	}
	u, err := url.Parse(dsn)
	if err != nil || u.Hostname() != "127.0.0.1" || !strings.Contains(u.Path, "test") {
		t.Fatal("disposable loopback test DB required")
	}
	db, _, account, aliases, _ := poolFixture(t, 1, dsn)
	ctx := context.Background()
	if err := db.ConfigureMailArchive(t.TempDir(), 1<<20); err != nil {
		t.Fatal(err)
	}
	alias := aliases[0]
	applyArchiveV2Batch(t, ctx, db, account.ID, []domain.Alias{alias}, []domain.ArchivedMessage{{AccountID: account.ID, UIDValidity: 1, UID: 1, InternalDate: time.Now().Add(-time.Hour), RawMIME: []byte("old message"), OTP: "123456", AliasIDs: []int64{alias.ID}}}, 1, 1, true)
	client, _, err := db.CreateSafePoolClient(ctx, "PG safe project")
	if err != nil {
		t.Fatal(err)
	}
	leases, err := db.ClaimPool(ctx, client.ID, store.PoolClaim{RequestID: "pg-safe-claim", Count: 1, TTLSeconds: 60})
	if err != nil {
		t.Fatal(err)
	}
	if messages, err := db.ListAccessibleMailboxMessages(ctx, alias.ID); err != nil || len(messages) != 0 {
		t.Fatalf("prior allocation exposed: %d %v", len(messages), err)
	}
	before, err := db.GetAlias(ctx, alias.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.UpdatePoolClient(ctx, client.ID, false, false); err != nil {
		t.Fatal(err)
	}
	after, err := db.GetAlias(ctx, alias.ID)
	if err != nil || bytes.Equal(before.APIKeyHash, after.APIKeyHash) {
		t.Fatalf("project credential revocation failed: %v", err)
	}
	if _, err := db.DB().ExecContext(ctx, `UPDATE pool_leases SET expires_at=$1 WHERE id=$2`, time.Now().Add(-time.Second).UnixNano(), leases[0].ID); err != nil {
		t.Fatal(err)
	}
	if err := db.RefreshPool(ctx); err != nil {
		t.Fatal(err)
	}
	reviews, err := db.ListPoolReleaseReviews(ctx)
	if err != nil || len(reviews) != 1 {
		t.Fatalf("expiry review missing: %d %v", len(reviews), err)
	}
	if err := db.ResolvePoolReleaseReview(ctx, leases[0].ID, true); err != nil {
		t.Fatal(err)
	}
	validated := time.Now()
	old, err := db.UpsertAppleWebSession(ctx, domain.AppleWebSession{AccountID: account.ID, AppleID: account.Email, Region: "global", Ciphertext: "old", Authenticated: true, LastValidatedAt: &validated})
	if err != nil {
		t.Fatal(err)
	}
	next := old
	next.Ciphertext = "mail-refresh"
	if saved, err := db.UpdateObservedAppleWebSession(ctx, account, old, next); err != nil || !saved {
		t.Fatalf("matching observed session not saved: %v %v", saved, err)
	}
	if saved, err := db.UpdateObservedAppleWebSession(ctx, account, old, next); err != nil || saved {
		t.Fatalf("stale observed session replaced new state: %v %v", saved, err)
	}
}

func TestWorkspacePostgresStateAndHistoryCompatibility(t *testing.T) {
	dsn := os.Getenv("ICLOUD_TEST_POSTGRES_URL")
	if dsn == "" {
		t.Skip("ICLOUD_TEST_POSTGRES_URL is not set")
	}
	u, err := url.Parse(dsn)
	if err != nil || u.Hostname() != "127.0.0.1" || !strings.Contains(u.Path, "test") {
		t.Fatal("disposable loopback test DB required")
	}
	ctx := context.Background()
	db, err := store.Open(dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.ConfigureMailArchive(t.TempDir(), 1<<20); err != nil {
		t.Fatal(err)
	}
	suffix := fmt.Sprint(time.Now().UnixNano())
	account := createAccount(t, ctx, db, "PG workspace", "workspace-"+suffix+"@icloud.com")
	alias := createAlias(t, ctx, db, account.ID, "workspace-alias-"+suffix+"@icloud.com", func() []byte { sum := sha256.Sum256([]byte(suffix)); return sum[:] }())
	var admin int64
	if err := db.DB().QueryRowContext(ctx, `INSERT INTO admins(username,password_hash,created_at) VALUES($1,'fixture',$2) RETURNING id`, "ws-"+suffix, time.Now().UnixNano()).Scan(&admin); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	applyArchiveV2Batch(t, ctx, db, account.ID, []domain.Alias{alias}, []domain.ArchivedMessage{{AccountID: account.ID, UIDValidity: 81, UID: 100, InternalDate: now, Subject: "PG %_ message", RawMIME: []byte("From: test@example.test\r\n\r\nbody"), AliasIDs: []int64{alias.ID}}}, 81, 100, true)
	if _, err := db.PatchWorkspaceMail(ctx, admin, alias.ID, 1, domain.WorkspaceMailPatch{Read: boolPtr(true), Starred: boolPtr(true)}); err != nil {
		t.Fatal(err)
	}
	p, err := db.ListWorkspaceMail(ctx, domain.WorkspaceMailFilter{AdminID: admin, AliasID: alias.ID, Query: "%_", Starred: true})
	if err != nil || p.Total != 1 || !p.Items[0].Read {
		t.Fatalf("PG state=%#v %v", p, err)
	}
	fresh, err := db.GetAccount(ctx, account.ID)
	if err != nil {
		t.Fatal(err)
	}
	j, err := db.CreateMailHistoryJob(ctx, fresh, "pg-history-"+suffix, 7)
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := db.ClaimMailHistoryJob(ctx, j); err != nil || !ok {
		t.Fatalf("claim %v %v", ok, err)
	}
	b := domain.MailHistoryBatch{UIDValidity: 81, LastUID: 4, TargetUID: 100, Scanned: 4, Done: true, Messages: []domain.ArchivedMessage{{AccountID: account.ID, UIDValidity: 81, UID: 4, InternalDate: now.Add(-time.Hour), RawMIME: []byte("From: test@example.test\r\n\r\nolder"), AliasIDs: []int64{alias.ID}}}}
	if err := db.ApplyHistoryMailboxBatch(ctx, fresh, []domain.Alias{alias}, j, b); err != nil {
		t.Fatal(err)
	}
	live, err := db.GetIMAPSyncState(ctx, account.ID)
	if err != nil || live.LastUID != 100 {
		t.Fatalf("PG live cursor=%#v %v", live, err)
	}
	if err := db.Migrate(ctx); err != nil {
		t.Fatalf("repeated additive migration: %v", err)
	}
}

func TestWorkspacePostgresLegacyCreationPacing(t *testing.T) {
	dsn := os.Getenv("ICLOUD_TEST_POSTGRES_URL")
	if dsn == "" {
		t.Skip("ICLOUD_TEST_POSTGRES_URL is not set")
	}
	u, err := url.Parse(dsn)
	if err != nil || u.Hostname() != "127.0.0.1" || !strings.Contains(u.Path, "test") {
		t.Fatal("disposable loopback test DB required")
	}
	ctx := context.Background()
	db, err := store.Open(dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	// This fixture is a dedicated throwaway database, reset by the test runner.
	if _, err := db.DB().ExecContext(ctx, `ALTER TABLE alias_creation_jobs DROP COLUMN interval_seconds`); err != nil {
		t.Fatal(err)
	}
	if err := db.EnsureAliasCreationJobs(ctx); err != nil {
		t.Fatal(err)
	}
	suffix := fmt.Sprint(time.Now().UnixNano())
	account := createAccount(t, ctx, db, "PG pacing", "pg-pacing-"+suffix+"@icloud.com")
	next := time.Now().Add(time.Hour)
	job := store.AliasCreationJob{ID: "pg-paced-" + suffix, AccountID: account.ID, Target: 100, Completed: 1, Channel: "apple_account", IntervalSeconds: 300, Status: "waiting", NextRunAt: &next}
	if err := db.CreateAliasCreationJob(ctx, job); err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if err := db.InterruptAliasCreationJobs(ctx); err != nil {
		t.Fatal(err)
	}
	got, err := db.GetAliasCreationJob(ctx, account.ID, job.ID)
	if err != nil || got.IntervalSeconds != 300 || got.Completed != 1 || got.Status != "waiting" || got.NextRunAt == nil {
		t.Fatalf("PG paced checkpoint=%+v %v", got, err)
	}
}
