package syncer

import (
	"context"
	"icloud-api/internal/domain"
	"sync"
	"testing"
	"time"
)

type backgroundPreferenceRepo struct {
	*fakeRepo
	enabled map[int64]bool
}

func (r *backgroundPreferenceRepo) GetMailboxPreferences(_ context.Context, id int64) (domain.MailboxPreferences, error) {
	return domain.MailboxPreferences{AccountID: id, BackgroundEnabled: r.enabled[id], HistoryDays: 7}, nil
}
func TestBackgroundReceivingRunsWithoutHTTPAndHonorsPreferences(t *testing.T) {
	now := time.Now()
	r := &backgroundPreferenceRepo{fakeRepo: newFakeRepo(domain.Account{ID: 1, Enabled: true, UpdatedAt: now}, domain.Account{ID: 2, Enabled: true, UpdatedAt: now}), enabled: map[int64]bool{1: false, 2: true}}
	var mu sync.Mutex
	called := []int64{}
	f := fetcherFunc(func(_ context.Context, a domain.Account, _ string, _ []domain.Alias, _ *domain.IMAPSyncState) (domain.MailboxSyncResult, error) {
		mu.Lock()
		called = append(called, a.ID)
		mu.Unlock()
		return domain.MailboxSyncResult{State: domain.IMAPSyncState{AccountID: a.ID, UIDValidity: 1, LastUID: 1, UpdatedAt: now}, Reset: true}, nil
	})
	m := New(r, cipherFunc(fixedCipher), f, discardLogger(), time.Minute, 2)
	m.syncAllRound(context.Background(), nil)
	if len(called) != 1 || called[0] != 2 {
		t.Fatalf("background fetched paused accounts or required HTTP: %v", called)
	}
	if len(r.applyCalls()) != 1 || r.applyCalls()[0].accountID != 2 {
		t.Fatalf("background did not publish enabled account: %#v", r.applyCalls())
	}
	r.enabled[1] = true
	r.enabled[2] = false
	called = nil
	m.syncAllRound(context.Background(), nil)
	if len(called) != 1 || called[0] != 1 {
		t.Fatalf("changed receiving preferences not honored: %v", called)
	}
}
