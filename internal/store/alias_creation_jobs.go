package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

type AliasCreationJobEntry struct {
	AliasID   int64     `json:"alias_id"`
	Address   string    `json:"address"`
	Channel   string    `json:"channel"`
	CreatedAt time.Time `json:"created_at"`
}

type AliasCreationJob struct {
	ID              string                  `json:"id"`
	AccountID       int64                   `json:"account_id"`
	Target          int                     `json:"target"`
	Completed       int                     `json:"completed"`
	Channel         string                  `json:"channel"`
	IntervalSeconds int                     `json:"interval_seconds"`
	Status          string                  `json:"status"`
	LastError       string                  `json:"last_error,omitempty"`
	NextRunAt       *time.Time              `json:"next_run_at,omitempty"`
	Entries         []AliasCreationJobEntry `json:"entries"`
	CreatedAt       time.Time               `json:"created_at"`
	UpdatedAt       time.Time               `json:"updated_at"`
}

var ErrAliasCreationJobConflict = errors.New("alias creation job conflict")

func (s *Store) EnsureAliasCreationJobs(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := s.ensureAliasCreationJobsTx(ctx, tx); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) ensureAliasCreationJobsTx(ctx context.Context, tx *sql.Tx) error {
	_, err := s.txExecContext(ctx, tx, `CREATE TABLE IF NOT EXISTS alias_creation_jobs (
 id TEXT PRIMARY KEY, account_id BIGINT NOT NULL, target INTEGER NOT NULL CHECK(target BETWEEN 1 AND 100),
 completed INTEGER NOT NULL DEFAULT 0, channel TEXT NOT NULL, status TEXT NOT NULL,
 interval_seconds INTEGER NOT NULL DEFAULT 0,
 last_error TEXT NOT NULL DEFAULT '', next_run_at BIGINT, entries_json TEXT NOT NULL DEFAULT '[]',
 created_at BIGINT NOT NULL, updated_at BIGINT NOT NULL,
 FOREIGN KEY(account_id) REFERENCES accounts(id) ON DELETE CASCADE)`)
	if err != nil {
		return fmt.Errorf("create alias creation jobs: %w", err)
	}
	if s.dialect == dialectPostgres {
		_, err = s.txExecContext(ctx, tx, `ALTER TABLE alias_creation_jobs ADD COLUMN IF NOT EXISTS interval_seconds INTEGER NOT NULL DEFAULT 0`)
	} else {
		var exists bool
		exists, err = legacySQLiteColumnExists(ctx, tx, "alias_creation_jobs", "interval_seconds")
		if err == nil && !exists {
			_, err = s.txExecContext(ctx, tx, `ALTER TABLE alias_creation_jobs ADD COLUMN interval_seconds INTEGER NOT NULL DEFAULT 0`)
		}
	}
	if err != nil {
		return fmt.Errorf("migrate creation pacing: %w", err)
	}
	_, err = s.txExecContext(ctx, tx, `CREATE UNIQUE INDEX IF NOT EXISTS alias_creation_jobs_active_account
 ON alias_creation_jobs(account_id) WHERE status IN ('running','waiting')`)
	return err
}

func (s *Store) InterruptAliasCreationJobs(ctx context.Context) error {
	// A waiting paced task has no request in flight and can safely resume.
	// A running task may already have reserved an address upstream.
	_, err := s.execContext(ctx, `UPDATE alias_creation_jobs SET status='interrupted', next_run_at=NULL, updated_at=? WHERE status='running' OR (status='waiting' AND (interval_seconds=0 OR next_run_at IS NULL))`, timestamp(s.now()))
	return err
}

func (s *Store) CreateAliasCreationJob(ctx context.Context, j AliasCreationJob) error {
	if j.CreatedAt.IsZero() {
		j.CreatedAt = s.now()
	}
	if j.UpdatedAt.IsZero() {
		j.UpdatedAt = j.CreatedAt
	}
	b, err := json.Marshal(j.Entries)
	if err != nil {
		return err
	}
	_, err = s.execContext(ctx, `INSERT INTO alias_creation_jobs(id,account_id,target,completed,channel,status,last_error,next_run_at,entries_json,created_at,updated_at,interval_seconds) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, j.ID, j.AccountID, j.Target, j.Completed, j.Channel, j.Status, j.LastError, nullableCreationTimestamp(j.NextRunAt), string(b), timestamp(j.CreatedAt), timestamp(j.UpdatedAt), j.IntervalSeconds)
	if err != nil {
		if _, activeErr := s.GetActiveAliasCreationJob(ctx, j.AccountID); activeErr == nil {
			return ErrAliasCreationJobConflict
		}
		return fmt.Errorf("insert alias creation job: %w", err)
	}
	return nil
}

func (s *Store) SaveAliasCreationJob(ctx context.Context, j AliasCreationJob) error {
	b, err := json.Marshal(j.Entries)
	if err != nil {
		return err
	}
	j.UpdatedAt = s.now()
	result, err := s.execContext(ctx, `UPDATE alias_creation_jobs SET completed=?,status=?,last_error=?,next_run_at=?,entries_json=?,updated_at=? WHERE id=?`, j.Completed, j.Status, j.LastError, nullableCreationTimestamp(j.NextRunAt), string(b), timestamp(j.UpdatedAt), j.ID)
	if err != nil {
		return err
	}
	return requireAffected(result, "alias creation job")
}

func (s *Store) GetLatestAliasCreationJob(ctx context.Context, accountID int64) (AliasCreationJob, error) {
	return s.getAliasCreationJob(ctx, `WHERE account_id=? ORDER BY created_at DESC LIMIT 1`, accountID)
}
func (s *Store) GetActiveAliasCreationJob(ctx context.Context, accountID int64) (AliasCreationJob, error) {
	return s.getAliasCreationJob(ctx, `WHERE account_id=? AND status IN ('running','waiting') LIMIT 1`, accountID)
}
func (s *Store) StopAliasCreationJob(ctx context.Context, accountID int64) error {
	_, err := s.execContext(ctx, `UPDATE alias_creation_jobs SET status='stopped',next_run_at=NULL,updated_at=? WHERE account_id=? AND status IN ('running','waiting')`, timestamp(s.now()), accountID)
	return err
}

const aliasCreationJobColumns = `id,account_id,target,completed,channel,status,last_error,next_run_at,entries_json,created_at,updated_at,interval_seconds`

func scanAliasCreationJob(row rowScanner) (AliasCreationJob, error) {
	var j AliasCreationJob
	var next sql.NullInt64
	var created, updated int64
	var raw string
	err := row.Scan(&j.ID, &j.AccountID, &j.Target, &j.Completed, &j.Channel, &j.Status, &j.LastError, &next, &raw, &created, &updated, &j.IntervalSeconds)
	if errors.Is(err, sql.ErrNoRows) {
		return j, ErrNotFound
	}
	if err != nil {
		return j, err
	}
	if err = json.Unmarshal([]byte(raw), &j.Entries); err != nil {
		return j, err
	}
	if j.Entries == nil {
		j.Entries = []AliasCreationJobEntry{}
	}
	j.CreatedAt, j.UpdatedAt = timeFromTimestamp(created), timeFromTimestamp(updated)
	j.NextRunAt = timePtr(next)
	return j, nil
}

func (s *Store) getAliasCreationJob(ctx context.Context, suffix string, args ...any) (AliasCreationJob, error) {
	return scanAliasCreationJob(s.queryRowContext(ctx, `SELECT `+aliasCreationJobColumns+` FROM alias_creation_jobs `+suffix, args...))
}

func (s *Store) ListResumableAliasCreationJobs(ctx context.Context) ([]AliasCreationJob, error) {
	rows, err := s.queryContext(ctx, `SELECT `+aliasCreationJobColumns+` FROM alias_creation_jobs WHERE status='waiting' AND interval_seconds>0 AND next_run_at IS NOT NULL ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	jobs := []AliasCreationJob{}
	for rows.Next() {
		job, err := scanAliasCreationJob(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}

func nullableCreationTimestamp(t *time.Time) any {
	if t == nil {
		return nil
	}
	return timestamp(*t)
}

// GetAliasCreationJob resolves an operation without depending on latest-task ordering.
func (s *Store) GetAliasCreationJob(ctx context.Context, accountID int64, id string) (AliasCreationJob, error) {
	return s.getAliasCreationJob(ctx, `WHERE account_id=? AND id=?`, accountID, id)
}
