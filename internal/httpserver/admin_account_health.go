package httpserver

import (
	"context"
	"errors"
	"time"

	"icloud-api/internal/domain"
	"icloud-api/internal/hmesync"
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

// An expired session still in automatic backoff is retrying, not yet a manual login.
func appleChannelHealth(info hmesync.SessionInfo, err error) string {
	switch {
	case err != nil:
		return appleHealthUnknown
	case info.Health == appleHealthRetryWait:
		return appleHealthRetryWait
	case info.Status == hmesync.StatusExpired:
		return appleHealthExpired
	case info.Status != hmesync.StatusAuthenticated:
		return appleHealthNotConnected
	default:
		return appleHealthHealthy
	}
}

var errAccountHealthUnavailable = errors.New("account health unavailable")

type sessionHealthReader interface {
	GetSessionHealth(context.Context, []int64) (map[int64]hmesync.SessionHealth, error)
}

func (s *Server) adminAPIAppleHealth(ctx context.Context, accounts []domain.Account) map[int64]*adminAPIAccountAppleDTO {
	result := make(map[int64]*adminAPIAccountAppleDTO)
	if s.hmeSync == nil {
		return result
	}
	ids := make([]int64, 0, len(accounts))
	for _, account := range accounts {
		if account.Enabled && domain.NormalizeMailboxType(account.MailboxType) != domain.MailboxTypeCustom {
			ids = append(ids, account.ID)
		}
	}
	if len(ids) == 0 {
		return result
	}
	local, _ := s.store.ListAccountAppleHealth(ctx, ids)
	sessions := make(map[int64]hmesync.SessionHealth, len(ids))
	if reader, ok := s.hmeSync.(sessionHealthReader); ok {
		var err error
		sessions, err = reader.GetSessionHealth(ctx, ids)
		if err != nil {
			sessions = make(map[int64]hmesync.SessionHealth, len(ids))
			for _, id := range ids {
				sessions[id] = hmesync.SessionHealth{WebErr: err, AccountErr: err}
			}
		}
	} else {
		// Preserve compatibility with service adapters using the original interface.
		for _, id := range ids {
			web, webErr := s.hmeSync.GetSession(ctx, id)
			info := hmesync.SessionHealth{Web: web, WebErr: webErr}
			if reader, ok := s.hmeSync.(accountSessionReader); ok {
				info.Account, info.AccountErr = reader.GetAccountSession(ctx, id)
			} else {
				info.AccountErr = errAccountHealthUnavailable
			}
			sessions[id] = info
		}
	}
	now := time.Now()
	for _, id := range ids {
		info := sessions[id]
		dto := &adminAPIAccountAppleDTO{Web: appleChannelHealth(info.Web, info.WebErr), Account: appleChannelHealth(info.Account, info.AccountErr)}
		state := local[id]
		dto.KeepAlive = state.KeepAliveStatus
		if state.CreationCooldownUntil.After(now) {
			dto.CreationCooldownUntil = adminAPIOptionalTime(&state.CreationCooldownUntil)
		}
		dto.NeedsAttention = dto.Web == appleHealthExpired || dto.Account == appleHealthExpired || dto.KeepAlive == "needs_action"
		result[id] = dto
	}
	return result
}
