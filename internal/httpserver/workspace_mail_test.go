package httpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"icloud-api/internal/domain"
)

const workspaceFixtureMIME = "From: Sender <sender@example.test>\r\nTo: ws@example.test\r\nSubject: workspace fixture\r\nMIME-Version: 1.0\r\nContent-Type: multipart/mixed; boundary=ws\r\n\r\n--ws\r\nContent-Type: text/plain; charset=utf-8\r\n\r\nPlain body 123456\r\n--ws\r\nContent-Type: text/html; charset=utf-8\r\n\r\n<p onclick=evil()>中文内容<script>evil()</script><img src=https://tracker.test/x></p>\r\n--ws\r\nContent-Type: text/plain\r\nContent-Disposition: attachment; filename=notes.txt\r\n\r\nAttachment content\r\n--ws--\r\n"

func TestWorkspaceMailAuthenticatedBodyAttachmentAndState(t *testing.T) {
	env := newAdminAPITestEnv(t)
	ctx := context.Background()
	if err := env.store.ConfigureMailArchive(t.TempDir(), 4<<20); err != nil {
		t.Fatal(err)
	}
	account := adminAPITestCreateAccount(t, env, "ws-owner@icloud.com")
	alias, _ := createV2AliasFixture(t, env, account.ID, "ws@example.test")
	fresh, err := env.store.GetAccount(ctx, account.ID)
	if err != nil {
		t.Fatal(err)
	}
	received := time.Now().UTC()
	result := domain.MailboxSyncResult{State: domain.IMAPSyncState{AccountID: account.ID, UIDValidity: 4, LastUID: 9, UpdatedAt: received}, Reset: true, ArchivedMessages: []domain.ArchivedMessage{{AccountID: account.ID, UIDValidity: 4, UID: 9, InternalDate: received, Subject: "workspace fixture", From: []domain.MailAddress{{Name: "Sender", Email: "sender@example.test"}}, RawMIME: []byte(workspaceFixtureMIME), AliasIDs: []int64{alias.ID}, OTP: "123456"}}}
	if err := env.store.ApplyMailboxSync(ctx, account.ID, fresh.UpdatedAt, []domain.Alias{alias}, result, received); err != nil {
		t.Fatal(err)
	}
	cookie, csrf, _ := env.createSession(t, "workspace-admin", "test-password")
	base := fmt.Sprintf("/admin/api/v1/mailboxes/%d/messages/1", alias.ID)
	if r := env.request(t, http.MethodGet, base, nil, "", nil, ""); r.Code != 401 {
		t.Fatalf("unauthenticated=%d", r.Code)
	}
	r := env.request(t, http.MethodGet, base, nil, "", []*http.Cookie{cookie}, "")
	if r.Code != 200 {
		t.Fatalf("detail=%d %s", r.Code, r.Body.String())
	}
	var detail struct {
		Data struct {
			Text, HTML       string
			ContentAvailable bool `json:"content_available"`
			Attachments      []workspaceAttachment
		}
	}
	if err := json.Unmarshal(r.Body.Bytes(), &detail); err != nil {
		t.Fatal(err)
	}
	if !detail.Data.ContentAvailable || !strings.Contains(detail.Data.Text, "Plain body") || !strings.Contains(detail.Data.HTML, "中文内容") || strings.Contains(detail.Data.HTML, "evil") || strings.Contains(detail.Data.HTML, "tracker") {
		t.Fatalf("wrong/safe body: %s", r.Body.String())
	}
	if len(detail.Data.Attachments) != 1 {
		t.Fatalf("attachments=%#v", detail.Data.Attachments)
	}
	attach := env.request(t, http.MethodGet, "/admin/api/v1"+detail.Data.Attachments[0].DownloadPath, nil, "", []*http.Cookie{cookie}, "")
	if attach.Code != 200 || !strings.Contains(attach.Body.String(), "Attachment content") || !strings.HasPrefix(attach.Header().Get("Content-Disposition"), "attachment") {
		t.Fatalf("attachment=%d %s", attach.Code, attach.Body.String())
	}
	source := env.request(t, http.MethodGet, base+"/source", nil, "", []*http.Cookie{cookie}, "")
	if source.Code != 200 || source.Body.String() != workspaceFixtureMIME {
		t.Fatalf("source=%d", source.Code)
	}
	bad := env.request(t, http.MethodPatch, base, []byte(`{"read":true}`), "", []*http.Cookie{cookie}, "")
	if bad.Code != 403 {
		t.Fatalf("missing csrf=%d", bad.Code)
	}
	patched := env.request(t, http.MethodPatch, base, []byte(`{"read":true,"starred":true,"deleted":true}`), "application/json", []*http.Cookie{cookie}, csrf)
	if patched.Code != 200 {
		t.Fatalf("patch=%d %s", patched.Code, patched.Body.String())
	}
	list := env.request(t, http.MethodGet, "/admin/api/v1/messages?folder=trash", nil, "", []*http.Cookie{cookie}, "")
	if list.Code != 200 || !strings.Contains(list.Body.String(), "workspace fixture") {
		t.Fatalf("trash=%d %s", list.Code, list.Body.String())
	}
	for _, table := range []string{"consumed_messages", "imap_seen_tasks"} {
		var count int
		if err := env.store.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("web reading changed %s", table)
		}
	}
	if missing := env.request(t, http.MethodGet, fmt.Sprintf("/admin/api/v1/mailboxes/%d/messages/1/source", alias.ID+100), nil, "", []*http.Cookie{cookie}, ""); missing.Code != 404 {
		t.Fatalf("wrong alias source=%d", missing.Code)
	}
}

func TestWorkspaceMIMEFilenameAndMetadataBudget(t *testing.T) {
	content, err := parseWorkspaceMIME(strings.NewReader(workspaceFixtureMIME), 2, 3, 1<<20)
	if err != nil || len(content.Attachments) != 1 || content.Attachments[0].Part != "3" {
		t.Fatalf("parse=%#v %v", content, err)
	}
	if got := workspaceFilename("../../bad\r\nname.txt"); got != "badname.txt" {
		t.Fatalf("unsafe attachment filename=%q", got)
	}
}
