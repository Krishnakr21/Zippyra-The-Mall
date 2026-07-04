package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	mw "github.com/zippyra/platform/services/auth-service/internal/middleware"
	"github.com/zippyra/platform/services/auth-service/internal/model"
)

// UserRepo abstracts user database operations for testability.
type UserRepo interface {
	UpsertByPhone(ctx context.Context, phone string) (*model.User, bool, error)
	GetByID(ctx context.Context, id uuid.UUID) (*model.User, error)
}

// LoginAttemptRepo abstracts login attempt recording for testability.
type LoginAttemptRepo interface {
	Insert(ctx context.Context, phone, ipAddress, userAgent, status string) error
	UpdateStatus(ctx context.Context, phone, status string) error
}

// AppVersionRepo abstracts app version lookups for testability.
type AppVersionRepo interface {
	GetLatest(ctx context.Context, platform string) (*model.AppVersion, error)
}

// EventPublisher abstracts Kafka event publishing for testability.
type EventPublisher interface {
	PublishLoginEvent(ctx context.Context, userID string, isNewUser bool)
}

// TokenGenerator abstracts JWT operations for testability.
type TokenGenerator interface {
	GenerateAccessToken(userID, userType, phone, deviceID string) (string, error)
	GenerateRefreshToken(userID, userType, phone, deviceID string) (string, error)
	ValidateToken(tokenString string) (*mw.Claims, error)
	BlacklistToken(ctx context.Context, jti string, expiresAt time.Time) error
	IsBlacklisted(ctx context.Context, jti string) (bool, error)
}

// SessionStore abstracts session database operations for testability.
type SessionStore interface {
	CreateSession(ctx context.Context, userID uuid.UUID, deviceID, deviceModel, ip, userAgent string) error
	UpdateSessionActivity(ctx context.Context, userID, deviceID string) error
	LogoutSession(ctx context.Context, userID, deviceID string) error
	HasActiveSession(ctx context.Context, userID, deviceID string) (bool, error)
	ListSessions(ctx context.Context, userID uuid.UUID) ([]SessionRow, error)
	RevokeSession(ctx context.Context, userID, sessionID uuid.UUID) error
	RevokeAllSessions(ctx context.Context, userID uuid.UUID, currentDeviceID string) (int, error)
}

// SessionRow is a raw session record from the database.
type SessionRow struct {
	ID           uuid.UUID
	DeviceID     string
	DeviceModel  string
	LastActiveAt interface{} // time.Time
	IPAddress    string
}
