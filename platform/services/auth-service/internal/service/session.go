package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

// SessionService handles session listing and revocation.
type SessionService struct {
	rdb           *redis.Client
	sessionStore  SessionStore
	jwtMiddleware TokenGenerator
}

// NewSessionService creates a new SessionService.
func NewSessionService(rdb *redis.Client, sessionStore SessionStore, jwtMiddleware TokenGenerator) *SessionService {
	return &SessionService{rdb: rdb, sessionStore: sessionStore, jwtMiddleware: jwtMiddleware}
}

// SessionInfo is the public representation of an auth session.
type SessionInfo struct {
	ID           string    `json:"id"`
	DeviceModel  string    `json:"device_model"`
	LastActiveAt time.Time `json:"last_active_at"`
	IPAddress    string    `json:"ip_address"`
	IsCurrent    bool      `json:"is_current"`
}

// ListSessions returns all active sessions for a user.
func (s *SessionService) ListSessions(ctx context.Context, userID, currentDeviceID string) ([]SessionInfo, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user_id: %w", err)
	}

	rows, err := s.sessionStore.ListSessions(ctx, uid)
	if err != nil {
		return nil, err
	}

	var sessions []SessionInfo
	for _, row := range rows {
		lastActive, _ := row.LastActiveAt.(time.Time)
		sessions = append(sessions, SessionInfo{
			ID:           row.ID.String(),
			DeviceModel:  row.DeviceModel,
			LastActiveAt: lastActive,
			IPAddress:    row.IPAddress,
			IsCurrent:    row.DeviceID == currentDeviceID,
		})
	}

	return sessions, nil
}

// RevokeSession revokes a specific session. Validates it belongs to the authenticated user.
func (s *SessionService) RevokeSession(ctx context.Context, userID, sessionID string) error {
	uid, _ := uuid.Parse(userID)
	sid, err := uuid.Parse(sessionID)
	if err != nil {
		return fmt.Errorf("invalid session_id: %w", err)
	}

	if err := s.sessionStore.RevokeSession(ctx, uid, sid); err != nil {
		return err
	}

	log.Info().Str("user_id", userID).Str("session_id", sessionID).Msg("session revoked")
	return nil
}

// RevokeAllSessions revokes all sessions except the current one.
// Returns the count of sessions revoked.
func (s *SessionService) RevokeAllSessions(ctx context.Context, userID, currentDeviceID string) (int, error) {
	uid, _ := uuid.Parse(userID)

	count, err := s.sessionStore.RevokeAllSessions(ctx, uid, currentDeviceID)
	if err != nil {
		return 0, err
	}

	// Delete all refresh tokens for other devices
	rCtx, rCancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer rCancel()
	iter := s.rdb.Scan(rCtx, 0, "refresh:"+userID+":*", 0).Iterator()
	for iter.Next(rCtx) {
		k := iter.Val()
		if k == "refresh:"+userID+":"+currentDeviceID {
			continue
		}
		s.rdb.Del(rCtx, k)
	}
	_ = iter.Err()

	log.Info().Str("user_id", userID).Int("revoked_count", count).Msg("all other sessions revoked")
	return count, nil
}
