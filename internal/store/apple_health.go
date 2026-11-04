package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"icloud-api/internal/domain"
)

// Keep each IN clause below SQLite's parameter limit as well as PostgreSQL's.
const appleHealthBatchSize = 500

type AccountAppleHealth struct {
	ConfigEnabled         bool
	KeepAliveStatus       string
	CreationCooldownUntil time.Time
}

// AppleSessionBatchError retains the failing channel and requested ID set so
// a read failure cannot hide usable records from the other session channel.
type AppleSessionBatchError struct {
	Web, Account map[int64]error
}

func (*AppleSessionBatchError) Error() string { return "Apple session batch read failed" }

func appleHealthAccountIDs(ids []int64) []int64 {
	result := make([]int64, 0, len(ids))
	seen := make(map[int64]bool, len(ids))
	for _, id := range ids {
		if id > 0 && !seen[id] {
			seen[id] = true
			result = append(result, id)
		}
	}
	return result
}

func appleHealthIDParams(ids []int64) (string, []any) {
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	return strings.TrimSuffix(strings.Repeat("?,", len(ids)), ","), args
}

// ListAppleSessions reads each persisted channel once for the requested IDs.
// Missing sessions are absent from their map, preserving legacy Web-envelope
// fallback decisions in hmesync without making a query per account.
func (s *Store) ListAppleSessions(ctx context.Context, accountIDs []int64) (web, account map[int64]domain.AppleWebSession, err error) {
	web, account = map[int64]domain.AppleWebSession{}, map[int64]domain.AppleWebSession{}
	batchErr := &AppleSessionBatchError{Web: map[int64]error{}, Account: map[int64]error{}}
	ids := appleHealthAccountIDs(accountIDs)
	for start := 0; start < len(ids); start += appleHealthBatchSize {
		end := min(start+appleHealthBatchSize, len(ids))
		params, args := appleHealthIDParams(ids[start:end])
		for _, channel := range []struct {
			table  string
			items  map[int64]domain.AppleWebSession
			errors map[int64]error
		}{{appleWebSessionTable, web, batchErr.Web}, {appleAccountSessionTable, account, batchErr.Account}} {
			rows, queryErr := s.queryContext(ctx, `SELECT `+appleWebSessionColumns+` FROM `+channel.table+` WHERE account_id IN (`+params+`)`, args...)
			err = consumeAppleHealthRows(rows, queryErr, func(rows *sql.Rows) error {
				record, scanErr := scanAppleWebSession(rows)
				if scanErr == nil {
					channel.items[record.AccountID] = record
				}
				return scanErr
			})
			if err != nil {
				for _, id := range ids[start:end] {
					channel.errors[id] = err
					delete(channel.items, id)
				}
			}
		}
	}
	if len(batchErr.Web) != 0 || len(batchErr.Account) != 0 {
		return web, account, batchErr
	}
	return web, account, nil
}

// ListAccountAppleHealth reads configuration, both keep-alive states and both
// creation cooldowns in three queries per batch, independent of account count.
// Credentials are deliberately excluded from this local health snapshot.
func (s *Store) ListAccountAppleHealth(ctx context.Context, accountIDs []int64) (map[int64]AccountAppleHealth, error) {
	ids := appleHealthAccountIDs(accountIDs)
	result := make(map[int64]AccountAppleHealth, len(ids))
	for _, id := range ids {
		result[id] = AccountAppleHealth{KeepAliveStatus: "disabled"}
	}
	var readErr error
	for start := 0; start < len(ids); start += appleHealthBatchSize {
		end := min(start+appleHealthBatchSize, len(ids))
		params, args := appleHealthIDParams(ids[start:end])
		rows, err := s.queryContext(ctx, `SELECT account_id, enabled FROM account_auto_login_configs WHERE account_id IN (`+params+`)`, args...)
		if err = consumeAppleHealthRows(rows, err, func(rows *sql.Rows) error {
			var id int64
			var enabled bool
			if err := rows.Scan(&id, &enabled); err != nil {
				return err
			}
			item := result[id]
			item.ConfigEnabled = enabled
			if enabled {
				item.KeepAliveStatus = "idle"
			}
			result[id] = item
			return nil
		}); err != nil {
			readErr = errors.Join(readErr, err)
			for _, id := range ids[start:end] {
				item := result[id]
				item.ConfigEnabled, item.KeepAliveStatus = false, "unknown"
				result[id] = item
			}
		}
		rows, err = s.queryContext(ctx, `SELECT account_id, status FROM account_auto_login_states WHERE account_id IN (`+params+`) AND channel IN ('apple_account', 'icloud_web') ORDER BY account_id, channel`, args...)
		if err = consumeAppleHealthRows(rows, err, func(rows *sql.Rows) error {
			var id int64
			var status string
			if err := rows.Scan(&id, &status); err != nil {
				return err
			}
			item := result[id]
			if item.ConfigEnabled && appleKeepAliveRank(status) > appleKeepAliveRank(item.KeepAliveStatus) {
				item.KeepAliveStatus = status
				result[id] = item
			}
			return nil
		}); err != nil {
			readErr = errors.Join(readErr, err)
		}
		rows, err = s.queryContext(ctx, `SELECT account_id, until_at FROM apple_creation_channel_cooldowns WHERE account_id IN (`+params+`) AND channel IN ('apple_account', 'icloud_web')`, args...)
		if err = consumeAppleHealthRows(rows, err, func(rows *sql.Rows) error {
			var id, until int64
			if err := rows.Scan(&id, &until); err != nil {
				return err
			}
			item := result[id]
			if paused := timeFromTimestamp(until); paused.After(item.CreationCooldownUntil) {
				item.CreationCooldownUntil = paused
				result[id] = item
			}
			return nil
		}); err != nil {
			readErr = errors.Join(readErr, err)
		}
	}
	return result, readErr
}

func appleKeepAliveRank(status string) int {
	switch status {
	case "idle":
		return 1
	case "authenticated":
		return 2
	case "logging_in", "waiting_code":
		return 3
	case "retry_wait":
		return 4
	case "needs_action":
		return 5
	default:
		return 0
	}
}

func consumeAppleHealthRows(rows *sql.Rows, err error, consume func(*sql.Rows) error) error {
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		if err := consume(rows); err != nil {
			return err
		}
	}
	return rows.Err()
}
