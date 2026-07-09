package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/zippyra/platform/services/order-service/internal/model"
	"github.com/zippyra/platform/services/order-service/internal/repository"
	"github.com/zippyra/platform/services/order-service/internal/testutil"
)

func setupTestReturnService(t *testing.T) (*pgxpool.Pool, *ReturnService, *repository.OrderRepository, *repository.OrderItemRepository, *repository.ReturnRepository, redisCmdableWrapper, *MockPublisher, func()) {
	pool, dbCleanup := testutil.SetupDB()
	rdb, redisCleanup := newTestRedis(t)

	orderRepo := repository.NewOrderRepository(pool)
	orderItemRepo := repository.NewOrderItemRepository(pool)
	returnRepo := repository.NewReturnRepository(pool)

	mockPublisher := &MockPublisher{
		PublishedMsgs: make(map[string]interface{}),
	}

	svc := NewReturnService(orderRepo, orderItemRepo, returnRepo, rdb, mockPublisher, "http://localhost:8086")

	cleanup := func() {
		dbCleanup()
		redisCleanup()
	}

	return pool, svc, orderRepo, orderItemRepo, returnRepo, rdb, mockPublisher, cleanup
}

func TestReturnService_CreateReturn_Success(t *testing.T) {
	pool, svc, orderRepo, orderItemRepo, returnRepo, _, _, cleanup := setupTestReturnService(t)
	defer cleanup()

	ctx := context.Background()
	userID, storeID, productID, err := testutil.SeedBaseData(ctx, pool)
	assert.NoError(t, err)

	// Create an order
	orderID := uuid.New()
	returnEnds := time.Now().Add(12 * time.Hour)
	order := &model.Order{
		ID:                 orderID,
		UserID:             userID,
		StoreID:            storeID,
		OrderNumber:        "ZP-2026-RETURNOK",
		Status:             "PAID",
		TotalAmount:        100.00,
		ReturnWindowEndsAt: &returnEnds,
	}

	tx, err := pool.Begin(ctx)
	assert.NoError(t, err)
	_, _, err = orderRepo.UpsertOrder(ctx, tx, order)
	assert.NoError(t, err)

	itemID := uuid.New()
	item := model.OrderItem{
		ID:         itemID,
		OrderID:    orderID,
		ProductID:  productID,
		Barcode:    "123456789012",
		Quantity:   1,
		UnitPrice:  100.00,
		TotalPrice: 100.00,
		HSNCode:    "123456",
	}
	err = orderItemRepo.CreateItems(ctx, tx, []model.OrderItem{item})
	assert.NoError(t, err)
	err = tx.Commit(ctx)
	assert.NoError(t, err)

	// Call CreateReturn
	ret, err := svc.CreateReturn(ctx, userID.String(), orderID.String(), []string{itemID.String()}, "damaged item")
	assert.NoError(t, err)
	assert.NotNil(t, ret)
	assert.Equal(t, model.ReturnStatusPendingApproval, ret.Status)

	// Verify request exists in DB
	dbRet, err := returnRepo.GetReturnRequestByID(ctx, ret.ID.String())
	assert.NoError(t, err)
	assert.NotNil(t, dbRet)
	assert.Equal(t, model.ReturnStatusPendingApproval, dbRet.Status)
	assert.Len(t, dbRet.Items, 1)
	assert.Equal(t, itemID.String(), dbRet.Items[0])
}

func TestReturnService_CreateReturn_ExpiredWindow(t *testing.T) {
	pool, svc, orderRepo, orderItemRepo, _, _, _, cleanup := setupTestReturnService(t)
	defer cleanup()

	ctx := context.Background()
	userID, storeID, productID, err := testutil.SeedBaseData(ctx, pool)
	assert.NoError(t, err)

	// Create an order with expired return window (e.g. return ends in the past)
	orderID := uuid.New()
	returnEnds := time.Now().Add(-1 * time.Hour)
	order := &model.Order{
		ID:                 orderID,
		UserID:             userID,
		StoreID:            storeID,
		OrderNumber:        "ZP-2026-RETURNOK",
		Status:             "PAID",
		TotalAmount:        100.00,
		ReturnWindowEndsAt: &returnEnds,
	}

	tx, err := pool.Begin(ctx)
	assert.NoError(t, err)
	_, _, err = orderRepo.UpsertOrder(ctx, tx, order)
	assert.NoError(t, err)

	itemID := uuid.New()
	item := model.OrderItem{
		ID:         itemID,
		OrderID:    orderID,
		ProductID:  productID,
		Barcode:    "123456789012",
		Quantity:   1,
		UnitPrice:  100.00,
		TotalPrice: 100.00,
		HSNCode:    "123456",
	}
	err = orderItemRepo.CreateItems(ctx, tx, []model.OrderItem{item})
	assert.NoError(t, err)
	err = tx.Commit(ctx)
	assert.NoError(t, err)

	_, err = svc.CreateReturn(ctx, userID.String(), orderID.String(), []string{itemID.String()}, "damaged item")
	assert.ErrorIs(t, err, ErrReturnWindowClosed)
}

func TestReturnService_CreateReturn_ItemNotReturnable(t *testing.T) {
	pool, svc, orderRepo, orderItemRepo, _, _, _, cleanup := setupTestReturnService(t)
	defer cleanup()

	ctx := context.Background()
	userID, storeID, productID, err := testutil.SeedBaseData(ctx, pool)
	assert.NoError(t, err)

	// Make product non-returnable in database
	_, err = pool.Exec(ctx, "UPDATE products SET is_returnable = false WHERE id = $1", productID)
	assert.NoError(t, err)

	orderID := uuid.New()
	returnEnds := time.Now().Add(12 * time.Hour)
	order := &model.Order{
		ID:                 orderID,
		UserID:             userID,
		StoreID:            storeID,
		OrderNumber:        "ZP-2026-RETURNOK",
		Status:             "PAID",
		TotalAmount:        100.00,
		ReturnWindowEndsAt: &returnEnds,
	}

	tx, err := pool.Begin(ctx)
	assert.NoError(t, err)
	_, _, err = orderRepo.UpsertOrder(ctx, tx, order)
	assert.NoError(t, err)

	itemID := uuid.New()
	item := model.OrderItem{
		ID:         itemID,
		OrderID:    orderID,
		ProductID:  productID,
		Barcode:    "123456789012",
		Quantity:   1,
		UnitPrice:  100.00,
		TotalPrice: 100.00,
		HSNCode:    "123456",
	}
	err = orderItemRepo.CreateItems(ctx, tx, []model.OrderItem{item})
	assert.NoError(t, err)
	err = tx.Commit(ctx)
	assert.NoError(t, err)

	_, err = svc.CreateReturn(ctx, userID.String(), orderID.String(), []string{itemID.String()}, "damaged item")
	assert.ErrorIs(t, err, ErrItemNotReturnable)
}

func TestReturnService_CreateReturn_DuplicateReturn(t *testing.T) {
	pool, svc, orderRepo, orderItemRepo, _, _, _, cleanup := setupTestReturnService(t)
	defer cleanup()

	ctx := context.Background()
	userID, storeID, productID, err := testutil.SeedBaseData(ctx, pool)
	assert.NoError(t, err)

	orderID := uuid.New()
	returnEnds := time.Now().Add(12 * time.Hour)
	order := &model.Order{
		ID:                 orderID,
		UserID:             userID,
		StoreID:            storeID,
		OrderNumber:        "ZP-2026-RETURNDUP",
		Status:             "PAID",
		TotalAmount:        100.00,
		ReturnWindowEndsAt: &returnEnds,
	}

	tx, err := pool.Begin(ctx)
	assert.NoError(t, err)
	_, _, err = orderRepo.UpsertOrder(ctx, tx, order)
	assert.NoError(t, err)

	itemID := uuid.New()
	item := model.OrderItem{
		ID:         itemID,
		OrderID:    orderID,
		ProductID:  productID,
		Barcode:    "123456789012",
		Quantity:   1,
		UnitPrice:  100.00,
		TotalPrice: 100.00,
		HSNCode:    "123456",
	}
	err = orderItemRepo.CreateItems(ctx, tx, []model.OrderItem{item})
	assert.NoError(t, err)
	err = tx.Commit(ctx)
	assert.NoError(t, err)

	// First Request
	_, err = svc.CreateReturn(ctx, userID.String(), orderID.String(), []string{itemID.String()}, "damaged item")
	assert.NoError(t, err)

	// Second Request (should fail)
	_, err = svc.CreateReturn(ctx, userID.String(), orderID.String(), []string{itemID.String()}, "damaged item")
	assert.ErrorIs(t, err, ErrReturnAlreadyExists)
}

func TestReturnService_AcceptReturn_Success(t *testing.T) {
	pool, svc, orderRepo, orderItemRepo, returnRepo, _, mockPublisher, cleanup := setupTestReturnService(t)
	defer cleanup()

	ctx := context.Background()
	userID, storeID, productID, err := testutil.SeedBaseData(ctx, pool)
	assert.NoError(t, err)

	orderID := uuid.New()
	paymentID := uuid.New()
	order := &model.Order{
		ID:          orderID,
		UserID:      userID,
		StoreID:     storeID,
		OrderNumber: "ZP-2026-RETURNACCEPT",
		Status:      "PAID",
		TotalAmount: 100.00,
		PaymentID:   &paymentID,
	}

	tx, err := pool.Begin(ctx)
	assert.NoError(t, err)
	_, _, err = orderRepo.UpsertOrder(ctx, tx, order)
	assert.NoError(t, err)

	itemID := uuid.New()
	item := model.OrderItem{
		ID:         itemID,
		OrderID:    orderID,
		ProductID:  productID,
		Barcode:    "123456789012",
		Quantity:   2, // Return both
		UnitPrice:  50.00,
		TotalPrice: 100.00,
		HSNCode:    "123456",
	}
	err = orderItemRepo.CreateItems(ctx, tx, []model.OrderItem{item})
	assert.NoError(t, err)
	err = tx.Commit(ctx)
	assert.NoError(t, err)

	// Seed Return Request
	retReq := &model.ReturnRequest{
		ID:              uuid.New(),
		OrderID:         orderID,
		UserID:          userID,
		StoreID:         storeID,
		Status:          model.ReturnStatusPendingApproval,
		Reason:          "Damaged",
		Items:           []string{itemID.String()},
		RefundInitiated: false,
	}
	err = returnRepo.CreateReturnRequest(ctx, retReq)
	assert.NoError(t, err)

	// Setup Mock Refund Server to return 200 OK
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Update svc with mock server URL
	svc.paymentServiceURL = server.URL

	// Accept Return
	err = svc.AcceptReturn(ctx, retReq.ID.String(), []string{itemID.String()})
	assert.NoError(t, err)

	// Assert: status is ACCEPTED
	updatedRet, err := returnRepo.GetReturnRequestByID(ctx, retReq.ID.String())
	assert.NoError(t, err)
	assert.NotNil(t, updatedRet)
	assert.Equal(t, model.ReturnStatusAccepted, updatedRet.Status)
	assert.True(t, updatedRet.RefundInitiated)

	// Assert: database product stock incremented
	prod, err := orderItemRepo.GetProductByID(ctx, productID.String())
	assert.NoError(t, err)
	assert.NotNil(t, prod)
	assert.Equal(t, 12, prod.StockQuantity) // Seeded with 10 + 2 restored = 12

	// Assert: Kafka events published
	mockPublisher.lock.Lock()
	inventoryEvent := mockPublisher.PublishedMsgs["inventory.movement"].(model.InventoryMovementEvent)
	loyaltyEvent := mockPublisher.PublishedMsgs["loyalty.points_reversed"].(model.LoyaltyPointsReversedEvent)
	mockPublisher.lock.Unlock()

	assert.Equal(t, "inventory.movement", inventoryEvent.EventType)
	assert.Equal(t, productID.String(), inventoryEvent.ProductID)
	assert.Equal(t, 2, inventoryEvent.Quantity)

	assert.Equal(t, "loyalty.points_reversed", loyaltyEvent.EventType)
	assert.Equal(t, 100.00, loyaltyEvent.Amount)
}
