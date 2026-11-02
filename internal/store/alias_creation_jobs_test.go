package store

import (
	"context"
	"errors"
	"icloud-api/internal/domain"
	"testing"
	"time"
)

func TestAliasCreationJobsProgressAndInterrupt(t *testing.T) {
	s := openAliasDeletionJobTestStore(t, ":memory:")
	_ = createAliasDeletionJobTestAdmin(t, s, "creation-test")
	a, err := s.CreateAccount(context.Background(), domain.Account{Name: "creation", Email: "creation@icloud.com", Enabled: true, IMAPHost: "imap", IMAPPort: 993})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.EnsureAliasCreationJobs(context.Background()); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	j := AliasCreationJob{ID: "job-1", AccountID: a.ID, Target: 30, Completed: 1, Channel: "auto", Status: "running", Entries: []AliasCreationJobEntry{{AliasID: 9, Address: "x@icloud.com", Channel: "auto", CreatedAt: now}}, CreatedAt: now, UpdatedAt: now}
	if err := s.CreateAliasCreationJob(context.Background(), j); err != nil {
		t.Fatal(err)
	}
	if err := s.InterruptAliasCreationJobs(context.Background()); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetLatestAliasCreationJob(context.Background(), a.ID)
	if err != nil || got.Status != "interrupted" || got.Completed != 1 || len(got.Entries) != 1 {
		t.Fatalf("job=%+v err=%v", got, err)
	}
	if _, err := s.GetActiveAliasCreationJob(context.Background(), a.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("active err=%v", err)
	}
}

func TestAliasCreationJobsSingleActive(t *testing.T) {
	s := openAliasDeletionJobTestStore(t, ":memory:")
	_ = createAliasDeletionJobTestAdmin(t, s, "single")
	a, err := s.CreateAccount(context.Background(), domain.Account{Name: "single", Email: "single@icloud.com", Enabled: true, IMAPHost: "imap", IMAPPort: 993})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.EnsureAliasCreationJobs(context.Background()); err != nil {
		t.Fatal(err)
	}
	j := AliasCreationJob{ID: "one", AccountID: a.ID, Target: 1, Channel: "auto", Status: "running"}
	if err := s.CreateAliasCreationJob(context.Background(), j); err != nil {
		t.Fatal(err)
	}
	j.ID = "two"
	if err := s.CreateAliasCreationJob(context.Background(), j); !errors.Is(err, ErrAliasCreationJobConflict) {
		t.Fatalf("err=%v", err)
	}
}

func TestAliasCreationPacingMigratesLegacyAndRecoversOnlySafeWaits(t *testing.T) {
	s := openAliasDeletionJobTestStore(t, ":memory:")
	ctx := context.Background()
	a, err := s.CreateAccount(ctx, domain.Account{Name: "paced", Email: "paced-store@icloud.com", Enabled: true, IMAPHost: "imap", IMAPPort: 993})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB().ExecContext(ctx, `DROP TABLE alias_creation_jobs`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB().ExecContext(ctx, `CREATE TABLE alias_creation_jobs(id TEXT PRIMARY KEY,account_id BIGINT NOT NULL,target INTEGER NOT NULL,completed INTEGER NOT NULL DEFAULT 0,channel TEXT NOT NULL,status TEXT NOT NULL,last_error TEXT NOT NULL DEFAULT '',next_run_at BIGINT,entries_json TEXT NOT NULL DEFAULT '[]',created_at BIGINT NOT NULL,updated_at BIGINT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := s.EnsureAliasCreationJobs(ctx); err != nil {
			t.Fatal(err)
		}
	}
	next := time.Now().Add(time.Hour)
	j := AliasCreationJob{ID: "paced-wait", AccountID: a.ID, Target: 100, Completed: 1, Channel: "apple_account", IntervalSeconds: 300, Status: "waiting", NextRunAt: &next}
	if err := s.CreateAliasCreationJob(ctx, j); err != nil {
		t.Fatal(err)
	}
	if err := s.InterruptAliasCreationJobs(ctx); err != nil {
		t.Fatal(err)
	}
	resumed, err := s.ListResumableAliasCreationJobs(ctx)
	if err != nil || len(resumed) != 1 || resumed[0].IntervalSeconds != 300 || resumed[0].Completed != 1 {
		t.Fatalf("safe wait not recovered: %+v %v", resumed, err)
	}
	j.Status = "running"
	j.NextRunAt = nil
	if err := s.SaveAliasCreationJob(ctx, j); err != nil {
		t.Fatal(err)
	}
	if err := s.InterruptAliasCreationJobs(ctx); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetAliasCreationJob(ctx, a.ID, j.ID)
	if err != nil || got.Status != "interrupted" || got.IntervalSeconds != 300 {
		t.Fatalf("uncertain reservation replayed: %+v %v", got, err)
	}
}
