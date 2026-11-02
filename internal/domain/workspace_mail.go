package domain

import "time"

type WorkspaceMailFilter struct {
	AdminID   int64
	AccountID int64
	AliasID   int64
	GroupID   int64
	Query     string
	Folder    string
	Unread    bool
	Starred   bool
	Limit     int
	Offset    int
}

type WorkspaceMailItem struct {
	ID           int64         `json:"id"`
	AliasID      int64         `json:"alias_id"`
	MailboxUID   uint32        `json:"mailbox_uid"`
	AccountID    int64         `json:"account_id"`
	Address      string        `json:"address"`
	AliasLabel   string        `json:"alias_label"`
	MailboxNote  string        `json:"mailbox_note"`
	GroupName    string        `json:"group_name"`
	Subject      string        `json:"subject"`
	From         []MailAddress `json:"from"`
	ReceivedAt   time.Time     `json:"received_at"`
	ContentState string        `json:"content_state"`
	OTP          string        `json:"otp"`
	ContentBytes int64         `json:"content_bytes"`
	Read         bool          `json:"read"`
	Starred      bool          `json:"starred"`
	Archived     bool          `json:"archived"`
	Deleted      bool          `json:"deleted"`
}

type WorkspaceMailPage struct {
	Items       []WorkspaceMailItem `json:"items"`
	Total       int                 `json:"total"`
	UnreadCount int                 `json:"unread_count"`
	HasMore     bool                `json:"has_more"`
}
type WorkspaceMailPatch struct {
	Read     *bool `json:"read"`
	Starred  *bool `json:"starred"`
	Archived *bool `json:"archived"`
	Deleted  *bool `json:"deleted"`
}
