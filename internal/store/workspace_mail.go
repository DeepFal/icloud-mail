package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"icloud-api/internal/domain"
)

var ErrWorkspaceMailInput = errors.New("invalid workspace mail request")

func (s *Store) migrateWorkspaceMail(ctx context.Context, tx *sql.Tx) error {
	_, err := s.txExecContext(ctx, tx, `CREATE TABLE IF NOT EXISTS workspace_mail_states (
		admin_id BIGINT NOT NULL REFERENCES admins(id) ON DELETE CASCADE,
		alias_id BIGINT NOT NULL,
		message_id BIGINT NOT NULL,
		read BOOLEAN NOT NULL DEFAULT FALSE,
		starred BOOLEAN NOT NULL DEFAULT FALSE,
		archived BOOLEAN NOT NULL DEFAULT FALSE,
		deleted BOOLEAN NOT NULL DEFAULT FALSE,
		updated_at BIGINT NOT NULL,
		PRIMARY KEY(admin_id, alias_id, message_id),
		FOREIGN KEY(alias_id, message_id) REFERENCES alias_messages(alias_id, message_id) ON DELETE CASCADE
	)`)
	if err != nil {
		return fmt.Errorf("create workspace mail states: %w", err)
	}
	_, err = s.txExecContext(ctx, tx, `CREATE INDEX IF NOT EXISTS workspace_mail_states_admin_updated_idx ON workspace_mail_states(admin_id, updated_at)`)
	return err
}

func workspaceLike(value string) string {
	return "%" + strings.NewReplacer("!", "!!", "%", "!%", "_", "!_").Replace(strings.ToLower(value)) + "%"
}

func workspaceWhere(f domain.WorkspaceMailFilter) (string, []any) {
	w := []string{"1=1"}
	a := []any{}
	if f.AccountID > 0 {
		w = append(w, "al.account_id=?")
		a = append(a, f.AccountID)
	}
	if f.AliasID > 0 {
		w = append(w, "am.alias_id=?")
		a = append(a, f.AliasID)
	}
	if f.GroupID > 0 {
		w = append(w, "al.group_id=?")
		a = append(a, f.GroupID)
	}
	if q := strings.TrimSpace(f.Query); q != "" {
		p := workspaceLike(q)
		w = append(w, "(LOWER(m.subject) LIKE ? ESCAPE '!' OR LOWER(m.from_json) LIKE ? ESCAPE '!' OR LOWER(al.address) LIKE ? ESCAPE '!' OR LOWER(al.label) LIKE ? ESCAPE '!' OR EXISTS (SELECT 1 FROM mailbox_notes mn WHERE mn.address=al.address AND LOWER(mn.note) LIKE ? ESCAPE '!'))")
		a = append(a, p, p, p, p, p)
	}
	switch strings.ToLower(strings.TrimSpace(f.Folder)) {
	case "inbox":
		w = append(w, "COALESCE(ws.archived,FALSE)=FALSE AND COALESCE(ws.deleted,FALSE)=FALSE")
	case "archived":
		w = append(w, "COALESCE(ws.archived,FALSE)=TRUE AND COALESCE(ws.deleted,FALSE)=FALSE")
	case "trash":
		w = append(w, "COALESCE(ws.deleted,FALSE)=TRUE")
	case "", "all":
		w = append(w, "COALESCE(ws.deleted,FALSE)=FALSE")
	default:
		w = append(w, "0=1")
	}
	if f.Unread {
		w = append(w, "COALESCE(ws.read,FALSE)=FALSE")
	}
	if f.Starred {
		w = append(w, "COALESCE(ws.starred,FALSE)=TRUE")
	}
	return strings.Join(w, " AND "), a
}

const workspaceSelect = `SELECT m.id, am.alias_id, am.mailbox_uid, al.account_id, al.address, al.label, COALESCE(mg.name,''), m.subject, m.from_json, m.internal_date, m.content_state, am.otp, m.content_bytes, COALESCE(ws.read,FALSE), COALESCE(ws.starred,FALSE), COALESCE(ws.archived,FALSE), COALESCE(ws.deleted,FALSE), COALESCE((SELECT mn.note FROM mailbox_notes mn WHERE mn.address=al.address),'') FROM alias_messages am JOIN archived_messages m ON m.id=am.message_id JOIN aliases al ON al.id=am.alias_id LEFT JOIN mail_groups mg ON mg.id=al.group_id LEFT JOIN workspace_mail_states ws ON ws.alias_id=am.alias_id AND ws.message_id=am.message_id AND ws.admin_id=?`

func scanWorkspaceItem(row rowScanner) (domain.WorkspaceMailItem, error) {
	var v domain.WorkspaceMailItem
	var uid, ts int64
	var from string
	if err := row.Scan(&v.ID, &v.AliasID, &uid, &v.AccountID, &v.Address, &v.AliasLabel, &v.GroupName, &v.Subject, &from, &ts, &v.ContentState, &v.OTP, &v.ContentBytes, &v.Read, &v.Starred, &v.Archived, &v.Deleted, &v.MailboxNote); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return v, ErrNotFound
		}
		return v, err
	}
	if uid < 1 || uid > int64(^uint32(0)) {
		return v, ErrWorkspaceMailInput
	}
	v.MailboxUID = uint32(uid)
	v.ReceivedAt = timeFromTimestamp(ts)
	if err := json.Unmarshal([]byte(from), &v.From); err != nil {
		return v, err
	}
	return v, nil
}

func (s *Store) ListWorkspaceMail(ctx context.Context, f domain.WorkspaceMailFilter) (domain.WorkspaceMailPage, error) {
	if f.AdminID < 1 {
		return domain.WorkspaceMailPage{}, ErrWorkspaceMailInput
	}
	if f.Limit <= 0 {
		f.Limit = 50
	}
	if f.Limit > 100 {
		f.Limit = 100
	}
	if f.Offset < 0 {
		f.Offset = 0
	}
	if f.Offset > 1000000 {
		f.Offset = 1000000
	}
	where, args := workspaceWhere(f)
	page := domain.WorkspaceMailPage{Items: []domain.WorkspaceMailItem{}}
	q := ` FROM alias_messages am JOIN archived_messages m ON m.id=am.message_id JOIN aliases al ON al.id=am.alias_id LEFT JOIN mail_groups mg ON mg.id=al.group_id LEFT JOIN workspace_mail_states ws ON ws.alias_id=am.alias_id AND ws.message_id=am.message_id AND ws.admin_id=? WHERE ` + where
	if err := s.queryRowContext(ctx, "SELECT COUNT(*)"+q, append([]any{f.AdminID}, args...)...).Scan(&page.Total); err != nil {
		return page, err
	}
	// Unread count follows the same account/alias/group/query constraints, but ignores folder/unread/starred.
	base := f
	base.Folder = "all"
	base.Unread = false
	base.Starred = false
	bw, ba := workspaceWhere(base)
	unreadSQL := ` FROM alias_messages am JOIN archived_messages m ON m.id=am.message_id JOIN aliases al ON al.id=am.alias_id LEFT JOIN mail_groups mg ON mg.id=al.group_id LEFT JOIN workspace_mail_states ws ON ws.alias_id=am.alias_id AND ws.message_id=am.message_id AND ws.admin_id=? WHERE ` + bw + ` AND COALESCE(ws.read,FALSE)=FALSE`
	if err := s.queryRowContext(ctx, "SELECT COUNT(*)"+unreadSQL, append([]any{f.AdminID}, ba...)...).Scan(&page.UnreadCount); err != nil {
		return page, err
	}
	queryArgs := append([]any{f.AdminID}, args...)
	queryArgs = append(queryArgs, f.Limit, f.Offset)
	rows, err := s.queryContext(ctx, workspaceSelect+" WHERE "+where+" ORDER BY m.internal_date DESC,m.id DESC,am.alias_id DESC LIMIT ? OFFSET ?", queryArgs...)
	if err != nil {
		return page, err
	}
	defer rows.Close()
	for rows.Next() {
		v, e := scanWorkspaceItem(rows)
		if e != nil {
			return page, e
		}
		page.Items = append(page.Items, v)
	}
	if err = rows.Err(); err != nil {
		return page, err
	}
	page.HasMore = f.Offset+len(page.Items) < page.Total
	return page, nil
}

func (s *Store) GetWorkspaceMail(ctx context.Context, adminID, aliasID int64, uid uint32) (domain.WorkspaceMailItem, error) {
	if adminID < 1 || aliasID < 1 || uid < 1 {
		return domain.WorkspaceMailItem{}, ErrWorkspaceMailInput
	}
	return scanWorkspaceItem(s.queryRowContext(ctx, workspaceSelect+` WHERE am.alias_id=? AND am.mailbox_uid=?`, adminID, aliasID, int64(uid)))
}

func (s *Store) PatchWorkspaceMail(ctx context.Context, adminID, aliasID int64, uid uint32, p domain.WorkspaceMailPatch) (domain.WorkspaceMailItem, error) {
	if adminID < 1 || aliasID < 1 || uid < 1 {
		return domain.WorkspaceMailItem{}, ErrWorkspaceMailInput
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.WorkspaceMailItem{}, err
	}
	defer tx.Rollback()
	var mid int64
	if err = s.txQueryRowContext(ctx, tx, `SELECT message_id FROM alias_messages WHERE alias_id=? AND mailbox_uid=?`, aliasID, int64(uid)).Scan(&mid); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.WorkspaceMailItem{}, ErrNotFound
		}
		return domain.WorkspaceMailItem{}, err
	}
	if _, err = s.txExecContext(ctx, tx, `INSERT INTO workspace_mail_states(admin_id,alias_id,message_id,updated_at) VALUES(?,?,?,?) ON CONFLICT(admin_id,alias_id,message_id) DO NOTHING`, adminID, aliasID, mid, timestamp(s.now())); err != nil {
		return domain.WorkspaceMailItem{}, err
	}
	sets := []string{}
	vals := []any{}
	if p.Read != nil {
		sets = append(sets, "read=?")
		vals = append(vals, *p.Read)
	}
	if p.Starred != nil {
		sets = append(sets, "starred=?")
		vals = append(vals, *p.Starred)
	}
	if p.Archived != nil {
		sets = append(sets, "archived=?")
		vals = append(vals, *p.Archived)
	}
	if p.Deleted != nil {
		sets = append(sets, "deleted=?")
		vals = append(vals, *p.Deleted)
	}
	if len(sets) > 0 {
		sets = append(sets, "updated_at=?")
		vals = append(vals, timestamp(s.now()), adminID, aliasID, mid)
		if _, err = s.txExecContext(ctx, tx, `UPDATE workspace_mail_states SET `+strings.Join(sets, ",")+` WHERE admin_id=? AND alias_id=? AND message_id=?`, vals...); err != nil {
			return domain.WorkspaceMailItem{}, err
		}
	}
	if err = tx.Commit(); err != nil {
		return domain.WorkspaceMailItem{}, err
	}
	return s.GetWorkspaceMail(ctx, adminID, aliasID, uid)
}
