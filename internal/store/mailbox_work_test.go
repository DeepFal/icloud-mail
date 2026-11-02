package store_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	"icloud-api/internal/domain"
)

func TestMailboxPreferencesAndHistoryJobLifecycle(t *testing.T) {
	ctx := context.Background()
	db, _ := openArchiveV2Store(t, 1<<20)
	account := createAccount(t, ctx, db, "history", "history@example.com")
	p, err := db.GetMailboxPreferences(ctx, account.ID)
	if err != nil || !p.BackgroundEnabled || p.HistoryDays != 7 {
		t.Fatalf("defaults=%#v err=%v", p, err)
	}
	p.BackgroundEnabled = false
	p.HistoryDays = 14
	if _, err = db.SetMailboxPreferences(ctx, p); err != nil {
		t.Fatal(err)
	}
	if got, _ := db.GetMailboxPreferences(ctx, account.ID); got.BackgroundEnabled || got.HistoryDays != 14 {
		t.Fatalf("saved preferences=%#v", got)
	}

	j, err := db.CreateMailHistoryJob(ctx, account, "history-job-0000001", 7)
	if err != nil {
		t.Fatal(err)
	}
	if again, err := db.CreateMailHistoryJob(ctx, account, j.ID, 7); err != nil || again.ID != j.ID {
		t.Fatalf("idempotent create=%#v err=%v", again, err)
	}
	if _, err := db.CreateMailHistoryJob(ctx, account, j.ID, 8); err == nil {
		t.Fatal("different days did not conflict")
	}
	if _, err := db.CreateMailHistoryJob(ctx, account, "history-job-0000002", 7); err == nil {
		t.Fatal("second active account job did not conflict")
	}
	if _, err = db.ActMailHistoryJob(ctx, account.ID, j.ID, "pause"); err != nil {
		t.Fatal(err)
	}
	if _, err = db.ActMailHistoryJob(ctx, account.ID, j.ID, "resume"); err != nil {
		t.Fatal(err)
	}
	j, _ = db.GetMailHistoryJob(ctx, account.ID, j.ID)
	if ok, err := db.ClaimMailHistoryJob(ctx, j); err != nil || !ok {
		t.Fatalf("claim=%v err=%v", ok, err)
	}
	if err := db.RecoverMailHistoryJobs(ctx); err != nil {
		t.Fatal(err)
	}
	j, _ = db.GetMailHistoryJob(ctx, account.ID, j.ID)
	if j.Status != "queued" {
		t.Fatalf("recovered status=%s", j.Status)
	}
	if _, err = db.ActMailHistoryJob(ctx, account.ID, j.ID, "cancel"); err != nil {
		t.Fatal(err)
	}
}

func TestApplyHistoryMailboxBatchDoesNotMoveLiveCursor(t *testing.T) {
	ctx := context.Background()
	db, _ := openArchiveV2Store(t, 1<<20)
	account := createAccount(t, ctx, db, "history batch", "history-batch@example.com")
	alias := createAlias(t, ctx, db, account.ID, "batch@example.com", bytes.Repeat([]byte{9}, 32))
	applyArchiveV2Batch(t, ctx, db, account.ID, []domain.Alias{alias}, []domain.ArchivedMessage{{AccountID: account.ID, UIDValidity: 55, UID: 100, InternalDate: time.Now().UTC(), Subject: "live", RawMIME: []byte("live"), AliasIDs: []int64{alias.ID}}}, 55, 100, true)
	beforeCursor, err := db.GetIMAPSyncState(ctx, account.ID)
	if err != nil || beforeCursor.LastUID != 100 || beforeCursor.UIDValidity != 55 {
		t.Fatalf("live cursor=%#v err=%v", beforeCursor, err)
	}
	j, err := db.CreateMailHistoryJob(ctx, account, "history-job-0000003", 3)
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := db.ClaimMailHistoryJob(ctx, j); err != nil || !ok {
		t.Fatalf("claim=%v err=%v", ok, err)
	}
	j, _ = db.GetMailHistoryJob(ctx, account.ID, j.ID)
	at := time.Now().UTC()
	b := domain.MailHistoryBatch{UIDValidity: 55, LastUID: 4, TargetUID: 8, Scanned: 4, Done: true, Messages: []domain.ArchivedMessage{{AccountID: account.ID, UIDValidity: 55, UID: 4, InternalDate: at, Subject: "history", RawMIME: []byte("history body"), AliasIDs: []int64{alias.ID}}}}
	if err := db.ApplyHistoryMailboxBatch(ctx, account, []domain.Alias{alias}, j, b); err != nil {
		t.Fatal(err)
	}
	afterCursor, err := db.GetIMAPSyncState(ctx, account.ID)
	if err != nil || afterCursor.LastUID != 100 || afterCursor.UIDValidity != 55 {
		t.Fatalf("history moved live cursor: before=%#v after=%#v", beforeCursor, afterCursor)
	}
	msgs, err := db.ListArchivedMailboxMessages(ctx, alias.ID)
	if err != nil || len(msgs) != 2 {
		t.Fatalf("archive=%#v err=%v", msgs, err)
	}
	// Replaying the same historical batch must not create another archive row.
	job, _ := db.GetMailHistoryJob(ctx, account.ID, j.ID)
	if job.Status != "completed" {
		t.Fatalf("job status=%s", job.Status)
	}
	if err := db.ApplyHistoryMailboxBatch(ctx, account, []domain.Alias{alias}, job, b); err == nil {
		t.Fatal("completed job batch unexpectedly committed")
	}
	j2, err := db.CreateMailHistoryJob(ctx, account, "history-job-0000004", 2)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.ActMailHistoryJob(ctx, account.ID, j2.ID, "pause"); err != nil {
		t.Fatal(err)
	}
	if err = db.ApplyHistoryMailboxBatch(ctx, account, []domain.Alias{alias}, j2, domain.MailHistoryBatch{UIDValidity: 55, LastUID: 101, TargetUID: 101, Messages: []domain.ArchivedMessage{{AccountID: account.ID, UIDValidity: 55, UID: 101, InternalDate: at.Add(time.Minute), Subject: "paused", RawMIME: []byte("paused"), AliasIDs: []int64{alias.ID}}}}); err == nil {
		t.Fatal("paused history batch committed")
	}
	msgs, _ = db.ListArchivedMailboxMessages(ctx, alias.ID)
	if len(msgs) != 2 {
		t.Fatalf("paused batch changed archive: %d", len(msgs))
	}
}
