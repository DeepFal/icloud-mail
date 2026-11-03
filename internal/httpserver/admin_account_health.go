package httpserver

import (
	"context"
	"errors"
	"time"

	"icloud-api/internal/domain"
	"icloud-api/internal/hmesync"
	"icloud-api/internal/store"
)

// Apple channel health in the account list. Everything here is read from local
// state; nothing contacts Apple.
const (
	appleHealthHealthy      = "healthy"
	appleHealthRetryWait    = "retry_wait"
	appleHealthExpired      = "expired"
	appleHealthNotConnected = "not_connected"
	appleHealthUnknown      = "unknown"
)

type adminAPIAccountAppleDTO struct {
	Web                   string  `json:"web"`
	Account               string  `json:"account"`
	KeepAlive             string  `json:"keep_alive"`
	CreationCooldownUntil *string `json:"creation_cooldown_until"`
	NeedsAttention        bool    `json:"needs_attention"`
}

type accountSessionReader interface {
	GetAccountSession(context.Context, int64) (hmesync.SessionInfo, error)
}

func appleChannelHealth(info hmesync.SessionInfo, err error) string {
	switch {
	case err != nil:
		return appleHealthUnknown
	case info.Status == hmesync.StatusExpired:
		return appleHealthExpired
	case info.Status != hmesync.StatusAuthenticated:
		return appleHealthNotConnected
	case info.Health == appleHealthRetryWait:
		return appleHealthRetryWait
	default:
		return appleHealthHealthy
	}
}

// keepAliveRank orders automatic-login states by how much attention they need.
var keepAliveRank = map[string]int{"idle": 1, "authenticated": 2, "logging_in": 3, "waiting_code": 3, "retry_wait": 4, "needs_action": 5}

func (s *Server) adminAPIAppleHealth(ctx context.Context, account domain.Account) *adminAPIAccountAppleDTO {
	if s.hmeSync == nil || !account.Enabled || domain.NormalizeMailboxType(account.MailboxType) == domain.MailboxTypeCustom {
		return nil
	}
	web, webErr := s.hmeSync.GetSession(ctx, account.ID)
	dto := &adminAPIAccountAppleDTO{Web: appleChannelHealth(web, webErr), Account: appleHealthUnknown, KeepAlive: "disabled"}
	if reader, ok := s.hmeSync.(accountSessionReader); ok {
		info, err := reader.GetAccountSession(ctx, account.ID)
		dto.Account = appleChannelHealth(info, err)
	}
	dto.KeepAlive = s.adminAPIKeepAliveStatus(ctx, account.ID)
	var until time.Time
	for _, channel := range []string{domain.AutoLoginChannelAppleAccount, domain.AutoLoginChannelICloudWeb} {
		if paused, err := s.store.AppleCreationChannelPausedUntil(ctx, account.ID, channel); err == nil && paused.After(until) {
			until = paused
		}
	}
	if until.After(time.Now()) {
		dto.CreationCooldownUntil = adminAPIOptionalTime(&until)
	}
	dto.NeedsAttention = dto.Web == appleHealthExpired || dto.Account == appleHealthExpired || dto.KeepAlive == "needs_action"
	return dto
}

func (s *Server) adminAPIKeepAliveStatus(ctx context.Context, accountID int64) string {
	config, err := s.store.GetAccountAutoLoginConfig(ctx, accountID)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return appleHealthUnknown
	}
	if !config.Enabled {
		return "disabled"
	}
	status := "idle"
	for _, channel := range []string{domain.AutoLoginChannelAppleAccount, domain.AutoLoginChannelICloudWeb} {
		state, err := s.store.GetAccountAutoLoginState(ctx, accountID, channel)
		if err == nil && keepAliveRank[state.Status] > keepAliveRank[status] {
			status = state.Status
		}
	}
	return status
}
