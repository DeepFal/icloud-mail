package syncer

import (
	"context"
	"errors"
	"time"

	"icloud-api/internal/domain"
	mailfetch "icloud-api/internal/mail"
	"icloud-api/internal/store"
)

type historyRepository interface {
	RecoverMailHistoryJobs(context.Context) error
	DueMailHistoryJobs(context.Context) ([]domain.MailHistoryJob, error)
	ClaimMailHistoryJob(context.Context, domain.MailHistoryJob) (bool, error)
	GetMailHistoryJob(context.Context, int64, string) (domain.MailHistoryJob, error)
	ApplyHistoryMailboxBatch(context.Context, domain.Account, []domain.Alias, domain.MailHistoryJob, domain.MailHistoryBatch) error
	FailMailHistoryJob(context.Context, domain.MailHistoryJob, string, bool) error
}
type historyFetcher interface {
	FetchHistoryBatch(context.Context, domain.Account, string, []domain.Alias, domain.MailHistoryJob) (domain.MailHistoryBatch, error)
}

// RunMailHistoryJobs restores read-only work after restart and commits one bounded
// batch per turn, yielding the account execution slot to live mail between batches.
func (m *Manager) RunMailHistoryJobs(ctx context.Context) {
	repo, ok := m.repo.(historyRepository)
	if !ok {
		return
	}
	fetcher, ok := m.fetcher.(historyFetcher)
	if !ok {
		return
	}
	for delay := time.Second; ; delay = min(delay*2, 30*time.Second) {
		if err := repo.RecoverMailHistoryJobs(ctx); err == nil {
			break
		}
		if ctx.Err() != nil {
			return
		}
		m.logger.Warn("恢复历史补取任务暂未完成，稍后重试")
		if !waitForInterval(ctx, delay) {
			return
		}
	}
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		jobs, err := repo.DueMailHistoryJobs(ctx)
		if err != nil && ctx.Err() == nil {
			m.logger.Warn("读取历史补取任务失败", "error", err)
		}
		for _, job := range jobs {
			if ctx.Err() != nil {
				return
			}
			claimed, err := repo.ClaimMailHistoryJob(ctx, job)
			if err != nil || !claimed {
				continue
			}
			job.Status = "running"
			err = m.runMailHistoryBatch(ctx, repo, fetcher, job)
			if err == nil || errors.Is(err, store.ErrMailHistoryConflict) {
				continue
			}
			if ctx.Err() != nil {
				return
			} // persisted running is recovered on next boot
			retry := !errors.Is(err, mailfetch.ErrHistoryMailboxChanged) && !errors.Is(err, store.ErrAliasMailboxSyncStale) && !errors.Is(err, store.ErrAccountDisabled) && !errors.Is(err, domain.ErrIMAPAuthenticationPaused) && !errors.Is(err, store.ErrICloudMailboxRequired)
			// Only fixed diagnoses reach persistence; transport errors can contain server addresses.
			message := "上游读取暂未完成，将按退避时间重试"
			if !retry {
				message = "邮箱配置、认证或UID代次已变化，请处理连接后新建补取任务"
			}
			if errors.Is(err, store.ErrICloudMailboxRequired) {
				message = "历史补取需要 IMAP 收件连接，请切换收件通道后重新提交"
			}
			for {
				if saveErr := repo.FailMailHistoryJob(ctx, job, message, retry); saveErr == nil {
					break
				}
				if ctx.Err() != nil {
					return
				}
				m.logger.Warn("保存历史补取结果暂未完成，稍后重试", "account_id", job.AccountID)
				if !waitForInterval(ctx, 5*time.Second) {
					return
				}
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
func (m *Manager) runMailHistoryBatch(parent context.Context, repo historyRepository, fetcher historyFetcher, job domain.MailHistoryJob) error {
	ctx, cancel := context.WithTimeout(parent, m.syncTimeout)
	defer cancel()
	return m.WithAccountIMAPSlot(ctx, job.AccountID, func() error {
		current, err := repo.GetMailHistoryJob(ctx, job.AccountID, job.ID)
		if err != nil {
			return err
		}
		if current.Status != "running" {
			return store.ErrMailHistoryConflict
		}
		job = current
		account, err := m.repo.GetAccount(ctx, job.AccountID)
		if err != nil {
			return err
		}
		if !account.Enabled {
			return store.ErrAccountDisabled
		}
		if account.MailboxConfigVersion != job.ConfigVersion || account.MailTransportVersion != job.TransportVersion {
			return store.ErrAliasMailboxSyncStale
		}
		transport, err := accountMailTransport(ctx, m.repo, account.ID)
		if err != nil {
			return err
		}
		if transport != "imap" {
			return store.ErrICloudMailboxRequired
		}
		if domain.IsIMAPAuthenticationFailure(account.LastSyncError) {
			return domain.ErrIMAPAuthenticationPaused
		}
		aliases, err := m.repo.ListEnabledAliasesByAccount(ctx, account.ID)
		if err != nil {
			return err
		}
		password, err := m.cipher.Decrypt(account.PasswordCiphertext)
		if err != nil {
			return err
		}
		if err = m.waitForFetchInterval(ctx, account.ID); err != nil {
			return err
		}
		batch, err := fetcher.FetchHistoryBatch(ctx, account, password, aliases, job)
		password = ""
		if err != nil {
			return err
		}
		return repo.ApplyHistoryMailboxBatch(ctx, account, aliases, job, batch)
	})
}

type receivingPreferences interface {
	GetMailboxPreferences(context.Context, int64) (domain.MailboxPreferences, error)
}

func backgroundAccountEnabled(ctx context.Context, repo any, id int64) bool {
	if r, ok := repo.(receivingPreferences); ok {
		p, err := r.GetMailboxPreferences(ctx, id)
		return err == nil && p.BackgroundEnabled
	}
	return true
}
