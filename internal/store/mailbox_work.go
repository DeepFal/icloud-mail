package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"icloud-api/internal/domain"
)

var ErrMailHistoryConflict = errors.New("history task already exists or changed")
var ErrMailHistoryInput = errors.New("invalid history request")

func (s *Store) migrateMailboxWork(ctx context.Context, tx *sql.Tx) error {
	if err := s.ensureAliasCreationJobsTx(ctx, tx); err != nil {
		return err
	}
	for _, q := range []string{
		`CREATE TABLE IF NOT EXISTS mailbox_preferences(account_id BIGINT PRIMARY KEY REFERENCES accounts(id) ON DELETE CASCADE, background_enabled BOOLEAN NOT NULL DEFAULT TRUE,history_days INTEGER NOT NULL DEFAULT 7 CHECK(history_days BETWEEN 1 AND 3650))`,
		`CREATE TABLE IF NOT EXISTS mail_history_jobs(id TEXT PRIMARY KEY,account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,status TEXT NOT NULL CHECK(status IN ('queued','running','waiting','paused','completed','failed','cancelled')),since_at BIGINT NOT NULL,until_at BIGINT NOT NULL,uid_validity BIGINT NOT NULL DEFAULT 0,last_uid BIGINT NOT NULL DEFAULT 0,target_uid BIGINT NOT NULL DEFAULT 0,initialized BOOLEAN NOT NULL DEFAULT FALSE,scanned_count BIGINT NOT NULL DEFAULT 0,archived_count BIGINT NOT NULL DEFAULT 0,failures INTEGER NOT NULL DEFAULT 0,last_error TEXT NOT NULL DEFAULT '',next_run_at BIGINT,config_version BIGINT NOT NULL,transport_version BIGINT NOT NULL,created_at BIGINT NOT NULL,updated_at BIGINT NOT NULL)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS mail_history_one_active_account ON mail_history_jobs(account_id) WHERE status IN ('queued','running','waiting','paused')`,
		`CREATE INDEX IF NOT EXISTS mail_history_due ON mail_history_jobs(status,next_run_at,created_at)`,
	} {
		if _, err := s.txExecContext(ctx, tx, q); err != nil {
			return fmt.Errorf("migrate mailbox work: %w", err)
		}
	}
	return nil
}
func (s *Store) GetMailboxPreferences(ctx context.Context, id int64) (domain.MailboxPreferences, error) {
	p := domain.MailboxPreferences{AccountID: id, BackgroundEnabled: true, HistoryDays: 7}
	if _, err := s.GetAccount(ctx, id); err != nil {
		return p, err
	}
	err := s.queryRowContext(ctx, `SELECT background_enabled,history_days FROM mailbox_preferences WHERE account_id=?`, id).Scan(&p.BackgroundEnabled, &p.HistoryDays)
	if errors.Is(err, sql.ErrNoRows) {
		err = nil
	}
	return p, err
}
func (s *Store) SetMailboxPreferences(ctx context.Context, p domain.MailboxPreferences) (domain.MailboxPreferences, error) {
	if p.AccountID < 1 || p.HistoryDays < 1 || p.HistoryDays > 3650 {
		return p, ErrMailHistoryInput
	}
	if _, err := s.GetAccount(ctx, p.AccountID); err != nil {
		return p, err
	}
	_, err := s.execContext(ctx, `INSERT INTO mailbox_preferences(account_id,background_enabled,history_days) VALUES(?,?,?) ON CONFLICT(account_id) DO UPDATE SET background_enabled=excluded.background_enabled,history_days=excluded.history_days`, p.AccountID, p.BackgroundEnabled, p.HistoryDays)
	return p, err
}

const historyColumns = `id,account_id,status,since_at,until_at,uid_validity,last_uid,target_uid,initialized,scanned_count,archived_count,failures,last_error,next_run_at,config_version,transport_version,created_at,updated_at`

func scanHistoryJob(r rowScanner) (domain.MailHistoryJob, error) {
	var j domain.MailHistoryJob
	var since, until, created, updated, valid, last, target int64
	var next sql.NullInt64
	err := r.Scan(&j.ID, &j.AccountID, &j.Status, &since, &until, &valid, &last, &target, &j.Initialized, &j.ScannedCount, &j.ArchivedCount, &j.Failures, &j.LastError, &next, &j.ConfigVersion, &j.TransportVersion, &created, &updated)
	if err != nil {
		return j, err
	}
	if valid < 0 || valid > int64(^uint32(0)) || last < 0 || last > int64(^uint32(0)) || target < 0 || target > int64(^uint32(0)) {
		return j, ErrMailHistoryInput
	}
	j.UIDValidity, j.LastUID, j.TargetUID = uint32(valid), uint32(last), uint32(target)
	j.Since = timeFromTimestamp(since)
	j.Until = timeFromTimestamp(until)
	j.CreatedAt = timeFromTimestamp(created)
	j.UpdatedAt = timeFromTimestamp(updated)
	j.NextRunAt = timePtr(next)
	return j, nil
}
func (s *Store) GetMailHistoryJob(ctx context.Context, accountID int64, id string) (domain.MailHistoryJob, error) {
	return scanHistoryJob(s.queryRowContext(ctx, `SELECT `+historyColumns+` FROM mail_history_jobs WHERE account_id=? AND id=?`, accountID, id))
}
func (s *Store) LatestMailHistoryJob(ctx context.Context, id int64) (domain.MailHistoryJob, error) {
	return scanHistoryJob(s.queryRowContext(ctx, `SELECT `+historyColumns+` FROM mail_history_jobs WHERE account_id=? ORDER BY CASE WHEN status IN ('queued','running','waiting','paused') THEN 0 ELSE 1 END,created_at DESC LIMIT 1`, id))
}
func (s *Store) ListMailHistoryJobs(ctx context.Context, limit int) ([]domain.MailHistoryJob, error) {
	if limit < 1 || limit > 200 {
		limit = 100
	}
	rows, err := s.queryContext(ctx, `SELECT `+historyColumns+` FROM mail_history_jobs ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.MailHistoryJob{}
	for rows.Next() {
		j, e := scanHistoryJob(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, j)
	}
	return out, rows.Err()
}
func (s *Store) CreateMailHistoryJob(ctx context.Context, account domain.Account, id string, days int) (domain.MailHistoryJob, error) {
	if len(id) < 16 || len(id) > 128 || strings.Trim(id, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_") != "" || days < 1 || days > 3650 {
		return domain.MailHistoryJob{}, ErrMailHistoryInput
	}
	if old, err := s.GetMailHistoryJob(ctx, account.ID, id); err == nil {
		if old.Until.Sub(old.Since) != time.Duration(days)*24*time.Hour {
			return old, ErrMailHistoryConflict
		}
		return old, nil
	} else if !errors.Is(err, ErrNotFound) {
		return old, err
	}
	now := s.now()
	j := domain.MailHistoryJob{ID: id, AccountID: account.ID, Status: "queued", Since: now.Add(-time.Duration(days) * 24 * time.Hour), Until: now, ConfigVersion: account.MailboxConfigVersion, TransportVersion: account.MailTransportVersion, CreatedAt: now, UpdatedAt: now}
	_, err := s.execContext(ctx, `INSERT INTO mail_history_jobs(id,account_id,status,since_at,until_at,config_version,transport_version,created_at,updated_at) VALUES(?,?,'queued',?,?,?,?,?,?)`, id, account.ID, timestamp(j.Since), timestamp(j.Until), j.ConfigVersion, j.TransportVersion, timestamp(now), timestamp(now))
	if err != nil {
		if old, e := s.GetMailHistoryJob(ctx, account.ID, id); e == nil && old.Until.Sub(old.Since) == time.Duration(days)*24*time.Hour {
			return old, nil
		}
		if isUniqueConstraintError(err) {
			return j, ErrMailHistoryConflict
		}
		return j, fmt.Errorf("create history task: %w", err)
	}
	return j, nil
}
func (s *Store) RecoverMailHistoryJobs(ctx context.Context) error {
	_, err := s.execContext(ctx, `UPDATE mail_history_jobs SET status='queued',updated_at=? WHERE status='running'`, timestamp(s.now()))
	return err
}
func (s *Store) DueMailHistoryJobs(ctx context.Context) ([]domain.MailHistoryJob, error) {
	rows, err := s.queryContext(ctx, `SELECT `+historyColumns+` FROM mail_history_jobs WHERE status IN ('queued','waiting') AND (next_run_at IS NULL OR next_run_at<=?) ORDER BY created_at LIMIT 8`, timestamp(s.now()))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.MailHistoryJob{}
	for rows.Next() {
		j, e := scanHistoryJob(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, j)
	}
	return out, rows.Err()
}
func (s *Store) ClaimMailHistoryJob(ctx context.Context, j domain.MailHistoryJob) (bool, error) {
	res, err := s.execContext(ctx, `UPDATE mail_history_jobs SET status='running',next_run_at=NULL,updated_at=? WHERE id=? AND status=?`, timestamp(s.now()), j.ID, j.Status)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}
func (s *Store) ActMailHistoryJob(ctx context.Context, accountID int64, id, action string) (domain.MailHistoryJob, error) {
	j, err := s.GetMailHistoryJob(ctx, accountID, id)
	if err != nil {
		return j, err
	}
	next := ""
	switch action {
	case "pause":
		if j.Status == "queued" || j.Status == "running" || j.Status == "waiting" {
			next = "paused"
		}
	case "cancel":
		if j.Status != "completed" && j.Status != "cancelled" {
			next = "cancelled"
		}
	case "resume":
		if j.Status == "paused" || j.Status == "failed" {
			next = "queued"
		}
	default:
		return j, ErrMailHistoryInput
	}
	if next == "" {
		return j, ErrMailHistoryConflict
	}
	res, err := s.execContext(ctx, `UPDATE mail_history_jobs SET status=?,last_error='',failures=0,next_run_at=NULL,updated_at=? WHERE id=? AND account_id=? AND status=?`, next, timestamp(s.now()), id, accountID, j.Status)
	if err != nil {
		return j, err
	}
	n, _ := res.RowsAffected()
	if n != 1 {
		return j, ErrMailHistoryConflict
	}
	return s.GetMailHistoryJob(ctx, accountID, id)
}
func (s *Store) FailMailHistoryJob(ctx context.Context, j domain.MailHistoryJob, message string, retry bool) error {
	status := "failed"
	var next any
	if retry && j.Failures < 8 {
		status = "waiting"
		next = timestamp(s.now().Add(min(time.Duration(1<<min(j.Failures, 6))*5*time.Second, 5*time.Minute)))
	}
	if len(message) > 800 {
		message = message[:800]
	}
	_, err := s.execContext(ctx, `UPDATE mail_history_jobs SET status=?,failures=failures+1,last_error=?,next_run_at=?,updated_at=? WHERE id=? AND status='running'`, status, message, next, timestamp(s.now()), j.ID)
	return err
}

// ApplyHistoryMailboxBatch publishes archive and durable task position atomically.
// It deliberately never touches imap_sync_states or legacy consumption/Seen.
func (s *Store) ApplyHistoryMailboxBatch(ctx context.Context, account domain.Account, aliases []domain.Alias, j domain.MailHistoryJob, b domain.MailHistoryBatch) error {
	defer s.cleanupArchiveInputs(b.Messages)
	unlock := s.lockMailArchiveAccount(account.ID)
	defer unlock()
	staged, err := s.stageArchiveMessages(ctx, b.Messages, true)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			s.cleanupPublishedArchive(staged)
		}
	}()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = s.lockAccountVersionForUpdate(ctx, tx, account.ID); err != nil {
		return err
	}
	if err = s.checkMailboxObservationTx(ctx, tx, account); err != nil {
		return err
	}
	current, err := scanHistoryJob(s.txQueryRowContext(ctx, tx, `SELECT `+historyColumns+` FROM mail_history_jobs WHERE id=? AND account_id=?`, j.ID, account.ID))
	if err != nil {
		return err
	}
	if current.Status != "running" || current.LastUID != j.LastUID || current.Initialized != j.Initialized {
		return ErrMailHistoryConflict
	}
	if current.ConfigVersion != account.MailboxConfigVersion || current.TransportVersion != account.MailTransportVersion {
		return ErrAliasMailboxSyncStale
	}
	_, filtered, err := s.filterMailboxRecipientsTx(ctx, tx, account.ID, aliases, domain.MailboxSyncResult{ArchivedMessages: b.Messages})
	if err != nil {
		return err
	}
	kept := map[string]bool{}
	newCount := int64(0)
	for _, msg := range filtered.ArchivedMessages {
		kept[archiveMessageKey(msg)] = true
		var exists int
		if err = s.txQueryRowContext(ctx, tx, `SELECT COUNT(*) FROM archived_messages WHERE account_id=? AND uid_validity=? AND upstream_uid=?`, account.ID, int64(msg.UIDValidity), int64(msg.UID)).Scan(&exists); err != nil {
			return err
		}
		if exists == 0 {
			newCount++
		}
	}
	discarded := map[string]stagedArchiveContent{}
	for key, content := range staged {
		if !kept[key] {
			discarded[key] = content
			delete(staged, key)
		}
	}
	defer s.cleanupPublishedArchive(discarded)
	if err = s.persistArchivedMessagesTx(ctx, tx, filtered.ArchivedMessages, staged, s.now()); err != nil {
		return err
	}
	status := "queued"
	if b.Done {
		status = "completed"
	}
	result, err := s.txExecContext(ctx, tx, `UPDATE mail_history_jobs SET status=?,uid_validity=?,last_uid=?,target_uid=?,initialized=TRUE,scanned_count=scanned_count+?,archived_count=archived_count+?,failures=0,last_error='',updated_at=? WHERE id=? AND status='running'`, status, int64(b.UIDValidity), int64(b.LastUID), int64(b.TargetUID), b.Scanned, newCount, timestamp(s.now()), j.ID)
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n != 1 {
		return ErrMailHistoryConflict
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	committed = true
	return s.EnforceMailArchiveLimit(ctx)
}
