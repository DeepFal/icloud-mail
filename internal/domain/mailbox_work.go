package domain

import "time"

// MailboxPreferences controls persistent receiving independently of browser requests.
type MailboxPreferences struct {
	AccountID         int64 `json:"account_id"`
	BackgroundEnabled bool  `json:"background_enabled"`
	HistoryDays       int   `json:"history_days"`
}

type MailHistoryJob struct {
	ID               string     `json:"id"`
	AccountID        int64      `json:"account_id"`
	Status           string     `json:"status"`
	Since            time.Time  `json:"since"`
	Until            time.Time  `json:"until"`
	UIDValidity      uint32     `json:"uid_validity"`
	LastUID          uint32     `json:"last_uid"`
	TargetUID        uint32     `json:"target_uid"`
	Initialized      bool       `json:"initialized"`
	ScannedCount     int64      `json:"scanned_count"`
	ArchivedCount    int64      `json:"archived_count"`
	Failures         int        `json:"failures"`
	LastError        string     `json:"last_error"`
	NextRunAt        *time.Time `json:"next_run_at,omitempty"`
	ConfigVersion    int64      `json:"-"`
	TransportVersion int64      `json:"-"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

type MailHistoryBatch struct {
	UIDValidity uint32
	LastUID     uint32
	TargetUID   uint32
	Scanned     int
	Done        bool
	Messages    []ArchivedMessage
}
