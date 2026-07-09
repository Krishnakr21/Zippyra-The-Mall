package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/zippyra/platform/services/order-service/internal/model"
	"github.com/zippyra/platform/services/order-service/internal/repository"
	"github.com/zippyra/platform/services/order-service/internal/testutil"
	"github.com/zippyra/platform/shared/jwt"
)

func setupTestExitTokenService(t *testing.T) (*pgxpool.Pool, *ExitTokenService, *repository.OrderRepository, *repository.ExitTokenRepository, redisCmdableWrapper, func()) {
	pool, dbCleanup := testutil.SetupDB()
	rdb, redisCleanup := newTestRedis(t)

	orderRepo := repository.NewOrderRepository(pool)
	exitTokenRepo := repository.NewExitTokenRepository(pool)

	jwtSvc := MockJWTService{
		GenerateExitTokenFn: func(claims jwt.ExitTokenClaims) (string, error) {
			return "mocked_jwt_exit_token", nil
		},
	}

	svc := NewExitTokenService(exitTokenRepo, orderRepo, rdb, jwtSvc)

	cleanup := func() {
		dbCleanup()
		redisCleanup()
	}

	return pool, svc, orderRepo, exitTokenRepo, rdb, cleanup
}

type redisCmdableWrapper interface {
	Get(ctx context.Context, key string) *redis.StringCmd
}

func TestExitTokenService_Issue(t *testing.T) {
	pool, svc, orderRepo, exitTokenRepo, rdb, cleanup := setupTestExitTokenService(t)
	defer cleanup()

	ctx := context.Background()
	userID, storeID, _, err := testutil.SeedBaseData(ctx, pool)
	assert.NoError(t, err)

	orderID := uuid.New()
	order := &model.Order{
		ID:          orderID,
		UserID:      userID,
		StoreID:     storeID,
		OrderNumber: "ZP-2026-TOKENISSUE",
	}

	// Insert order first
	tx, err := pool.Begin(ctx)
	assert.NoError(t, err)
	_, _, err = orderRepo.UpsertOrder(ctx, tx, order)
	assert.NoError(t, err)
	err = tx.Commit(ctx)
	assert.NoError(t, err)

	token, err := svc.Issue(ctx, order)
	assert.NoError(t, err)
	assert.Equal(t, "mocked_jwt_exit_token", token)

	// Verify hashed token stored in DB
	expectedHash := sha256Hex(token)
	et, err := exitTokenRepo.GetActiveByOrderID(ctx, orderID.String())
	assert.NoError(t, err)
	assert.NotNil(t, et)
	assert.Equal(t, expectedHash, et.TokenHash)
	assert.False(t, et.IsUsed)
	assert.WithinDuration(t, time.Now().Add(10*time.Minute), et.ExpiresAt, 5*time.Second)

	// Verify cached in Redis
	redisKey := fmt.Sprintf("exit_preauth:%s", orderID.String())
	cachedVal, err := rdb.Get(ctx, redisKey).Result()
	assert.NoError(t, err)
	assert.Equal(t, expectedHash, cachedVal)
}

func TestExitTokenService_Refresh(t *testing.T) {
	pool, svc, orderRepo, exitTokenRepo, _, cleanup := setupTestExitTokenService(t)
	defer cleanup()

	ctx := context.Background()
	userID, storeID, _, err := testutil.SeedBaseData(ctx, pool)
	assert.NoError(t, err)

	orderID := uuid.New()
	expiresAt := time.Now().Add(-5 * time.Minute) // Already expired
	order := &model.Order{
		ID:                 orderID,
		UserID:             userID,
		StoreID:            storeID,
		OrderNumber:        "ZP-2026-TOKENREFRESH",
		ExitToken:          stringPtr("old_jwt_token"),
		ExitTokenExpiresAt: &expiresAt,
	}

	// Insert base order
	tx, err := pool.Begin(ctx)
	assert.NoError(t, err)
	_, _, err = orderRepo.UpsertOrder(ctx, tx, order)
	assert.NoError(t, err)
	err = tx.Commit(ctx)
	assert.NoError(t, err)

	// Perform refresh
	newToken, err := svc.Refresh(ctx, order)
	assert.NoError(t, err)
	assert.Equal(t, "mocked_jwt_exit_token", newToken)

	// Verify order table updated
	updatedOrder, err := orderRepo.GetByID(ctx, orderID.String())
	assert.NoError(t, err)
	assert.NotNil(t, updatedOrder)
	assert.Equal(t, "mocked_jwt_exit_token", *updatedOrder.ExitToken)
	assert.True(t, updatedOrder.ExitTokenExpiresAt.After(time.Now()))

	// Verify new hash active in DB
	expectedHash := sha256Hex(newToken)
	et, err := exitTokenRepo.GetActiveByOrderID(ctx, orderID.String())
	assert.NoError(t, err)
	assert.NotNil(t, et)
	assert.Equal(t, expectedHash, et.TokenHash)
}

func stringPtr(s string) *string {
	return &s
}
