package domain

import "time"

type MailboxProfile struct {
	AliasID       int64                 `json:"alias_id"`
	Address       string                `json:"address"`
	Note          string                `json:"note"`
	UpdatedAt     *time.Time            `json:"updated_at"`
	Registrations []ServiceRegistration `json:"registrations"`
}
type MailService struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Domain    string    `json:"domain"`
	Note      string    `json:"note"`
	Archived  bool      `json:"archived"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
type ServiceRegistration struct {
	ID                int64     `json:"id"`
	ServiceID         int64     `json:"service_id"`
	ServiceName       string    `json:"service_name"`
	Address           string    `json:"address"`
	AccountIdentifier string    `json:"account_identifier"`
	Status            string    `json:"status"`
	Note              string    `json:"note"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}
type ServiceRegistrationInput struct {
	ServiceID         int64  `json:"service_id"`
	AccountIdentifier string `json:"account_identifier"`
	Status            string `json:"status"`
	Note              string `json:"note"`
}
type ServiceMailbox struct {
	AliasID           int64  `json:"alias_id"`
	AccountID         int64  `json:"account_id"`
	Address           string `json:"address"`
	AliasLabel        string `json:"alias_label"`
	Note              string `json:"note"`
	Enabled           bool   `json:"enabled"`
	GroupName         string `json:"group_name"`
	Status            string `json:"status"`
	RegistrationCount int    `json:"registration_count"`
}
type ServiceMailboxFilter struct {
	ServiceID   int64
	Query       string
	Status      string
	AccountID   int64
	GroupID     int64
	EnabledOnly bool
	Limit       int
	Offset      int
}
type ServiceMailboxPage struct {
	Items   []ServiceMailbox `json:"items"`
	Total   int              `json:"total"`
	HasMore bool             `json:"has_more"`
}
