package store

import (
	"context"
	"database/sql"
	"fmt"
	"icloud-api/internal/domain"
	"time"
)

func (s *Store) migratePoolSafety(ctx context.Context, tx *sql.Tx) error {
	for _, q := range []string{
		`CREATE TABLE IF NOT EXISTS pool_client_policies(client_id TEXT PRIMARY KEY REFERENCES pool_clients(id) ON DELETE CASCADE,safe_allocation BOOLEAN NOT NULL DEFAULT TRUE)`,
		`CREATE TABLE IF NOT EXISTS pool_release_reviews(lease_id TEXT PRIMARY KEY REFERENCES pool_leases(id) ON DELETE CASCADE,alias_id BIGINT NOT NULL REFERENCES aliases(id) ON DELETE CASCADE,created_at BIGINT NOT NULL,resolved_at BIGINT)`,
		`CREATE INDEX IF NOT EXISTS pool_release_review_pending ON pool_release_reviews(alias_id) WHERE resolved_at IS NULL`,
		`CREATE TABLE IF NOT EXISTS mailbox_access_windows(alias_id BIGINT PRIMARY KEY REFERENCES aliases(id) ON DELETE CASCADE,visible_after BIGINT NOT NULL)`,
	} {
		if _, err := s.txExecContext(ctx, tx, q); err != nil {
			return fmt.Errorf("migrate pool safety: %w", err)
		}
	}
	return nil
}
func (s *Store) CreateSafePoolClient(ctx context.Context, project string) (PoolClient, string, error) {
	return s.createPoolClient(ctx, project, true)
}
func (s *Store) safePoolClientTx(ctx context.Context, tx *sql.Tx, id string) (bool, error) {
	var safe bool
	err := s.txQueryRowContext(ctx, tx, `SELECT COALESCE((SELECT safe_allocation FROM pool_client_policies WHERE client_id=?),FALSE)`, id).Scan(&safe)
	return safe, err
}
func (s *Store) MailboxContentVisible(ctx context.Context, aliasID int64, at time.Time) (bool, error) {
	var visible bool
	err := s.queryRowContext(ctx, `SELECT NOT EXISTS(SELECT 1 FROM mailbox_access_windows WHERE alias_id=? AND visible_after>?)`, aliasID, timestamp(at)).Scan(&visible)
	return visible, err
}
func (s *Store) ListAccessibleMailboxMessages(ctx context.Context, aliasID int64) ([]domain.ArchivedMailboxMessage, error) {
	messages, err := s.ListArchivedMailboxMessages(ctx, aliasID)
	if err != nil {
		return nil, err
	}
	var after int64
	err = s.queryRowContext(ctx, `SELECT visible_after FROM mailbox_access_windows WHERE alias_id=?`, aliasID).Scan(&after)
	if err == sql.ErrNoRows {
		return messages, nil
	}
	if err != nil {
		return nil, err
	}
	out := []domain.ArchivedMailboxMessage{}
	for _, m := range messages {
		if timestamp(m.InternalDate) >= after {
			out = append(out, m)
		}
	}
	return out, nil
}

type PoolReleaseReview struct {
	LeaseID   string    `json:"lease_id"`
	AliasID   int64     `json:"alias_id"`
	Address   string    `json:"address"`
	Project   string    `json:"project"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *Store) ListPoolReleaseReviews(ctx context.Context) ([]PoolReleaseReview, error) {
	rows, err := s.queryContext(ctx, `SELECT r.lease_id,r.alias_id,a.address,c.project,r.created_at FROM pool_release_reviews r JOIN aliases a ON a.id=r.alias_id JOIN pool_leases l ON l.id=r.lease_id JOIN pool_clients c ON c.id=l.client_id WHERE r.resolved_at IS NULL ORDER BY r.created_at DESC LIMIT 200`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PoolReleaseReview{}
	for rows.Next() {
		var r PoolReleaseReview
		var at int64
		if err := rows.Scan(&r.LeaseID, &r.AliasID, &r.Address, &r.Project, &at); err != nil {
			return nil, err
		}
		r.CreatedAt = timeFromTimestamp(at)
		out = append(out, r)
	}
	return out, rows.Err()
}
func (s *Store) ResolvePoolReleaseReview(ctx context.Context, leaseID string, unused bool) error {
	tx, err := s.poolTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var aliasID int64
	if err = s.txQueryRowContext(ctx, tx, `SELECT alias_id FROM pool_release_reviews WHERE lease_id=? AND resolved_at IS NULL`, leaseID).Scan(&aliasID); err != nil {
		return err
	}
	alias, err := s.getAliasByIDTx(ctx, tx, aliasID)
	if err != nil {
		return err
	}
	if _, err = s.lockAccountVersionForUpdate(ctx, tx, alias.AccountID); err != nil {
		return err
	}
	state := "paused"
	if unused {
		var consumed bool
		if err = s.txQueryRowContext(ctx, tx, `SELECT EXISTS(SELECT 1 FROM pool_leases WHERE lower(trim(address))=lower(trim(?)) AND state IN ('used','retiring','retired'))`, alias.Address).Scan(&consumed); err != nil {
			return err
		}
		if consumed {
			return ErrPoolConflict
		}
		state = "available"
	} else {
		state = "used"
		if _, err = s.txExecContext(ctx, tx, `UPDATE pool_leases SET state='used',updated_at=? WHERE id=? AND state='expired'`, timestamp(s.now()), leaseID); err != nil {
			return err
		}
	}
	if _, err = s.txExecContext(ctx, tx, `UPDATE pool_release_reviews SET resolved_at=? WHERE lease_id=?`, timestamp(s.now()), leaseID); err != nil {
		return err
	}
	if _, err = s.txExecContext(ctx, tx, `UPDATE pool_members SET state=?,updated_at=? WHERE alias_id=?`, state, timestamp(s.now()), aliasID); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Store) revokeSafePoolCredentialsTx(ctx context.Context, tx *sql.Tx, clientID string) error {
	safe, err := s.safePoolClientTx(ctx, tx, clientID)
	if err != nil || !safe {
		return err
	}
	rows, err := s.txQueryContext(ctx, tx, `SELECT al.id,al.account_id FROM aliases al JOIN pool_leases l ON l.alias_id=al.id WHERE l.client_id=? AND l.state IN ('leased','used') ORDER BY al.account_id,al.id`, clientID)
	if err != nil {
		return err
	}
	type target struct{ id, account int64 }
	targets := []target{}
	for rows.Next() {
		var t target
		if err = rows.Scan(&t.id, &t.account); err != nil {
			rows.Close()
			return err
		}
		targets = append(targets, t)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, t := range targets {
		if _, err = s.lockAccountVersionForUpdate(ctx, tx, t.account); err != nil {
			return err
		}
		alias, err := s.getAliasByIDTx(ctx, tx, t.id)
		if err != nil {
			return err
		}
		if err = s.rotatePoolAliasTx(ctx, tx, alias); err != nil {
			return err
		}
	}
	return nil
}
