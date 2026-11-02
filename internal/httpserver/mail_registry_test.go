package httpserver

import (
	"encoding/json"
	"icloud-api/internal/domain"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

func TestMailRegistryAuthAndProfileNote(t *testing.T) {
	env := newAdminAPITestEnv(t)
	a := adminAPITestCreateAccount(t, env, "registry-owner@icloud.com")
	al, _ := createV2AliasFixture(t, env, a.ID, "registry@example.com")
	if r := env.request(t, http.MethodGet, "/admin/api/v1/services", nil, "", nil, ""); r.Code != 401 {
		t.Fatalf("anonymous=%d", r.Code)
	}
	cookie, csrf, _ := env.createSession(t, "registry-admin", "pw")
	path := "/admin/api/v1/aliases/" + strconv.FormatInt(al.ID, 10) + "/profile"
	if r := env.request(t, http.MethodPatch, path, []byte(`{"note":"x"}`), "application/json", []*http.Cookie{cookie}, ""); r.Code != 403 {
		t.Fatalf("csrf=%d", r.Code)
	}
	if r := env.request(t, http.MethodPatch, path, []byte(`{}`), "application/json", []*http.Cookie{cookie}, csrf); r.Code != 400 {
		t.Fatalf("missing note=%d", r.Code)
	}
	r := env.request(t, http.MethodPatch, path, []byte(`{"note":"备注"}`), "application/json", []*http.Cookie{cookie}, csrf)
	if r.Code != 200 {
		t.Fatal(r.Code, r.Body.String())
	}
	var got struct {
		Data domain.MailboxProfile `json:"data"`
	}
	_ = json.Unmarshal(r.Body.Bytes(), &got)
	if got.Data.Note != "备注" {
		t.Fatalf("note=%q", got.Data.Note)
	}
}

func TestMailRegistryCRUDBatchAndQueries(t *testing.T) {
	env := newAdminAPITestEnv(t)
	a := adminAPITestCreateAccount(t, env, "registry-owner2@icloud.com")
	al, _ := createV2AliasFixture(t, env, a.ID, "registry2@example.com")
	al2, _ := createV2AliasFixture(t, env, a.ID, "registry3@example.com")
	cookie, csrf, _ := env.createSession(t, "registry-admin2", "pw")
	create := env.request(t, http.MethodPost, "/admin/api/v1/services", adminAPITestJSON(t, map[string]any{"name": "GitHub", "domain": "github.com", "note": "x"}), "application/json", []*http.Cookie{cookie}, csrf)
	if create.Code != 201 {
		t.Fatal(create.Code, create.Body.String())
	}
	var svc struct {
		Data domain.MailService `json:"data"`
	}
	_ = json.Unmarshal(create.Body.Bytes(), &svc)
	reg := func(id int64, body string) *httptest.ResponseRecorder {
		return env.request(t, http.MethodPut, "/admin/api/v1/aliases/"+strconv.FormatInt(id, 10)+"/registrations", []byte(body), "application/json", []*http.Cookie{cookie}, csrf)
	}
	if r := reg(al.ID, `{"service_id":`+strconv.FormatInt(svc.Data.ID, 10)+`,"account_identifier":"u","status":"registered"}`); r.Code != 200 {
		t.Fatal(r.Code, r.Body.String())
	}
	if r := env.request(t, http.MethodPost, "/admin/api/v1/service-registrations/batch", adminAPITestJSON(t, map[string]any{"alias_ids": []int64{al.ID, al2.ID}, "service_id": svc.Data.ID, "account_identifier": "batch", "status": "pending"}), "application/json", []*http.Cookie{cookie}, csrf); r.Code != 200 {
		t.Fatal(r.Code, r.Body.String())
	}
	if r := env.request(t, http.MethodGet, "/admin/api/v1/service-mailboxes?status=unrecorded&service_id=bad", nil, "", []*http.Cookie{cookie}, csrf); r.Code != 400 {
		t.Fatalf("bad query=%d", r.Code)
	}
}

func TestMailRegistryValidationAtomicityAndUnknownStatus(t *testing.T) {
	env := newAdminAPITestEnv(t)
	account := adminAPITestCreateAccount(t, env, "registry-boundary@icloud.com")
	first, _ := createV2AliasFixture(t, env, account.ID, "registry-first@example.test")
	second, _ := createV2AliasFixture(t, env, account.ID, "registry-second@example.test")
	cookie, csrf, _ := env.createSession(t, "registry-boundary-admin", "pw")
	call := func(method, path string, body any) *httptest.ResponseRecorder {
		var raw []byte
		if body != nil {
			raw = adminAPITestJSON(t, body)
		}
		return env.request(t, method, "/admin/api/v1"+path, raw, "application/json", []*http.Cookie{cookie}, csrf)
	}
	r := call(http.MethodPost, "/services", map[string]any{"name": "Notes-only service"})
	if r.Code != 201 {
		t.Fatal(r.Code, r.Body.String())
	}
	var service struct {
		Data domain.MailService `json:"data"`
	}
	if err := json.Unmarshal(r.Body.Bytes(), &service); err != nil {
		t.Fatal(err)
	}
	servicePath := "/services/" + strconv.FormatInt(service.Data.ID, 10)
	for _, body := range []map[string]any{{}, {"name": nil}, {"note": nil}, {"archived": nil}, {"archived": "true"}, {"unknown": "x"}} {
		if got := call(http.MethodPatch, servicePath, body); got.Code != 400 {
			t.Fatalf("bad service patch %#v: %d", body, got.Code)
		}
	}
	profilePath := "/aliases/" + strconv.FormatInt(first.ID, 10) + "/profile"
	for _, body := range []map[string]any{{}, {"note": nil}, {"note": 3}} {
		if got := call(http.MethodPatch, profilePath, body); got.Code != 400 {
			t.Fatalf("bad note %#v: %d", body, got.Code)
		}
	}
	for _, note := range []string{"本地备注", ""} {
		got := call(http.MethodPatch, profilePath, map[string]any{"note": note})
		if got.Code != 200 {
			t.Fatal(got.Code, got.Body.String())
		}
		var p struct {
			Data domain.MailboxProfile `json:"data"`
		}
		if err := json.Unmarshal(got.Body.Bytes(), &p); err != nil || p.Data.Note != note {
			t.Fatalf("note roundtrip: %s %v", got.Body.String(), err)
		}
	}
	if got := call(http.MethodGet, "/aliases/999999/profile", nil); got.Code != 404 {
		t.Fatalf("missing profile: %d", got.Code)
	}
	sid := strconv.FormatInt(service.Data.ID, 10)
	for _, q := range []string{"", "?service_id=0", "?service_id=" + sid + "&status=invalid", "?service_id=" + sid + "&enabled_only=true", "?service_id=" + sid + "&offset=1000001"} {
		if got := call(http.MethodGet, "/service-mailboxes"+q, nil); got.Code != 400 {
			t.Fatalf("query %s: %d", q, got.Code)
		}
	}
	regPath := "/aliases/" + strconv.FormatInt(first.ID, 10) + "/registrations"
	if got := call(http.MethodPut, regPath, map[string]any{"service_id": service.Data.ID, "status": "registered"}); got.Code != 200 {
		t.Fatal(got.Code, got.Body.String())
	}
	if got := call(http.MethodPost, "/service-registrations/batch", map[string]any{"alias_ids": []int64{second.ID, first.ID}, "service_id": service.Data.ID, "status": "not_registered"}); got.Code != 409 {
		t.Fatalf("conflict: %d %s", got.Code, got.Body.String())
	}
	if got := call(http.MethodPost, "/service-registrations/batch", map[string]any{"alias_ids": []int64{second.ID, 999999}, "service_id": service.Data.ID, "status": "pending"}); got.Code != 404 {
		t.Fatalf("invalid alias: %d", got.Code)
	}
	got := call(http.MethodGet, "/service-mailboxes?service_id="+sid+"&status=unrecorded", nil)
	var rows struct {
		Data domain.ServiceMailboxPage `json:"data"`
	}
	if err := json.Unmarshal(got.Body.Bytes(), &rows); err != nil || got.Code != 200 || rows.Data.Total != 1 || rows.Data.Items[0].AliasID != second.ID {
		t.Fatalf("unrecorded: %d %s %v", got.Code, got.Body.String(), err)
	}
	if got := call(http.MethodPatch, servicePath, map[string]any{"note": "hello", "domain": "example.test"}); got.Code != 200 {
		t.Fatal(got.Code)
	}
	got = call(http.MethodPatch, servicePath, map[string]any{"note": "", "domain": ""})
	if err := json.Unmarshal(got.Body.Bytes(), &service); err != nil || got.Code != 200 || service.Data.Note != "" || service.Data.Domain != "" {
		t.Fatalf("clear service fields: %d %s", got.Code, got.Body.String())
	}
}
