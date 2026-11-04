package hmesync

import (
	"context"
	"errors"

	"icloud-api/internal/apple"
	"icloud-api/internal/domain"
	"icloud-api/internal/store"
)

// SessionHealth is the same local view as GetSession/GetAccountSession, with
// independent errors so one broken envelope does not hide the other channel.
type SessionHealth struct {
	Web, Account       SessionInfo
	WebErr, AccountErr error
}

type sessionHealthRepository interface {
	ListAppleSessions(context.Context, []int64) (map[int64]domain.AppleWebSession, map[int64]domain.AppleWebSession, error)
}

// GetSessionHealth reads both channels by ID set and decrypts each envelope at
// most once. It never probes Apple or updates a stored session.
func (s *Service) GetSessionHealth(ctx context.Context, ids []int64) (map[int64]SessionHealth, error) {
	result := make(map[int64]SessionHealth, len(ids))
	repo, ok := s.repo.(sessionHealthRepository)
	if !ok {
		// Compatibility for repository adapters implementing only single reads.
		for _, id := range ids {
			web, webErr := s.GetSession(ctx, id)
			account, accountErr := s.GetAccountSession(ctx, id)
			result[id] = SessionHealth{web, account, webErr, accountErr}
		}
		return result, nil
	}
	webRecords, accountRecords, err := repo.ListAppleSessions(ctx, ids)
	var readErrors *store.AppleSessionBatchError
	if err != nil && !errors.As(err, &readErrors) {
		return nil, err
	}
	for _, id := range ids {
		if id < 1 {
			continue
		}
		if _, exists := result[id]; exists {
			continue
		}
		web, webExists := webRecords[id]
		account, accountExists := accountRecords[id]
		var webErr, accountErr error
		if !webExists {
			webErr = store.ErrNotFound
		}
		if !accountExists {
			accountErr = store.ErrNotFound
		}
		if readErrors != nil {
			if err := readErrors.Web[id]; err != nil {
				webErr = err
			}
			if err := readErrors.Account[id]; err != nil {
				accountErr = err
			}
		}
		type decoded struct {
			session apple.Session
			err     error
		}
		type envelopeKey struct{ ciphertext, appleID, region string }
		cache := make(map[envelopeKey]decoded, 2)
		decrypt := func(record domain.AppleWebSession) (apple.Session, error) {
			key := envelopeKey{record.Ciphertext, record.AppleID, record.Region}
			if value, ok := cache[key]; ok {
				return value.session, value.err
			}
			session, err := s.decryptSession(record)
			cache[key] = decoded{session, err}
			return session, err
		}
		info, infoErr := s.webSessionInfo(web, webErr, decrypt)
		managed, managedErr := accountManagementSession(account, accountErr, web, webErr, decrypt)
		accountInfo, accountInfoErr := s.accountSessionHealth(managed, managedErr)
		result[id] = SessionHealth{info, accountInfo, infoErr, accountInfoErr}
	}
	return result, nil
}
