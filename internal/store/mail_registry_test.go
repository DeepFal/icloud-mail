package store_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"icloud-api/internal/domain"
	"icloud-api/internal/store"
)

func TestMailRegistrySQLiteContract(t *testing.T) { runRegistryContract(t, openTestStore(t)) }
func TestMailRegistryPostgresContract(t *testing.T) {
	dsn := os.Getenv("ICLOUD_TEST_POSTGRES_URL")
	if dsn == "" {
		t.Skip("ICLOUD_TEST_POSTGRES_URL is not set")
	}
	u, err := url.Parse(dsn)
	if err != nil || u.Hostname() != "127.0.0.1" || !strings.Contains(u.Path, "test") {
		t.Fatal("disposable loopback test database required")
	}
	db, err := store.Open(dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	runRegistryContract(t, db)
}
func runRegistryContract(t *testing.T, db *store.Store) {
	t.Helper()
	ctx := context.Background()
	suffix := fmt.Sprint(time.Now().UnixNano())
	key := func(value string) []byte { sum := sha256.Sum256([]byte(value + suffix)); return sum[:] }
	account := createAccount(t, ctx, db, "Registry", "registry-"+suffix+"@icloud.com")
	first := createAlias(t, ctx, db, account.ID, "first-"+suffix+"@icloud.com", key("first"))
	second := createAlias(t, ctx, db, account.ID, "second-"+suffix+"@icloud.com", key("second"))
	third := createAlias(t, ctx, db, account.ID, "third-"+suffix+"@icloud.com", key("third"))
	svc, err := db.CreateMailService(ctx, domain.MailService{Name: "GitHub-" + suffix, Domain: "github.com"})
	if err != nil {
		t.Fatal(err)
	}
	other, err := db.CreateMailService(ctx, domain.MailService{Name: "Other-" + suffix})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.CreateMailService(ctx, domain.MailService{Name: "ＧｉｔＨｕｂ-" + suffix}); !errors.Is(err, store.ErrMailRegistryConflict) {
		t.Fatalf("normalized duplicate: %v", err)
	}
	for _, value := range []string{"https://github.com", "github.com/x", "x#bad", "bad..test", "-bad.test", "bad\n.test"} {
		if _, err = db.CreateMailService(ctx, domain.MailService{Name: "bad-" + suffix, Domain: value}); !errors.Is(err, store.ErrMailRegistryInput) {
			t.Fatalf("bad domain %q: %v", value, err)
		}
	}
	if _, err = db.SetMailboxNote(ctx, first.ID, "工作账号 100%_\n备用用途"); err != nil {
		t.Fatal(err)
	}
	profile, err := db.GetMailboxProfile(ctx, first.ID)
	if err != nil || profile.UpdatedAt == nil || profile.Note != "工作账号 100%_\n备用用途" || len(profile.Registrations) != 0 {
		t.Fatalf("profile: %#v %v", profile, err)
	}
	before, err := db.GetAlias(ctx, first.ID)
	if err != nil || before.Note != profile.Note {
		t.Fatalf("alias note: %#v %v", before, err)
	}
	aliasPage, err := db.ListAliasesPage(ctx, store.AliasListFilter{AccountID: &account.ID, Query: "100%_", Limit: 50})
	if err != nil || aliasPage.Total != 1 || aliasPage.Items[0].ID != first.ID {
		t.Fatalf("alias note search: %#v %v", aliasPage, err)
	}
	if err := db.ConfigureMailArchive(t.TempDir(), 1<<20); err != nil {
		t.Fatal(err)
	}
	admin, err := db.CreateAdmin(ctx, "registry-"+suffix, "fixture")
	if err != nil {
		t.Fatal(err)
	}
	applyArchiveV2Batch(t, ctx, db, account.ID, []domain.Alias{first, second, third}, []domain.ArchivedMessage{{AccountID: account.ID, UIDValidity: 91, UID: 1, InternalDate: time.Now().UTC(), Subject: "registry note lookup", RawMIME: []byte("From: fixture@example.test\r\n\r\nbody"), AliasIDs: []int64{first.ID}}}, 91, 1, true)
	mailPage, err := db.ListWorkspaceMail(ctx, domain.WorkspaceMailFilter{AdminID: admin.ID, AccountID: account.ID, Query: "100%_", Limit: 50})
	if err != nil || mailPage.Total != 1 || mailPage.Items[0].MailboxNote != profile.Note {
		t.Fatalf("workspace note projection/search: %#v %v", mailPage, err)
	}
	negative := domain.ServiceRegistrationInput{ServiceID: svc.ID, Status: "not_registered"}
	if _, err = db.SaveMailboxRegistration(ctx, first.ID, negative); err != nil {
		t.Fatal(err)
	}
	positive := domain.ServiceRegistrationInput{ServiceID: svc.ID, Status: "registered", AccountIdentifier: "UserＡ", Note: "订阅说明"}
	if _, err = db.SaveMailboxRegistration(ctx, first.ID, positive); err != nil {
		t.Fatal(err)
	}
	positive.AccountIdentifier = "usera"
	if _, err = db.SaveMailboxRegistration(ctx, first.ID, positive); err != nil {
		t.Fatal(err)
	}
	profile, err = db.GetMailboxProfile(ctx, first.ID)
	if err != nil || len(profile.Registrations) != 1 || profile.Registrations[0].Status != "registered" {
		t.Fatalf("sentinel/idempotency: %#v %v", profile, err)
	}
	positive.AccountIdentifier = "另一个账号"
	positive.Status = "closed"
	if _, err = db.SaveMailboxRegistration(ctx, first.ID, positive); err != nil {
		t.Fatal(err)
	}
	if _, err = db.SaveMailboxRegistration(ctx, first.ID, negative); !errors.Is(err, store.ErrMailRegistryConflict) {
		t.Fatalf("negative conflict: %v", err)
	}
	if _, err = db.BatchSaveMailboxRegistrations(ctx, []int64{second.ID, first.ID}, negative); !errors.Is(err, store.ErrMailRegistryConflict) {
		t.Fatalf("batch conflict: %v", err)
	}
	profile, err = db.GetMailboxProfile(ctx, second.ID)
	if err != nil || len(profile.Registrations) != 0 {
		t.Fatalf("partial batch: %#v %v", profile, err)
	}
	if _, err = db.BatchSaveMailboxRegistrations(ctx, []int64{second.ID, 999999999}, negative); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("bad alias: %v", err)
	}
	if _, err = db.BatchSaveMailboxRegistrations(ctx, []int64{second.ID, second.ID}, negative); !errors.Is(err, store.ErrMailRegistryInput) {
		t.Fatalf("duplicate aliases: %v", err)
	}
	if _, err = db.SaveMailboxRegistration(ctx, second.ID, negative); err != nil {
		t.Fatal(err)
	}
	if _, err = db.SaveMailboxRegistration(ctx, third.ID, domain.ServiceRegistrationInput{ServiceID: other.ID, Status: "registered"}); err != nil {
		t.Fatal(err)
	}
	byStatus := func(status string) domain.ServiceMailboxPage {
		p, e := db.ListServiceMailboxes(ctx, domain.ServiceMailboxFilter{ServiceID: svc.ID, Status: status, AccountID: account.ID, Limit: 50})
		if e != nil {
			t.Fatal(e)
		}
		return p
	}
	p := byStatus("unrecorded")
	if p.Total != 1 || p.Items[0].AliasID != third.ID {
		t.Fatalf("unknown vs other service: %#v", p)
	}
	p = byStatus("registered")
	if p.Total != 1 || p.Items[0].AliasID != first.ID || p.Items[0].RegistrationCount != 2 {
		t.Fatalf("aggregation: %#v", p)
	}
	if p = byStatus("closed"); p.Total != 0 {
		t.Fatalf("registered must dominate closed: %#v", p)
	}
	if p = byStatus("not_registered"); p.Total != 1 || p.Items[0].AliasID != second.ID {
		t.Fatalf("explicit negative: %#v", p)
	}
	p, err = db.ListServiceMailboxes(ctx, domain.ServiceMailboxFilter{ServiceID: svc.ID, AccountID: account.ID, Query: "100%_", Limit: 1})
	if err != nil || p.Total != 1 || p.Items[0].AliasID != first.ID {
		t.Fatalf("note search: %#v %v", p, err)
	}
	p, err = db.ListServiceMailboxes(ctx, domain.ServiceMailboxFilter{ServiceID: svc.ID, AccountID: account.ID, Query: "usera", Limit: 1})
	if err != nil || p.Total != 1 {
		t.Fatalf("account search: %#v %v", p, err)
	}
	p, err = db.ListServiceMailboxes(ctx, domain.ServiceMailboxFilter{ServiceID: svc.ID, AccountID: account.ID, Limit: 1, Offset: 1})
	if err != nil || p.Total != 3 || len(p.Items) != 1 || !p.HasMore {
		t.Fatalf("pagination: %#v %v", p, err)
	}
	svc.Domain = ""
	svc.Note = ""
	svc.Archived = true
	if _, err = db.UpdateMailService(ctx, svc.ID, svc); err != nil {
		t.Fatal(err)
	}
	if _, err = db.SaveMailboxRegistration(ctx, third.ID, domain.ServiceRegistrationInput{ServiceID: svc.ID, Status: "registered"}); !errors.Is(err, store.ErrMailRegistryConflict) {
		t.Fatalf("archived new: %v", err)
	}
	positive.Status = "pending"
	if _, err = db.SaveMailboxRegistration(ctx, first.ID, positive); err != nil {
		t.Fatalf("archived existing edit: %v", err)
	}
	if err = db.Migrate(ctx); err != nil {
		t.Fatalf("repeat migration: %v", err)
	}
	if err = db.DeleteAlias(ctx, first.ID); err != nil {
		t.Fatal(err)
	}
	reincarnated := createAlias(t, ctx, db, account.ID, strings.ToUpper(first.Address), key("reimport"))
	profile, err = db.GetMailboxProfile(ctx, reincarnated.ID)
	if err != nil || profile.Note == "" || len(profile.Registrations) != 2 {
		t.Fatalf("delete/reimport lost records: %#v %v", profile, err)
	}
	if _, err = db.DeleteMailboxRegistration(ctx, second.ID, profile.Registrations[0].ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("cross mailbox delete: %v", err)
	}
	if _, err = db.DeleteMailboxRegistration(ctx, reincarnated.ID, profile.Registrations[0].ID); err != nil {
		t.Fatal(err)
	}
	final, err := db.GetAlias(ctx, reincarnated.ID)
	if err != nil || !bytes.Equal(final.APIKeyHash, key("reimport")) {
		t.Fatalf("credentials changed: %v", err)
	}
	if _, err = db.SetMailboxNote(ctx, reincarnated.ID, ""); err != nil {
		t.Fatal(err)
	}
	final, err = db.GetAlias(ctx, reincarnated.ID)
	if err != nil || final.Note != "" {
		t.Fatalf("clear note failed: %v", err)
	}
	if _, err = db.ListServiceMailboxes(ctx, domain.ServiceMailboxFilter{ServiceID: svc.ID, Status: "made_up"}); !errors.Is(err, store.ErrMailRegistryInput) {
		t.Fatalf("invalid status: %v", err)
	}
}
