package service

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"

	mw "github.com/zippyra/platform/services/auth-service/internal/middleware"
	"github.com/zippyra/platform/shared/errors"
	"github.com/zippyra/platform/shared/logger"
)

// AuthService handles user authentication, token management, and session creation.
type AuthService struct {
	rdb              *redis.Client
	userRepo         UserRepo
	loginAttemptRepo LoginAttemptRepo
	sessionStore     SessionStore
	jwtMiddleware    TokenGenerator
	publisher        EventPublisher
}

// NewAuthService creates a new AuthService.
func NewAuthService(
	rdb *redis.Client,
	userRepo UserRepo,
	loginAttemptRepo LoginAttemptRepo,
	sessionStore SessionStore,
	jwtMiddleware TokenGenerator,
	publisher EventPublisher,
) *AuthService {
	return &AuthService{
		rdb:              rdb,
		userRepo:         userRepo,
		loginAttemptRepo: loginAttemptRepo,
		sessionStore:     sessionStore,
		jwtMiddleware:    jwtMiddleware,
		publisher:        publisher,
	}
}

// AuthResult contains the tokens and user info returned after OTP verification.
type AuthResult struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	UserID       string `json:"user_id"`
	IsNewUser    bool   `json:"is_new_user"`
	ExpiresIn    int    `json:"expires_in"`
}

// CompleteLogin upserts the user, generates tokens, stores refresh hash, and publishes event.
func (s *AuthService) CompleteLogin(ctx context.Context, phone, deviceID, deviceModel, ip, userAgent string) (*AuthResult, error) {
	// 1. Upsert user
	user, isNew, err := s.userRepo.UpsertByPhone(ctx, phone)
	if err != nil {
		log.Error().Err(err).Str("phone", logger.MaskPhone(phone)).Msg("user upsert failed")
		return nil, fmt.Errorf("user upsert failed: %w", err)
	}

	userIDStr := user.ID.String()
	maskedPhone := logger.MaskPhone(phone)

	// 2. Generate Ed25519 access token (24h)
	accessToken, err := s.jwtMiddleware.GenerateAccessToken(userIDStr, "CUSTOMER", maskedPhone, deviceID)
	if err != nil {
		return nil, fmt.Errorf("access token generation failed: %w", err)
	}

	// 3. Generate Ed25519 refresh token (30 days)
	refreshToken, err := s.jwtMiddleware.GenerateRefreshToken(userIDStr, "CUSTOMER", maskedPhone, deviceID)
	if err != nil {
		return nil, fmt.Errorf("refresh token generation failed: %w", err)
	}

	// 4. Store refresh token hash in Redis
	refreshHash := sha256Hash(refreshToken)
	rCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()
	err = s.rdb.Set(rCtx, "refresh:"+userIDStr+":"+deviceID, refreshHash, 30*24*time.Hour).Err()
	if err != nil {
		log.Error().Err(err).Msg("failed to store refresh token hash")
		return nil, fmt.Errorf("refresh token storage failed: %w", err)
	}

	// 5. Create auth session
	if err := s.sessionStore.CreateSession(ctx, user.ID, deviceID, deviceModel, ip, userAgent); err != nil {
		log.Error().Err(err).Msg("failed to create auth session")
		// Non-fatal — proceed with login
	}

	// 6. Update login attempt status
	go func() {
		if err := s.loginAttemptRepo.UpdateStatus(context.Background(), phone, "SUCCESS"); err != nil {
			log.Error().Err(err).Msg("failed to update login attempt status")
		}
	}()

	// 7. Publish Kafka event (fire-and-forget)
	s.publisher.PublishLoginEvent(ctx, userIDStr, isNew)

	log.Info().
		Str("user_id", userIDStr).
		Bool("is_new_user", isNew).
		Str("phone", maskedPhone).
		Msg("login completed")

	return &AuthResult{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		UserID:       userIDStr,
		IsNewUser:    isNew,
		ExpiresIn:    86400,
	}, nil
}

// RefreshAccessToken validates a refresh token and issues a new access token.
func (s *AuthService) RefreshAccessToken(ctx context.Context, refreshTokenStr string) (string, string, error) {
	// 1. Validate token
	claims, err := s.jwtMiddleware.ValidateToken(refreshTokenStr)
	if err != nil {
		return "", errors.ErrTokenInvalid, err
	}

	// 2. Verify device_id binding against session store
	hasSession, err := s.sessionStore.HasActiveSession(ctx, claims.UserID, claims.DeviceID)
	if err != nil {
		return "", errors.ErrInternal, err
	}
	if !hasSession {
		return "", errors.ErrTokenInvalid, fmt.Errorf("no active session for device")
	}

	// 3. Check blacklist
	blacklisted, err := s.jwtMiddleware.IsBlacklisted(ctx, claims.ID)
	if err != nil {
		return "", errors.ErrInternal, err
	}
	if blacklisted {
		return "", errors.ErrTokenBlacklisted, fmt.Errorf("refresh token is blacklisted")
	}

	// 4. Verify refresh hash matches stored hash
	rCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()

	storedHash, err := s.rdb.Get(rCtx, "refresh:"+claims.UserID+":"+claims.DeviceID).Result()
	if err == redis.Nil {
		return "", errors.ErrTokenInvalid, fmt.Errorf("no refresh token found for user")
	}
	if err != nil {
		return "", errors.ErrInternal, err
	}

	incomingHash := sha256Hash(refreshTokenStr)
	if subtle.ConstantTimeCompare([]byte(storedHash), []byte(incomingHash)) != 1 {
		return "", errors.ErrTokenInvalid, fmt.Errorf("refresh token hash mismatch")
	}

	// 5. Issue new access token
	newAccessToken, err := s.jwtMiddleware.GenerateAccessToken(
		claims.UserID, claims.UserType, claims.Phone, claims.DeviceID,
	)
	if err != nil {
		return "", errors.ErrInternal, err
	}

	// 6. Update session last_active_at
	go func() {
		_ = s.sessionStore.UpdateSessionActivity(context.Background(), claims.UserID, claims.DeviceID)
	}()

	return newAccessToken, "", nil
}

// Logout blacklists the access token and deletes the refresh token.
func (s *AuthService) Logout(ctx context.Context, claims *mw.Claims) error {
	// 1. Blacklist access token
	if err := s.jwtMiddleware.BlacklistToken(ctx, claims.ID, claims.ExpiresAt.Time); err != nil {
		log.Error().Err(err).Msg("failed to blacklist token")
		return err
	}

	// 2. Delete refresh token
	rCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()
	s.rdb.Del(rCtx, "refresh:"+claims.UserID+":"+claims.DeviceID)

	// 3. Mark session as logged out
	go func() {
		_ = s.sessionStore.LogoutSession(context.Background(), claims.UserID, claims.DeviceID)
	}()

	log.Info().Str("user_id", claims.UserID).Msg("user logged out")
	return nil
}

func sha256Hash(data string) string {
	h := sha256.Sum256([]byte(data))
	return hex.EncodeToString(h[:])
}
