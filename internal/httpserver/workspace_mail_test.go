package httpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
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

func TestWorkspaceMailBatchStateAndHasOTPFilter(t *testing.T) {
	env := newAdminAPITestEnv(t)
	ctx := context.Background()
	if err := env.store.ConfigureMailArchive(t.TempDir(), 4<<20); err != nil {
		t.Fatal(err)
	}
	account := adminAPITestCreateAccount(t, env, "ws-batch-owner@icloud.com")
	alias, _ := createV2AliasFixture(t, env, account.ID, "ws-batch@example.test")
	fresh, err := env.store.GetAccount(ctx, account.ID)
	if err != nil {
		t.Fatal(err)
	}
	received := time.Now().UTC()
	archived := func(uid uint32, subject, otp string) domain.ArchivedMessage {
		return domain.ArchivedMessage{AccountID: account.ID, UIDValidity: 4, UID: uid, InternalDate: received, Subject: subject, From: []domain.MailAddress{{Email: "sender@example.test"}}, RawMIME: []byte(workspaceFixtureMIME), AliasIDs: []int64{alias.ID}, OTP: otp}
	}
	result := domain.MailboxSyncResult{State: domain.IMAPSyncState{AccountID: account.ID, UIDValidity: 4, LastUID: 11, UpdatedAt: received}, Reset: true, ArchivedMessages: []domain.ArchivedMessage{archived(10, "with code", "123456"), archived(11, "no code", "")}}
	if err := env.store.ApplyMailboxSync(ctx, account.ID, fresh.UpdatedAt, []domain.Alias{alias}, result, received); err != nil {
		t.Fatal(err)
	}
	cookie, csrf, _ := env.createSession(t, "workspace-batch-admin", "test-password")
	cookies := []*http.Cookie{cookie}
	items := fmt.Sprintf(`[{"alias_id":%d,"mailbox_uid":1},{"alias_id":%d,"mailbox_uid":2}]`, alias.ID, alias.ID)
	post := func(body, token string) *httptest.ResponseRecorder {
		return env.request(t, http.MethodPost, "/admin/api/v1/messages/batch", []byte(body), "application/json", cookies, token)
	}
	count := func(query string) int {
		t.Helper()
		r := env.request(t, http.MethodGet, "/admin/api/v1/messages?"+query, nil, "", cookies, "")
		var page struct {
			Data struct {
				Total int `json:"total"`
			} `json:"data"`
		}
		if r.Code != 200 || json.Unmarshal(r.Body.Bytes(), &page) != nil {
			t.Fatalf("list %s=%d %s", query, r.Code, r.Body.String())
		}
		return page.Data.Total
	}

	if r := env.request(t, http.MethodPost, "/admin/api/v1/messages/batch", []byte(`{}`), "application/json", nil, ""); r.Code != 401 {
		t.Fatalf("unauthenticated=%d", r.Code)
	}
	if r := post(`{"action":"read","items":`+items+`}`, ""); r.Code != 403 {
		t.Fatalf("missing csrf=%d", r.Code)
	}
	if count("unread=1") != 2 || count("has_otp=1") != 1 {
		t.Fatal("unexpected initial counts")
	}
	if r := env.request(t, http.MethodGet, "/admin/api/v1/messages?has_otp=maybe", nil, "", cookies, ""); r.Code != 400 {
		t.Fatalf("invalid has_otp=%d", r.Code)
	}

	// Invalid action, empty list, over the limit and malformed refs are rejected.
	for name, body := range map[string]string{
		"unknown action": `{"action":"purge","items":` + items + `}`,
		"empty items":    `{"action":"read","items":[]}`,
		"zero uid":       fmt.Sprintf(`{"action":"read","items":[{"alias_id":%d,"mailbox_uid":0}]}`, alias.ID),
		"over limit":     `{"action":"read","items":[` + strings.TrimSuffix(strings.Repeat(`{"alias_id":1,"mailbox_uid":1},`, workspaceMailBatchLimit+1), ",") + `]}`,
	} {
		if r := post(body, csrf); r.Code != 400 {
			t.Fatalf("%s=%d %s", name, r.Code, r.Body.String())
		}
	}
	if count("unread=1") != 2 {
		t.Fatal("rejected batch changed state")
	}

	// A missing message rolls the whole batch back.
	missing := fmt.Sprintf(`{"action":"read","items":[{"alias_id":%d,"mailbox_uid":1},{"alias_id":%d,"mailbox_uid":99}]}`, alias.ID, alias.ID)
	if r := post(missing, csrf); r.Code != 404 {
		t.Fatalf("missing id=%d %s", r.Code, r.Body.String())
	}
	if count("unread=1") != 2 {
		t.Fatal("partial batch was committed")
	}
	otherAlias := fmt.Sprintf(`{"action":"read","items":[{"alias_id":%d,"mailbox_uid":1}]}`, alias.ID+100)
	if r := post(otherAlias, csrf); r.Code != 404 {
		t.Fatalf("unknown alias=%d", r.Code)
	}

	for _, step := range []struct {
		action string
		query  string
		want   int
	}{
		{"read", "unread=1", 0},
		{"unread", "unread=1", 2},
		{"archive", "folder=archived", 2},
		{"unarchive", "folder=archived", 0},
		{"trash", "folder=trash", 2},
		{"restore", "folder=trash", 0},
	} {
		r := post(`{"action":"`+step.action+`","items":`+items+`}`, csrf)
		if r.Code != 200 || !strings.Contains(r.Body.String(), `"updated":2`) {
			t.Fatalf("%s=%d %s", step.action, r.Code, r.Body.String())
		}
		if got := count(step.query); got != step.want {
			t.Fatalf("after %s %s = %d, want %d", step.action, step.query, got, step.want)
		}
	}
}
