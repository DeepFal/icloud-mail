package syncer

import (
	"context"
	"errors"
	"icloud-api/internal/domain"
	"icloud-api/internal/store"
	"sync"
	"testing"
	"time"
)

type historyRunRepo struct {
	*fakeRepo
	historyMu    sync.Mutex
	job          domain.MailHistoryJob
	recoverCalls int
	committed    chan struct{}
}

func (r *historyRunRepo) RecoverMailHistoryJobs(context.Context) error {
	r.historyMu.Lock()
	defer r.historyMu.Unlock()
	r.recoverCalls++
	if r.recoverCalls == 1 {
		return errors.New("temporary DB outage")
	}
	r.job.Status = "queued"
	return nil
}
func (r *historyRunRepo) DueMailHistoryJobs(context.Context) ([]domain.MailHistoryJob, error) {
	r.historyMu.Lock()
	defer r.historyMu.Unlock()
	if r.job.Status == "queued" {
		return []domain.MailHistoryJob{r.job}, nil
	}
	return nil, nil
}
func (r *historyRunRepo) ClaimMailHistoryJob(_ context.Context, j domain.MailHistoryJob) (bool, error) {
	r.historyMu.Lock()
	defer r.historyMu.Unlock()
	if r.job.Status != "queued" {
		return false, nil
	}
	r.job.Status = "running"
	return true, nil
}
func (r *historyRunRepo) GetMailHistoryJob(context.Context, int64, string) (domain.MailHistoryJob, error) {
	r.historyMu.Lock()
	defer r.historyMu.Unlock()
	return r.job, nil
}
func (r *historyRunRepo) ApplyHistoryMailboxBatch(_ context.Context, _ domain.Account, _ []domain.Alias, _ domain.MailHistoryJob, b domain.MailHistoryBatch) error {
	r.historyMu.Lock()
	defer r.historyMu.Unlock()
	if r.job.Status != "running" {
		return store.ErrMailHistoryConflict
	}
	r.job.LastUID = b.LastUID
	r.job.Status = "completed"
	close(r.committed)
	return nil
}
func (r *historyRunRepo) FailMailHistoryJob(_ context.Context, _ domain.MailHistoryJob, _ string, _ bool) error {
	r.historyMu.Lock()
	defer r.historyMu.Unlock()
	r.job.Status = "failed"
	return nil
}

type historyRunFetcher struct{ fetcherFunc }

func (historyRunFetcher) FetchHistoryBatch(context.Context, domain.Account, string, []domain.Alias, domain.MailHistoryJob) (domain.MailHistoryBatch, error) {
	return domain.MailHistoryBatch{UIDValidity: 8, LastUID: 12, TargetUID: 12, Done: true}, nil
}
func TestMailHistoryWorkerRecoversTransientStartupFailureAndResumes(t *testing.T) {
	account := domain.Account{ID: 1, Enabled: true, UpdatedAt: time.Now()}
	r := &historyRunRepo{fakeRepo: newFakeRepo(account), job: domain.MailHistoryJob{ID: "restored-history-job", AccountID: 1, Status: "running"}, committed: make(chan struct{})}
	m := New(r, cipherFunc(fixedCipher), historyRunFetcher{}, discardLogger(), time.Minute, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() { defer close(done); m.RunMailHistoryJobs(ctx) }()
	select {
	case <-r.committed:
	case <-time.After(4 * time.Second):
		t.Fatal("history worker did not recover and resume")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("history worker did not stop")
	}
	r.historyMu.Lock()
	defer r.historyMu.Unlock()
	if r.recoverCalls != 2 || r.job.Status != "completed" || r.job.LastUID != 12 {
		t.Fatalf("recovery state calls=%d job=%#v", r.recoverCalls, r.job)
	}
}
