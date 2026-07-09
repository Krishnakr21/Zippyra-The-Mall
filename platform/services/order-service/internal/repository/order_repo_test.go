package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/zippyra/platform/services/order-service/internal/model"
	"github.com/zippyra/platform/services/order-service/internal/testutil"
)

func TestOrderRepository_CRUD(t *testing.T) {
	pool, dbCleanup := testutil.SetupDB()
	defer dbCleanup()

	repo := NewOrderRepository(pool)
	ctx := context.Background()

	userID, storeID, _, err := testutil.SeedBaseData(ctx, pool)
	assert.NoError(t, err)

	orderID := uuid.New()
	paymentID := uuid.New()
	expiresAt := time.Now().Add(10 * time.Minute)
	exitToken := "some_jwt_token"

	order := &model.Order{
		ID:                 orderID,
		UserID:             userID,
		StoreID:            storeID,
		OrderNumber:        "ZP-2026-REPO-TEST",
		Status:             "PAID",
		TotalAmount:        200.00,
		PaymentID:          &paymentID,
		ExitToken:          &exitToken,
		ExitTokenExpiresAt: &expiresAt,
	}

	tx, err := pool.Begin(ctx)
	assert.NoError(t, err)

	// UpsertOrder
	upserted, isNew, err := repo.UpsertOrder(ctx, tx, order)
	assert.NoError(t, err)
	assert.True(t, isNew)
	assert.NotNil(t, upserted)

	err = tx.Commit(ctx)
	assert.NoError(t, err)

	// GetByID
	retrieved, err := repo.GetByID(ctx, orderID.String())
	assert.NoError(t, err)
	assert.NotNil(t, retrieved)
	assert.Equal(t, orderID, retrieved.ID)
	assert.Equal(t, "PAID", retrieved.Status)

	// GetByPaymentID
	retrievedByPayment, err := repo.GetByPaymentID(ctx, pool, paymentID)
	assert.NoError(t, err)
	assert.NotNil(t, retrievedByPayment)
	assert.Equal(t, orderID, retrievedByPayment.ID)

	// GetHistory
	history, err := repo.GetHistory(ctx, userID.String(), &storeID, 10, 0)
	assert.NoError(t, err)
	assert.Len(t, history, 1)
	assert.Equal(t, orderID, history[0].ID)
}

func TestExitTokenRepository_CRUD(t *testing.T) {
	pool, dbCleanup := testutil.SetupDB()
	defer dbCleanup()

	repo := NewExitTokenRepository(pool)
	orderRepo := NewOrderRepository(pool)
	ctx := context.Background()

	userID, storeID, _, err := testutil.SeedBaseData(ctx, pool)
	assert.NoError(t, err)

	orderID := uuid.New()
	order := &model.Order{
		ID:          orderID,
		UserID:      userID,
		StoreID:     storeID,
		OrderNumber: "ZP-2026-ET-REPO",
		Status:      "PAID",
		TotalAmount: 100.00,
	}

	tx, err := pool.Begin(ctx)
	assert.NoError(t, err)
	_, _, err = orderRepo.UpsertOrder(ctx, tx, order)
	assert.NoError(t, err)
	err = tx.Commit(ctx)
	assert.NoError(t, err)

	et := &model.ExitToken{
		ID:        uuid.New(),
		OrderID:   orderID,
		UserID:    userID,
		StoreID:   storeID,
		TokenHash: "sha256_hash_here",
		ExpiresAt: time.Now().Add(10 * time.Minute),
		IsUsed:    false,
	}

	// Create
	err = repo.Create(ctx, et)
	assert.NoError(t, err)

	// GetActiveByOrderID
	retrieved, err := repo.GetActiveByOrderID(ctx, orderID.String())
	assert.NoError(t, err)
	assert.NotNil(t, retrieved)
	assert.Equal(t, "sha256_hash_here", retrieved.TokenHash)
	assert.False(t, retrieved.IsUsed)
}
