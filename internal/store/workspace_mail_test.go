package store_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	"icloud-api/internal/domain"
)

func TestWorkspaceMailStateIsolationAndRestore(t *testing.T) {
	ctx := context.Background()
	db, _ := openArchiveV2Store(t, 1<<20)
	account := createAccount(t, ctx, db, "workspace", "workspace@example.com")
	first := createAlias(t, ctx, db, account.ID, "first@example.com", bytes.Repeat([]byte{1}, 32))
	second := createAlias(t, ctx, db, account.ID, "second@example.com", bytes.Repeat([]byte{2}, 32))
	var admin1, admin2 int64
	if err := db.DB().QueryRowContext(ctx, `INSERT INTO admins(username,password_hash,created_at) VALUES(?,?,?) RETURNING id`, "ws-admin-1", "x", time.Now().UnixNano()).Scan(&admin1); err != nil {
		t.Fatal(err)
	}
	if err := db.DB().QueryRowContext(ctx, `INSERT INTO admins(username,password_hash,created_at) VALUES(?,?,?) RETURNING id`, "ws-admin-2", "x", time.Now().UnixNano()).Scan(&admin2); err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 10, 1, 1, 0, 0, 0, time.UTC)
	applyArchiveV2Batch(t, ctx, db, account.ID, []domain.Alias{first, second}, []domain.ArchivedMessage{
		{AccountID: account.ID, UIDValidity: 9, UID: 1, InternalDate: at, Subject: "first message", RawMIME: []byte("first"), OTP: "123456", AliasIDs: []int64{first.ID}},
		{AccountID: account.ID, UIDValidity: 9, UID: 2, InternalDate: at.Add(time.Minute), Subject: "second message", RawMIME: []byte("second"), AliasIDs: []int64{second.ID}},
	}, 9, 2, true)
	p, err := db.ListWorkspaceMail(ctx, domain.WorkspaceMailFilter{AdminID: admin1, Limit: 10})
	if err != nil || p.Total != 2 || p.UnreadCount != 2 || len(p.Items) != 2 {
		t.Fatalf("initial page=%#v err=%v", p, err)
	}
	if _, err = db.PatchWorkspaceMail(ctx, admin1, first.ID, 1, domain.WorkspaceMailPatch{Read: boolPtr(true), Starred: boolPtr(true), Archived: boolPtr(true)}); err != nil {
		t.Fatal(err)
	}
	p, err = db.ListWorkspaceMail(ctx, domain.WorkspaceMailFilter{AdminID: admin1, Folder: "archived", Limit: 10})
	if err != nil || p.Total != 1 || p.Items[0].Starred != true || p.Items[0].Read != true {
		t.Fatalf("archived state=%#v err=%v", p, err)
	}
	p2, err := db.ListWorkspaceMail(ctx, domain.WorkspaceMailFilter{AdminID: admin2, Limit: 10})
	if err != nil || p2.UnreadCount != 2 || p2.Items[0].Read || p2.Items[0].Starred {
		t.Fatalf("admin isolation=%#v err=%v", p2, err)
	}
	if _, err = db.PatchWorkspaceMail(ctx, admin1, first.ID, 1, domain.WorkspaceMailPatch{Deleted: boolPtr(true)}); err != nil {
		t.Fatal(err)
	}
	p, err = db.ListWorkspaceMail(ctx, domain.WorkspaceMailFilter{AdminID: admin1, Folder: "trash", Limit: 10})
	if err != nil || p.Total != 1 {
		t.Fatalf("trash=%#v err=%v", p, err)
	}
	if _, err = db.PatchWorkspaceMail(ctx, admin1, first.ID, 1, domain.WorkspaceMailPatch{Deleted: boolPtr(false)}); err != nil {
		t.Fatal(err)
	}
	if _, err = db.GetArchivedMailboxMessage(ctx, first.ID, 1); err != nil {
		t.Fatalf("archive changed by state patch: %v", err)
	}
}

func TestWorkspaceMailFilterPaginationAndEscaping(t *testing.T) {
	ctx := context.Background()
	db, _ := openArchiveV2Store(t, 1<<20)
	account := createAccount(t, ctx, db, "filter", "filter@example.com")
	alias := createAlias(t, ctx, db, account.ID, "filter@example.com", bytes.Repeat([]byte{3}, 32))
	var admin int64
	if err := db.DB().QueryRowContext(ctx, `INSERT INTO admins(username,password_hash,created_at) VALUES(?,?,?) RETURNING id`, "ws-filter", "x", time.Now().UnixNano()).Scan(&admin); err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
	msgs := make([]domain.ArchivedMessage, 0, 4)
	for i := 1; i <= 4; i++ {
		subject := "decoy"
		if i <= 3 {
			subject = "literal %_ subject"
		}
		msgs = append(msgs, domain.ArchivedMessage{AccountID: account.ID, UIDValidity: 10, UID: uint32(i), InternalDate: base.Add(time.Duration(i) * time.Minute), Subject: subject, RawMIME: []byte("x"), AliasIDs: []int64{alias.ID}})
	}
	applyArchiveV2Batch(t, ctx, db, account.ID, []domain.Alias{alias}, msgs, 10, 4, true)
	p, err := db.ListWorkspaceMail(ctx, domain.WorkspaceMailFilter{AdminID: admin, Query: "%_", Limit: 2, Offset: 0})
	if err != nil || len(p.Items) != 2 || !p.HasMore || p.Items[0].MailboxUID != 3 || p.Items[1].MailboxUID != 2 {
		t.Fatalf("pagination/escape=%#v err=%v", p, err)
	}
	p, err = db.ListWorkspaceMail(ctx, domain.WorkspaceMailFilter{AdminID: admin, Query: "%_", Limit: 2, Offset: 2})
	if err != nil || len(p.Items) != 1 || p.HasMore || p.Items[0].MailboxUID != 1 {
		t.Fatalf("second page=%#v err=%v", p, err)
	}
}

func boolPtr(v bool) *bool { return &v }
