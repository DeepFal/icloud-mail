package store

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"icloud-api/internal/domain"
)

func createAppleHealthBatchAccount(t *testing.T, db *Store, index int, enabled *bool) domain.Account {
	t.Helper()
	email := fmt.Sprintf("health-batch-%d@icloud.com", index)
	account := domain.Account{Name: email, Email: email, IMAPUsername: email,
		IMAPHost: "imap.mail.me.com", IMAPPort: 993, PasswordCiphertext: "fixture", Enabled: true}
	if enabled != nil {
		account.AutoLogin = &domain.AccountAutoLoginConfig{Enabled: *enabled, Region: "global", CredentialsCiphertext: "fixture"}
	}
	created, err := db.CreateAccount(context.Background(), account)
	if err != nil {
		t.Fatal(err)
	}
	return created
}

func TestListAccountAppleHealthMatchesIndividualReads(t *testing.T) {
	ctx := context.Background()
	db := openAccountVersionTestStore(t)
	enabled, disabled := true, false
	accounts := []domain.Account{
		createAppleHealthBatchAccount(t, db, 0, &enabled),
		createAppleHealthBatchAccount(t, db, 1, &enabled),
		createAppleHealthBatchAccount(t, db, 2, &enabled),
		createAppleHealthBatchAccount(t, db, 3, &disabled),
		createAppleHealthBatchAccount(t, db, 4, nil),
		createAppleHealthBatchAccount(t, db, 5, &enabled), // Not requested.
	}
	statuses := [][2]string{{"authenticated", "needs_action"}, {"logging_in", "waiting_code"}, {"unknown_state", "retry_wait"}, {"needs_action", "needs_action"}}
	channels := []string{domain.AutoLoginChannelAppleAccount, domain.AutoLoginChannelICloudWeb}
	for i, statuses := range statuses {
		for j, channel := range channels {
			_, err := db.DB().ExecContext(ctx, `INSERT INTO account_auto_login_states(account_id,channel,config_revision,status) VALUES(?,?,1,?)`, accounts[i].ID, channel, statuses[j])
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	until := time.Now().UTC().Add(time.Hour).Truncate(time.Second)
	for i, account := range accounts {
		for j, channel := range channels {
			if err := db.PauseAppleChannelCreation(ctx, account.ID, channel, until.Add(time.Duration(i+j)*time.Minute)); err != nil {
				t.Fatal(err)
			}
		}
	}
	// Sparse IDs exercise the second IN batch without creating hundreds of rows.
	ids := []int64{accounts[0].ID, accounts[1].ID, accounts[2].ID, accounts[3].ID, 0, -1, accounts[0].ID}
	for i := 0; i < appleHealthBatchSize; i++ {
		ids = append(ids, 100000+int64(i))
	}
	ids = append(ids, accounts[4].ID)
	got, err := db.ListAccountAppleHealth(ctx, ids)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != appleHealthBatchSize+5 {
		t.Fatalf("result count=%d, want %d", len(got), appleHealthBatchSize+5)
	}
	if _, ok := got[accounts[5].ID]; ok {
		t.Fatal("unrequested account returned")
	}
	ranks := map[string]int{"idle": 1, "authenticated": 2, "logging_in": 3, "waiting_code": 3, "retry_wait": 4, "needs_action": 5}
	for _, account := range accounts[:5] {
		want := AccountAppleHealth{KeepAliveStatus: "disabled"}
		config, err := db.GetAccountAutoLoginConfig(ctx, account.ID)
		if err != nil && !errors.Is(err, ErrNotFound) {
			t.Fatal(err)
		}
		want.ConfigEnabled = config.Enabled
		if config.Enabled {
			want.KeepAliveStatus = "idle"
		}
		for _, channel := range channels {
			if config.Enabled {
				state, err := db.GetAccountAutoLoginState(ctx, account.ID, channel)
				if err == nil && ranks[state.Status] > ranks[want.KeepAliveStatus] {
					want.KeepAliveStatus = state.Status
				}
			}
			paused, err := db.AppleCreationChannelPausedUntil(ctx, account.ID, channel)
			if err != nil {
				t.Fatal(err)
			}
			if paused.After(want.CreationCooldownUntil) {
				want.CreationCooldownUntil = paused
			}
		}
		if !reflect.DeepEqual(got[account.ID], want) {
			t.Fatalf("account %d health=%+v, want %+v", account.ID, got[account.ID], want)
		}
	}
	if got[100000] != (AccountAppleHealth{KeepAliveStatus: "disabled"}) {
		t.Fatalf("missing account health=%+v", got[100000])
	}
}

func TestListAppleSessionsMatchesIndividualReads(t *testing.T) {
	ctx := context.Background()
	db := openAccountVersionTestStore(t)
	account := createAppleHealthBatchAccount(t, db, 1, nil)
	unrequested := createAppleHealthBatchAccount(t, db, 2, nil)
	validated := time.Now().UTC().Truncate(time.Second)
	for _, id := range []int64{account.ID, unrequested.ID} {
		record := domain.AppleWebSession{AccountID: id, Ciphertext: "web fixture", AppleID: "fixture@icloud.com", Region: "USA", Authenticated: true, LastValidatedAt: &validated}
		if _, err := db.UpsertAppleWebSession(ctx, record); err != nil {
			t.Fatal(err)
		}
		record.Ciphertext, record.Authenticated = "account fixture", false
		if _, err := db.UpsertAppleAccountSession(ctx, record); err != nil {
			t.Fatal(err)
		}
	}
	ids := []int64{0, -1}
	for i := 0; i < appleHealthBatchSize; i++ {
		ids = append(ids, 100000+int64(i))
	}
	ids = append(ids, account.ID, account.ID)
	web, management, err := db.ListAppleSessions(ctx, ids)
	if err != nil {
		t.Fatal(err)
	}
	if len(web) != 1 || len(management) != 1 {
		t.Fatalf("session counts=%d,%d; want 1,1", len(web), len(management))
	}
	wantWeb, err := db.GetAppleWebSession(ctx, account.ID)
	if err != nil {
		t.Fatal(err)
	}
	wantAccount, err := db.GetAppleAccountSession(ctx, account.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(web[account.ID], wantWeb) || !reflect.DeepEqual(management[account.ID], wantAccount) {
		t.Fatalf("batched sessions differ from individual reads: web=%+v account=%+v", web, management)
	}
}

func TestAppleHealthEmptyIDsDoNotQuery(t *testing.T) {
	// A zero Store cannot query: empty and invalid sets must return before SQL.
	db := &Store{}
	for _, ids := range [][]int64{nil, {}, {0, -1, 0}} {
		health, err := db.ListAccountAppleHealth(context.Background(), ids)
		if err != nil || len(health) != 0 {
			t.Fatalf("empty health=%v, %v", health, err)
		}
		web, account, err := db.ListAppleSessions(context.Background(), ids)
		if err != nil || len(web) != 0 || len(account) != 0 {
			t.Fatalf("empty sessions=%v,%v, %v", web, account, err)
		}
	}
}

func TestListAccountAppleHealthKeepsSourcesIndependentOnReadFailure(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		table, wantStatus string
		wantCooldown      bool
	}{
		{"account_auto_login_configs", "unknown", true},
		{"account_auto_login_states", "idle", true},
		{"apple_creation_channel_cooldowns", "needs_action", false},
	} {
		t.Run(tc.table, func(t *testing.T) {
			db := openAccountVersionTestStore(t)
			enabled := true
			account := createAppleHealthBatchAccount(t, db, 1, &enabled)
			if err := db.SaveAccountAutoLoginState(ctx, domain.AccountAutoLoginState{
				AccountID: account.ID, Channel: domain.AutoLoginChannelAppleAccount, ConfigRevision: 1, Status: "needs_action",
			}); err != nil {
				t.Fatal(err)
			}
			until := time.Now().UTC().Add(time.Hour).Truncate(time.Second)
			if err := db.PauseAppleChannelCreation(ctx, account.ID, domain.AutoLoginChannelICloudWeb, until); err != nil {
				t.Fatal(err)
			}
			if _, err := db.DB().ExecContext(ctx, `DROP TABLE `+tc.table); err != nil {
				t.Fatal(err)
			}
			got, err := db.ListAccountAppleHealth(ctx, []int64{account.ID})
			if err == nil {
				t.Fatal("missing table did not report a read error")
			}
			item := got[account.ID]
			if item.KeepAliveStatus != tc.wantStatus {
				t.Fatalf("keep-alive=%q, want %q", item.KeepAliveStatus, tc.wantStatus)
			}
			if tc.wantCooldown && !item.CreationCooldownUntil.Equal(until) || !tc.wantCooldown && !item.CreationCooldownUntil.IsZero() {
				t.Fatalf("cooldown=%v, wantCooldown=%v", item.CreationCooldownUntil, tc.wantCooldown)
			}
		})
	}
}

func TestListAppleSessionsPreservesOtherChannelOnReadFailure(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		table string
		web   bool
	}{{appleWebSessionTable, true}, {appleAccountSessionTable, false}} {
		t.Run(tc.table, func(t *testing.T) {
			db := openAccountVersionTestStore(t)
			account := createAppleHealthBatchAccount(t, db, 1, nil)
			record := domain.AppleWebSession{AccountID: account.ID, Ciphertext: "fixture", AppleID: account.Email, Region: "USA", Authenticated: true}
			if _, err := db.UpsertAppleWebSession(ctx, record); err != nil {
				t.Fatal(err)
			}
			if _, err := db.UpsertAppleAccountSession(ctx, record); err != nil {
				t.Fatal(err)
			}
			if _, err := db.DB().ExecContext(ctx, `DROP TABLE `+tc.table); err != nil {
				t.Fatal(err)
			}
			ids := []int64{account.ID}
			for i := 0; i < appleHealthBatchSize; i++ {
				ids = append(ids, 100000+int64(i))
			}
			web, management, err := db.ListAppleSessions(ctx, ids)
			var batchErr *AppleSessionBatchError
			if !errors.As(err, &batchErr) {
				t.Fatalf("read error=%v, want AppleSessionBatchError", err)
			}
			failed, healthy := batchErr.Account, batchErr.Web
			retained, missing := web, management
			if tc.web {
				failed, healthy = batchErr.Web, batchErr.Account
				retained, missing = management, web
			}
			if len(failed) != len(ids) || len(healthy) != 0 {
				t.Fatalf("channel errors=%+v, want %d failing IDs only", batchErr, len(ids))
			}
			if len(retained) != 1 || retained[account.ID].Ciphertext != "fixture" || len(missing) != 0 {
				t.Fatalf("healthy channel lost or failed records leaked: web=%+v account=%+v", web, management)
			}
		})
	}
}
