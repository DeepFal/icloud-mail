package mail

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	imap "github.com/emersion/go-imap/v2"
	"icloud-api/internal/domain"
)

var ErrHistoryMailboxChanged = errors.New("mailbox UIDVALIDITY changed during history retrieval")

// FetchHistoryBatch pins a finite mailbox UID snapshot and a precise date range.
// It never advances the live cursor or alters upstream flags.
func (f *Fetcher) FetchHistoryBatch(ctx context.Context, account domain.Account, password string, aliases []domain.Alias, job domain.MailHistoryJob) (out domain.MailHistoryBatch, retErr error) {
	if job.Since.IsZero() || !job.Since.Before(job.Until) {
		return out, errors.New("invalid history time range")
	}
	if err := validateIMAPAccount(account, password); err != nil {
		return out, err
	}
	settings := f.settings()
	addresses, err := prepareAliases(account, aliases, settings.maxAliases)
	if err != nil {
		return out, err
	}
	host, address, username, err := accountEndpoint(account)
	if err != nil {
		return out, err
	}
	client, _, release, err := acquireArchiveConnection(ctx, account.ID, address, host, username, password, settings)
	if err != nil {
		return out, err
	}
	defer func() { release(retErr == nil) }()
	mailbox, err := client.Select("INBOX", &imap.SelectOptions{ReadOnly: true}).Wait()
	if err != nil {
		return out, err
	}
	if mailbox == nil || mailbox.UIDValidity == 0 || mailbox.UIDNext == 0 {
		return out, errors.New("invalid history mailbox state")
	}
	out.UIDValidity = mailbox.UIDValidity
	if job.Initialized {
		if job.UIDValidity != mailbox.UIDValidity {
			return out, ErrHistoryMailboxChanged
		}
		out.LastUID, out.TargetUID = job.LastUID, job.TargetUID
	} else {
		out.TargetUID = uint32(mailbox.UIDNext - 1)
		out.LastUID, err = establishActiveArchiveCursor(ctx, client, 0, out.TargetUID, job.Since)
		if err != nil {
			return out, fmt.Errorf("locate history window: %w", err)
		}
	}
	if out.LastUID >= out.TargetUID || len(addresses) == 0 {
		out.Done = true
		return out, nil
	}
	uids, more, through, err := discoverArchiveUIDs(client, out.LastUID, out.TargetUID, settings.maxIncrementalCandidates)
	if err != nil {
		return out, err
	}
	candidates, err := fetchArchiveCandidateHeaders(client, uids, addresses, account, settings)
	if err != nil {
		return out, err
	}
	eligible := make([]archiveCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if !candidate.internalDate.Before(job.Since) && !candidate.internalDate.After(job.Until) {
			eligible = append(eligible, candidate)
		}
	}
	messages, err := fetchArchivedMessages(client, eligible, account.ID, mailbox.UIDValidity, time.Now().UTC(), settings)
	if err != nil {
		return out, err
	}
	defer func() {
		if retErr != nil {
			for _, m := range messages {
				if m.RawMIMEPath != "" {
					_ = os.Remove(m.RawMIMEPath)
				}
			}
		}
	}()
	if err = ctx.Err(); err != nil {
		return out, err
	}
	out.Messages = messages
	out.LastUID = through
	out.Scanned = len(uids)
	out.Done = !more
	return out, nil
}
