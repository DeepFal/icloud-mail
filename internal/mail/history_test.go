package mail

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	imap "github.com/emersion/go-imap/v2"
	"icloud-api/internal/domain"
)

func TestHistoryFetchPinsSnapshotResumesAndFiltersPrecisely(t *testing.T) {
	fixture := startArchiveIMAPFixture(t, 0, 0)
	fixture.batchBodies = true
	now := time.Date(2026, 10, 29, 0, 0, 0, 0, time.UTC)
	since := now.Add(-7 * 24 * time.Hour)
	uid := uint32(0)
	appendMail := func(address string, at time.Time) {
		t.Helper()
		uid++
		raw := fmt.Sprintf("From: sender@example.test\r\nTo: %s\r\nX-Original-To: %s\r\nMessage-ID: <%d@history.test>\r\nSubject: history %d\r\n\r\nHistory content %d\r\n", address, address, uid, uid, uid)
		fixture.raw[uid] = raw
		if _, err := fixture.user.Append("INBOX", strings.NewReader(raw), &imap.AppendOptions{Time: at, Flags: []imap.Flag{imap.FlagSeen}}); err != nil {
			t.Fatal(err)
		}
	}
	for range 300 {
		appendMail(fixture.alias.Address, since.Add(-24*time.Hour))
	}
	want := map[uint32]bool{}
	for i := range 7 {
		appendMail(fixture.alias.Address, since.Add(time.Duration(i)*time.Hour))
		want[uid] = true
	}
	appendMail("unregistered@example.test", now.Add(-time.Hour))
	pinned := uid
	f := NewFetcher()
	f.ArchiveTempDir = t.TempDir()
	f.MaxIncrementalCandidates = 64
	j := domain.MailHistoryJob{Since: since, Until: now}
	found := 0
	for turn := range 10 {
		batch, err := f.FetchHistoryBatch(context.Background(), fixture.account, "fixture-password", []domain.Alias{fixture.alias}, j)
		if err != nil {
			t.Fatal(err)
		}
		if batch.TargetUID != pinned {
			t.Fatalf("history snapshot moved: got %d want %d", batch.TargetUID, pinned)
		}
		for _, m := range batch.Messages {
			if !want[m.UID] {
				t.Fatalf("unexpected history body UID %d", m.UID)
			}
			delete(want, m.UID)
			found++
			if m.RawMIMEPath != "" {
				_ = os.Remove(m.RawMIMEPath)
			}
		}
		j.Initialized = true
		j.UIDValidity = batch.UIDValidity
		j.LastUID = batch.LastUID
		j.TargetUID = batch.TargetUID
		if turn == 0 {
			appendMail(fixture.alias.Address, now.Add(time.Minute))
		}
		if batch.Done {
			break
		}
	}
	if found != 7 || len(want) != 0 || j.LastUID != pinned {
		t.Fatalf("history incomplete: found=%d remaining=%d last=%d target=%d", found, len(want), j.LastUID, pinned)
	}
	j.UIDValidity++
	_, err := f.FetchHistoryBatch(context.Background(), fixture.account, "fixture-password", []domain.Alias{fixture.alias}, j)
	if !errors.Is(err, ErrHistoryMailboxChanged) {
		t.Fatalf("changed UIDVALIDITY accepted: %v", err)
	}
}
