package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/zippyra/platform/services/store-service/internal/model"
)

// StoreRepo abstracts store database operations for testability.
type StoreRepo interface {
	GetByID(ctx context.Context, storeID uuid.UUID) (*model.Store, error)
	GetByIDAndChain(ctx context.Context, storeID, chainID uuid.UUID) (*model.Store, error)
	NearbyStores(ctx context.Context, lat, lng, radiusKM float64) ([]model.Store, []float64, error)
	GetHours(ctx context.Context, storeID uuid.UUID, dayOfWeek int) (*model.StoreHours, error)
}

// QRTokenRepo abstracts QR token database operations for testability.
type QRTokenRepo interface {
	GetActiveToken(ctx context.Context, token string) (*model.StoreQRToken, error)
	IncrementUsedCount(ctx context.Context, tokenID uuid.UUID) error
}

// DeviceRepo abstracts device database operations for testability.
type DeviceRepo interface {
	ListByStoreID(ctx context.Context, storeID uuid.UUID) ([]model.Device, error)
}

// EventPublisher abstracts Kafka event publishing for testability.
type EventPublisher interface {
	PublishCustomerEntered(ctx context.Context, userID, storeID, chainID, qrTokenID string)
	PublishCustomerExited(ctx context.Context, userID, storeID, chainID string, durationSeconds int64)
}

// RedisStore abstracts Redis operations for testability and portability.
type RedisStore interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value string, ttl time.Duration) error
	Incr(ctx context.Context, key string) (int64, error)
	Decr(ctx context.Context, key string) (int64, error)
	Del(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)
}
