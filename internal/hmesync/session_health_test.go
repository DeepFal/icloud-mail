package hmesync

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"icloud-api/internal/apple"
	"icloud-api/internal/domain"
	"icloud-api/internal/store"
)

type countingHealthRepository struct {
	*store.Store
	batches, singleReads int
}

func (r *countingHealthRepository) ListAppleSessions(ctx context.Context, ids []int64) (map[int64]domain.AppleWebSession, map[int64]domain.AppleWebSession, error) {
	r.batches++
	return r.Store.ListAppleSessions(ctx, ids)
}
func (r *countingHealthRepository) GetAppleWebSession(ctx context.Context, id int64) (domain.AppleWebSession, error) {
	r.singleReads++
	return r.Store.GetAppleWebSession(ctx, id)
}
func (r *countingHealthRepository) GetAppleAccountSession(ctx context.Context, id int64) (domain.AppleWebSession, error) {
	r.singleReads++
	return r.Store.GetAppleAccountSession(ctx, id)
}

func TestSessionHealthBatchMatchesSingleReads(t *testing.T) {
	s, db, account := independentAccountFixture(t, true)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	s.now = func() time.Time { return now }
	managed := apple.AccountSession{AppleID: account.Email, Region: apple.RegionGlobal, APIKey: "fixture-key", AuthenticatedAt: now.Add(-time.Hour), ExpiresAt: now.Add(time.Hour), UpdatedAt: now.Add(-time.Minute)}
	newer := managed
	newer.UpdatedAt = now
	retrying := managed
	retrying.ExpiresAt, retrying.RefreshFailures, retrying.RefreshAfter = now.Add(-time.Minute), 2, now.Add(time.Minute)
	rejected := managed
	rejected.RefreshRejected = true
	envelope := func(a *apple.AccountSession) *apple.Session {
		return &apple.Session{AppleID: account.Email, Region: apple.RegionGlobal, Account: a}
	}
	for _, tc := range []struct {
		name                           string
		web, account                   *apple.Session
		corruptWeb, corruptAccount     bool
		unauthenticated, wrongMetadata bool
	}{
		{name: "missing"},
		{name: "web only", web: envelope(nil)},
		{name: "legacy embedded account", web: envelope(&managed)},
		{name: "independent account", account: envelope(&managed)},
		{name: "newer web checkpoint", web: envelope(&newer), account: envelope(&managed)},
		{name: "older web checkpoint", web: envelope(&managed), account: envelope(&newer)},
		{name: "expired during backoff", web: envelope(nil), account: envelope(&retrying)},
		{name: "rejected", account: envelope(&rejected)},
		{name: "corrupt web retains account", web: envelope(nil), account: envelope(&managed), corruptWeb: true},
		{name: "corrupt account prevents legacy fallback", web: envelope(&managed), account: envelope(&managed), corruptAccount: true},
		{name: "explicit logout prevents legacy fallback", web: envelope(&managed), account: envelope(&managed), unauthenticated: true},
		{name: "identical ciphertext still validates metadata", web: envelope(&managed), account: envelope(&managed), wrongMetadata: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_ = db.DeleteAppleWebSession(ctx, account.ID)
			_ = db.DeleteAppleAccountSession(ctx, account.ID)
			for i, session := range []*apple.Session{tc.web, tc.account} {
				if session == nil {
					continue
				}
				payload, err := json.Marshal(session)
				if err != nil {
					t.Fatal(err)
				}
				ciphertext, err := s.cipher.EncryptAppleSession(string(payload))
				if err != nil {
					t.Fatal(err)
				}
				if i == 0 && tc.corruptWeb || i == 1 && tc.corruptAccount {
					ciphertext = "invalid-envelope"
				}
				record := domain.AppleWebSession{AccountID: account.ID, Ciphertext: ciphertext, AppleID: account.Email, Region: "global", Authenticated: i == 0 || !tc.unauthenticated}
				if i == 1 && tc.wrongMetadata {
					web, err := db.GetAppleWebSession(ctx, account.ID)
					if err != nil {
						t.Fatal(err)
					}
					record.Ciphertext, record.AppleID = web.Ciphertext, "different@icloud.com"
				}
				if i == 0 {
					_, err = db.UpsertAppleWebSession(ctx, record)
				} else {
					_, err = db.UpsertAppleAccountSession(ctx, record)
				}
				if err != nil {
					t.Fatal(err)
				}
			}
			web, webErr := s.GetSession(ctx, account.ID)
			acc, accErr := s.GetAccountSession(ctx, account.ID)
			want := SessionHealth{web, acc, webErr, accErr}
			repo := &countingHealthRepository{Store: db}
			s.repo = repo
			ids := make([]int64, 100)
			for i := range ids {
				ids[i] = account.ID + int64(i)
			}
			got, err := s.GetSessionHealth(ctx, append(ids, account.ID))
			s.repo = db
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got[account.ID], want) {
				t.Fatalf("batch = %#v, single reads = %#v", got[account.ID], want)
			}
			if repo.batches != 1 || repo.singleReads != 0 {
				t.Fatalf("100 accounts: batch reads=%d, single reads=%d", repo.batches, repo.singleReads)
			}
			if len(got) != 100 || got[account.ID+99].Web.Status != StatusLoginRequired {
				t.Fatalf("missing/duplicate ID behavior: %#v", got)
			}
		})
	}
}

func TestSessionHealthBatchKeepsWebResultWhenAccountReadFails(t *testing.T) {
	s, db, account := independentAccountFixture(t, true)
	ctx := context.Background()
	if _, err := s.saveSession(ctx, account.ID, apple.Session{AppleID: account.Email, Region: apple.RegionGlobal}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB().ExecContext(ctx, `DROP TABLE apple_account_sessions`); err != nil {
		t.Fatal(err)
	}
	want, err := s.GetSession(ctx, account.ID)
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.GetSessionHealth(ctx, []int64{account.ID})
	if err != nil {
		t.Fatal(err)
	}
	info := got[account.ID]
	if info.WebErr != nil || !reflect.DeepEqual(info.Web, want) || info.AccountErr == nil {
		t.Fatalf("channel errors were conflated: %#v", info)
	}
}
