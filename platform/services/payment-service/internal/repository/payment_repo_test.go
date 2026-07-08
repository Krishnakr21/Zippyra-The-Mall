package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/zippyra/platform/services/payment-service/internal/model"
	"github.com/zippyra/platform/services/payment-service/internal/testutil"
)

func newTestRedis(t *testing.T) (*redis.Client, func()) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "zippyra_local",
		DB:       2,
	})
	ctx := context.Background()
	err := rdb.Ping(ctx).Err()
	assert.NoError(t, err, "failed to connect/ping real redis")
	cleanup := func() {
		_ = rdb.FlushDB(ctx)
		_ = rdb.Close()
	}
	return rdb, cleanup
}

func TestPaymentRepository_All(t *testing.T) {
	pool, dbCleanup := testutil.SetupDB()
	defer dbCleanup()

	_, redisCleanup := newTestRedis(t)
	defer redisCleanup()

	ctx := context.Background()
	repo := NewPaymentRepository(pool)

	userID, storeID, orderID, err := testutil.SeedBaseData(ctx, pool)
	assert.NoError(t, err)

	t.Run("Create_and_GetByID", func(t *testing.T) {
		pID := uuid.New()
		paymentMethod := model.PaymentMethodUPI
		p := &model.Payment{
			ID:             pID,
			OrderID:        orderID,
			UserID:         userID,
			StoreID:        storeID,
			AmountPaise:    15000,
			Currency:       "INR",
			Status:         model.PaymentStatusPending,
			PaymentMethod:  &paymentMethod,
			Gateway:        "RAZORPAY",
			GatewayOrderID: stringPtr("go_order_123"),
			IdempotencyKey: "idem_key_create_repo",
		}

		tx, err := pool.Begin(ctx)
		assert.NoError(t, err)
		err = repo.Create(ctx, tx, p)
		assert.NoError(t, err)
		err = tx.Commit(ctx)
		assert.NoError(t, err)

		// Retrieve by ID
		retrieved, err := repo.GetByID(ctx, pID.String())
		assert.NoError(t, err)
		assert.NotNil(t, retrieved)
		assert.Equal(t, pID, retrieved.ID)
		assert.Equal(t, int64(15000), retrieved.AmountPaise)
		assert.Equal(t, model.PaymentStatusPending, retrieved.Status)
		assert.Equal(t, "idem_key_create_repo", retrieved.IdempotencyKey)
		assert.Equal(t, "go_order_123", *retrieved.GatewayOrderID)
	})

	t.Run("GetByIdempotencyKey", func(t *testing.T) {
		pID := uuid.New()
		p := &model.Payment{
			ID:             pID,
			OrderID:        orderID,
			UserID:         userID,
			StoreID:        storeID,
			AmountPaise:    20000,
			Currency:       "INR",
			Status:         model.PaymentStatusPending,
			Gateway:        "RAZORPAY",
			IdempotencyKey: "idem_key_get_idem",
		}

		tx, err := pool.Begin(ctx)
		assert.NoError(t, err)
		err = repo.Create(ctx, tx, p)
		assert.NoError(t, err)
		err = tx.Commit(ctx)
		assert.NoError(t, err)

		// Retrieve by key
		retrieved, err := repo.GetByIdempotencyKey(ctx, "idem_key_get_idem")
		assert.NoError(t, err)
		assert.NotNil(t, retrieved)
		assert.Equal(t, pID, retrieved.ID)
		assert.Equal(t, int64(20000), retrieved.AmountPaise)
	})

	t.Run("UpdateStatus", func(t *testing.T) {
		pID := uuid.New()
		p := &model.Payment{
			ID:             pID,
			OrderID:        orderID,
			UserID:         userID,
			StoreID:        storeID,
			AmountPaise:    30000,
			Currency:       "INR",
			Status:         model.PaymentStatusPending,
			Gateway:        "RAZORPAY",
			IdempotencyKey: "idem_key_update_status",
		}

		tx, err := pool.Begin(ctx)
		assert.NoError(t, err)
		err = repo.Create(ctx, tx, p)
		assert.NoError(t, err)
		err = tx.Commit(ctx)
		assert.NoError(t, err)

		err = repo.UpdateStatus(ctx, pID.String(), string(model.PaymentStatusSuccess), "")
		assert.NoError(t, err)

		retrieved, err := repo.GetByID(ctx, pID.String())
		assert.NoError(t, err)
		assert.Equal(t, model.PaymentStatusSuccess, retrieved.Status)
		assert.Nil(t, retrieved.FailureReason)
	})

	t.Run("UpdateGatewayPaymentID", func(t *testing.T) {
		pID := uuid.New()
		p := &model.Payment{
			ID:             pID,
			OrderID:        orderID,
			UserID:         userID,
			StoreID:        storeID,
			AmountPaise:    40000,
			Currency:       "INR",
			Status:         model.PaymentStatusPending,
			Gateway:        "RAZORPAY",
			IdempotencyKey: "idem_key_update_gw",
		}

		tx, err := pool.Begin(ctx)
		assert.NoError(t, err)
		err = repo.Create(ctx, tx, p)
		assert.NoError(t, err)
		err = tx.Commit(ctx)
		assert.NoError(t, err)

		err = repo.UpdateGatewayPaymentID(ctx, pID.String(), "gateway_payment_789")
		assert.NoError(t, err)

		retrieved, err := repo.GetByID(ctx, pID.String())
		assert.NoError(t, err)
		assert.Equal(t, "gateway_payment_789", *retrieved.GatewayPaymentID)
	})

	t.Run("GetByUserID", func(t *testing.T) {
		pID := uuid.New()
		p := &model.Payment{
			ID:             pID,
			OrderID:        orderID,
			UserID:         userID,
			StoreID:        storeID,
			AmountPaise:    50000,
			Currency:       "INR",
			Status:         model.PaymentStatusPending,
			Gateway:        "RAZORPAY",
			IdempotencyKey: "idem_key_get_user",
		}

		tx, err := pool.Begin(ctx)
		assert.NoError(t, err)
		err = repo.Create(ctx, tx, p)
		assert.NoError(t, err)
		err = tx.Commit(ctx)
		assert.NoError(t, err)

		payments, err := repo.GetByUserID(ctx, userID.String(), 10, 0)
		assert.NoError(t, err)
		assert.NotEmpty(t, payments)

		found := false
		for _, py := range payments {
			if py.ID == pID {
				found = true
				break
			}
		}
		assert.True(t, found)
	})
}

func stringPtr(s string) *string {
	return &s
}
