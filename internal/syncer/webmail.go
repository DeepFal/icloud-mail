package syncer

import (
	"context"
	"errors"
	"sort"
	"time"

	"icloud-api/internal/domain"
	"icloud-api/internal/store"
)

type WebMailFetcher func(context.Context, domain.Account, domain.Alias, []domain.Alias) (domain.MailboxSyncResult, error)

type mailTransportRepository interface {
	GetAccountMailTransport(context.Context, int64) (string, error)
}

type webMailRepository interface {
	ApplyAliasWebMailboxSync(context.Context, time.Time, domain.Alias, domain.MailboxSyncResult, time.Time) error
}

// SetWebMailFetcher installs the alternative receiver before workers start.
func (m *Manager) SetWebMailFetcher(fetch WebMailFetcher) { m.webMailFetcher = fetch }

func accountMailTransport(ctx context.Context, repo any, accountID int64) (string, error) {
	if r, ok := repo.(mailTransportRepository); ok {
		return r.GetAccountMailTransport(ctx, accountID)
	}
	return "imap", nil
}

// syncWebMailboxBatch runs a bounded round-robin batch for non-IMAP accounts.
// Callers already hold the same per-account receiver slot as on-demand reads.
func (m *Manager) syncWebMailboxBatch(ctx context.Context, accountID int64) error {
	if m.webMailFetcher == nil {
		return ErrSyncDeferred
	}
	account, err := m.repo.GetAccount(ctx, accountID)
	if err != nil {
		return err
	}
	if !account.Enabled {
		return store.ErrAccountDisabled
	}
	aliases, err := m.repo.ListEnabledAliasesByAccount(ctx, accountID)
	if err != nil {
		return err
	}
	sort.Slice(aliases, func(i, j int) bool { return aliases[i].ID < aliases[j].ID })
	m.webReceiveMu.Lock()
	last := m.webReceivePositions[accountID]
	m.webReceiveMu.Unlock()
	start := sort.Search(len(aliases), func(i int) bool { return aliases[i].ID > last })
	if start == len(aliases) {
		start = 0
	}
	end := min(start+16, len(aliases))
	for _, alias := range aliases[start:end] {
		if err = m.waitForFetchInterval(ctx, accountID); err != nil {
			return err
		}
		result, fetchErr := m.webMailFetcher(ctx, account, alias, aliases)
		if fetchErr != nil {
			if errors.Is(fetchErr, store.ErrAliasMailboxSyncStale) {
				return ErrSyncDeferred
			}
			failures := newFailureRecorder(m, ctx, accountID, account.UpdatedAt)
			failures.account = &account
			failures.record(fetchErr)
			failures.close()
			return fetchErr
		}
		observed, ok := m.repo.(interface {
			ApplyObservedAliasMailboxSync(context.Context, domain.Account, domain.Alias, domain.MailboxSyncResult, time.Time, bool) error
		})
		if !ok {
			return errors.New("observed web mail persistence unavailable")
		}
		if err = observed.ApplyObservedAliasMailboxSync(ctx, account, alias, result, time.Now().UTC(), true); err != nil {
			return err
		}
		m.webReceiveMu.Lock()
		m.webReceivePositions[accountID] = alias.ID
		m.webReceiveMu.Unlock()
	}
	if end < len(aliases) {
		return ErrSyncPending
	}
	m.webReceiveMu.Lock()
	delete(m.webReceivePositions, accountID)
	m.webReceiveMu.Unlock()
	return nil
}
