package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/zippyra/platform/services/order-service/internal/model"
	"github.com/zippyra/platform/services/order-service/internal/repository"
	"github.com/zippyra/platform/services/order-service/internal/service"
	"github.com/zippyra/platform/services/order-service/internal/testutil"
	"github.com/zippyra/platform/shared/jwt"
	"github.com/zippyra/platform/shared/middleware"
)

func setupTestReturnHandler(t *testing.T) (*pgxpool.Pool, *ReturnHandler, *service.ReturnService, func()) {
	pool, dbCleanup := testutil.SetupDB()
	rdb, redisCleanup := newTestRedis(t)

	orderRepo := repository.NewOrderRepository(pool)
	orderItemRepo := repository.NewOrderItemRepository(pool)
	returnRepo := repository.NewReturnRepository(pool)

	mockPublisher := MockPublisher{}

	svc := service.NewReturnService(orderRepo, orderItemRepo, returnRepo, rdb, mockPublisher, "http://localhost:8086")
	handler := NewReturnHandler(svc)

	cleanup := func() {
		dbCleanup()
		redisCleanup()
	}

	return pool, handler, svc, cleanup
}

func TestReturnHandler_Create_Success(t *testing.T) {
	pool, handler, _, cleanup := setupTestReturnHandler(t)
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
		OrderNumber:        "ZP-2026-RET-API",
		Status:             "PAID",
		TotalAmount:        100.00,
		ReturnWindowEndsAt: &returnEnds,
	}

	tx, err := pool.Begin(ctx)
	assert.NoError(t, err)
	orderRepo := repository.NewOrderRepository(pool)
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
	orderItemRepo := repository.NewOrderItemRepository(pool)
	err = orderItemRepo.CreateItems(ctx, tx, []model.OrderItem{item})
	assert.NoError(t, err)
	err = tx.Commit(ctx)
	assert.NoError(t, err)

	body, _ := json.Marshal(CreateReturnRequest{
		ItemIDs: []string{itemID.String()},
		Reason:  "Damaged on arrival",
	})

	req := httptest.NewRequest(http.MethodPost, "/orders/"+orderID.String()+"/returns", bytes.NewReader(body))
	claims := &jwt.ZippyraClaims{
		UserID:   userID.String(),
		UserType: jwt.UserTypeCustomer,
	}
	req = req.WithContext(middleware.ContextWithClaims(req.Context(), claims))

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", orderID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	handler.Create(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestReturnHandler_Accept_Success(t *testing.T) {
	pool, handler, svc, cleanup := setupTestReturnHandler(t)
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
		OrderNumber: "ZP-2026-RET-ACC-API",
		Status:      "PAID",
		TotalAmount: 100.00,
		PaymentID:   &paymentID,
	}

	tx, err := pool.Begin(ctx)
	assert.NoError(t, err)
	orderRepo := repository.NewOrderRepository(pool)
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
	orderItemRepo := repository.NewOrderItemRepository(pool)
	err = orderItemRepo.CreateItems(ctx, tx, []model.OrderItem{item})
	assert.NoError(t, err)
	err = tx.Commit(ctx)
	assert.NoError(t, err)

	returnRepo := repository.NewReturnRepository(pool)
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

	// Setup Mock Payment Refund HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Update svc url
	svc.SetPaymentServiceURL(server.URL)

	body, _ := json.Marshal(AcceptReturnRequest{
		ReturnID:      retReq.ID.String(),
		ItemsVerified: []string{itemID.String()},
	})

	req := httptest.NewRequest(http.MethodPost, "/returns/accept", bytes.NewReader(body))
	claims := &jwt.ZippyraClaims{
		UserID:   userID.String(),
		UserType: jwt.UserTypeStaff, // Must be staff
	}
	req = req.WithContext(middleware.ContextWithClaims(req.Context(), claims))

	w := httptest.NewRecorder()
	handler.Accept(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
