package httpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"icloud-api/internal/domain"
	"icloud-api/internal/hmesync"
)

type accountHealthFake struct {
	fakeHMESyncService
	web, account hmesync.SessionInfo
}

func (f *accountHealthFake) GetSession(context.Context, int64) (hmesync.SessionInfo, error) {
	return f.web, nil
}
func (f *accountHealthFake) GetAccountSession(context.Context, int64) (hmesync.SessionInfo, error) {
	return f.account, nil
}

type accountHealthItem struct {
	Email string                   `json:"email"`
	Apple *adminAPIAccountAppleDTO `json:"apple"`
}

func accountHealthList(t *testing.T, env *adminAPITestEnv, cookie *http.Cookie) map[string]accountHealthItem {
	t.Helper()
	response := env.request(t, http.MethodGet, "/admin/api/v1/accounts", nil, "", []*http.Cookie{cookie}, "")
	if response.Code != http.StatusOK {
		t.Fatalf("list accounts status=%d", response.Code)
	}
	var payload struct {
		Data struct {
			Items []accountHealthItem `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	result := map[string]accountHealthItem{}
	for _, item := range payload.Data.Items {
		result[item.Email] = item
	}
	return result
}

func TestAdminAPIAccountListReportsLocalAppleHealth(t *testing.T) {
	env := newAdminAPITestEnv(t)
	cookie, csrf, _ := env.createSession(t, "account-health-admin", "unused-password")
	healthy := time.Now().Add(time.Hour)
	env.server.SetHMESyncService(&accountHealthFake{
		web:     hmesync.SessionInfo{Status: hmesync.StatusAuthenticated, ExpiresAt: &healthy},
		account: hmesync.SessionInfo{Status: hmesync.StatusAuthenticated, Health: "retry_wait"},
	})
	account := autoLoginAPITestCreate(t, env, cookie, csrf, "health-keepalive@icloud.com", map[string]any{
		"enabled": true, "region": "global", "password": autoLoginAPITestPassword, "code_url": autoLoginAPITestCodeURL,
	})
	adminAPITestCreateAccount(t, env, "health-plain@icloud.com")
	ctx := context.Background()
	config, err := env.store.GetAccountAutoLoginConfig(ctx, account.ID)
	if err != nil {
		t.Fatal(err)
	}

	// Retrying keep-alive and a retrying Account channel are not actionable.
	items := accountHealthList(t, env, cookie)
	apple := items["health-keepalive@icloud.com"].Apple
	if apple == nil || apple.Web != "healthy" || apple.Account != "retry_wait" || apple.KeepAlive != "idle" || apple.NeedsAttention || apple.CreationCooldownUntil != nil {
		t.Fatalf("unexpected apple health: %+v", apple)
	}
	if got := items["health-plain@icloud.com"].Apple; got == nil || got.KeepAlive != "disabled" || got.NeedsAttention {
		t.Fatalf("plain account health = %+v", got)
	}

	// Keep-alive that gave up and a creation cooldown are reported from local rows.
	if err := env.store.SaveAccountAutoLoginState(ctx, domain.AccountAutoLoginState{
		AccountID: account.ID, Channel: domain.AutoLoginChannelICloudWeb, ConfigRevision: config.Revision, Status: "needs_action",
	}); err != nil {
		t.Fatal(err)
	}
	until := time.Now().Add(30 * time.Minute).UTC().Truncate(time.Second)
	if err := env.store.PauseAppleChannelCreation(ctx, account.ID, domain.AutoLoginChannelAppleAccount, until); err != nil {
		t.Fatal(err)
	}
	apple = accountHealthList(t, env, cookie)["health-keepalive@icloud.com"].Apple
	if apple.KeepAlive != "needs_action" || !apple.NeedsAttention || apple.CreationCooldownUntil == nil || *apple.CreationCooldownUntil != until.Format(time.RFC3339) {
		t.Fatalf("keep-alive/cooldown not reported: %+v", apple)
	}
}

func TestAdminAPIAccountListFlagsExpiredAppleSessionOnly(t *testing.T) {
	env := newAdminAPITestEnv(t)
	cookie, _, _ := env.createSession(t, "account-health-expired-admin", "unused-password")
	adminAPITestCreateAccount(t, env, "health-expired@icloud.com")
	cases := []struct {
		name         string
		web, account string
		wantWeb      string
		wantAccount  string
		attention    bool
	}{
		{"web expired", hmesync.StatusExpired, hmesync.StatusAuthenticated, "expired", "healthy", true},
		{"account expired", hmesync.StatusAuthenticated, hmesync.StatusExpired, "healthy", "expired", true},
		{"never connected", hmesync.StatusLoginRequired, hmesync.StatusLoginRequired, "not_connected", "not_connected", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env.server.SetHMESyncService(&accountHealthFake{
				web:     hmesync.SessionInfo{Status: tc.web},
				account: hmesync.SessionInfo{Status: tc.account, Health: "healthy"},
			})
			apple := accountHealthList(t, env, cookie)["health-expired@icloud.com"].Apple
			if apple.Web != tc.wantWeb || apple.Account != tc.wantAccount || apple.NeedsAttention != tc.attention {
				t.Fatalf("apple health = %+v", apple)
			}
		})
	}
}

func TestAdminAPIAccountListOmitsAppleHealthWithoutAppleService(t *testing.T) {
	env := newAdminAPITestEnv(t)
	cookie, _, _ := env.createSession(t, "account-health-none-admin", "unused-password")
	adminAPITestCreateAccount(t, env, "health-none@icloud.com")
	if got := accountHealthList(t, env, cookie)["health-none@icloud.com"].Apple; got != nil {
		t.Fatalf("apple health without service = %+v", got)
	}
}
