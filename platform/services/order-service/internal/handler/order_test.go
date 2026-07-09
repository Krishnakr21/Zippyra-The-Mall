package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/zippyra/platform/services/order-service/internal/model"
	"github.com/zippyra/platform/services/order-service/internal/repository"
	"github.com/zippyra/platform/services/order-service/internal/service"
	"github.com/zippyra/platform/services/order-service/internal/testutil"
	"github.com/zippyra/platform/shared/jwt"
	"github.com/zippyra/platform/shared/middleware"
)

type MockJWTService struct{}

func (m MockJWTService) GenerateExitToken(claims jwt.ExitTokenClaims) (string, error) {
	return "mock_exit_token_handler_jwt", nil
}

type MockPublisher struct{}

func (m MockPublisher) Publish(ctx context.Context, topic, key string, value interface{}) error {
	return nil
}

func newTestRedis(t *testing.T) (*redis.Client, func()) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "zippyra_local",
		DB:       2,
	})
	ctx := context.Background()
	err := rdb.Ping(ctx).Err()
	assert.NoError(t, err)
	cleanup := func() {
		_ = rdb.FlushDB(ctx)
		_ = rdb.Close()
	}
	return rdb, cleanup
}

func setupTestHandler(t *testing.T) (*pgxpool.Pool, *OrderHandler, func()) {
	pool, dbCleanup := testutil.SetupDB()
	_, redisCleanup := newTestRedis(t)

	orderRepo := repository.NewOrderRepository(pool)
	orderItemRepo := repository.NewOrderItemRepository(pool)
	exitTokenRepo := repository.NewExitTokenRepository(pool)

	mockPublisher := MockPublisher{}

	exitTokenSvc := service.NewExitTokenService(exitTokenRepo, orderRepo, nil, MockJWTService{})
	invoiceSvc := service.NewInvoiceService(orderRepo, orderItemRepo, mockPublisher)
	orderSvc := service.NewOrderService(pool, orderRepo, orderItemRepo, exitTokenSvc, invoiceSvc, mockPublisher)

	handler := NewOrderHandler(orderSvc, exitTokenSvc)

	cleanup := func() {
		dbCleanup()
		redisCleanup()
	}

	return pool, handler, cleanup
}

func TestOrderHandler_GetByID_Success(t *testing.T) {
	pool, handler, cleanup := setupTestHandler(t)
	defer cleanup()

	ctx := context.Background()
	userID, storeID, _, err := testutil.SeedBaseData(ctx, pool)
	assert.NoError(t, err)

	orderID := uuid.New()
	order := &model.Order{
		ID:          orderID,
		UserID:      userID,
		StoreID:     storeID,
		OrderNumber: "ZP-2026-GETOK",
		Status:      "PAID",
		TotalAmount: 150.00,
	}

	tx, err := pool.Begin(ctx)
	assert.NoError(t, err)
	orderRepo := repository.NewOrderRepository(pool)
	_, _, err = orderRepo.UpsertOrder(ctx, tx, order)
	assert.NoError(t, err)
	err = tx.Commit(ctx)
	assert.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/orders/"+orderID.String(), nil)
	// Inject Claims
	claims := &jwt.ZippyraClaims{
		UserID:   userID.String(),
		UserType: jwt.UserTypeCustomer,
	}
	req = req.WithContext(middleware.ContextWithClaims(req.Context(), claims))

	// chi routing params
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", orderID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	handler.GetByID(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp model.Order
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, orderID, resp.ID)
	assert.Equal(t, userID, resp.UserID)
}

func TestOrderHandler_GetByID_Forbidden(t *testing.T) {
	pool, handler, cleanup := setupTestHandler(t)
	defer cleanup()

	ctx := context.Background()
	userID, storeID, _, err := testutil.SeedBaseData(ctx, pool)
	assert.NoError(t, err)

	orderID := uuid.New()
	order := &model.Order{
		ID:          orderID,
		UserID:      userID,
		StoreID:     storeID,
		OrderNumber: "ZP-2026-GETFORBID",
		Status:      "PAID",
		TotalAmount: 150.00,
	}

	tx, err := pool.Begin(ctx)
	assert.NoError(t, err)
	orderRepo := repository.NewOrderRepository(pool)
	_, _, err = orderRepo.UpsertOrder(ctx, tx, order)
	assert.NoError(t, err)
	err = tx.Commit(ctx)
	assert.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/orders/"+orderID.String(), nil)
	// Inject Claims for ANOTHER user
	claims := &jwt.ZippyraClaims{
		UserID:   uuid.New().String(),
		UserType: jwt.UserTypeCustomer,
	}
	req = req.WithContext(middleware.ContextWithClaims(req.Context(), claims))

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", orderID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	handler.GetByID(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestOrderHandler_GetHistory_Success(t *testing.T) {
	pool, handler, cleanup := setupTestHandler(t)
	defer cleanup()

	ctx := context.Background()
	userID, storeID, _, err := testutil.SeedBaseData(ctx, pool)
	assert.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/orders?store_id=%s&page=1&limit=10", storeID.String()), nil)
	claims := &jwt.ZippyraClaims{
		UserID:   userID.String(),
		UserType: jwt.UserTypeCustomer,
	}
	req = req.WithContext(middleware.ContextWithClaims(req.Context(), claims))

	w := httptest.NewRecorder()
	handler.GetHistory(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestOrderHandler_GetExitToken_Success(t *testing.T) {
	pool, handler, cleanup := setupTestHandler(t)
	defer cleanup()

	ctx := context.Background()
	userID, storeID, _, err := testutil.SeedBaseData(ctx, pool)
	assert.NoError(t, err)

	orderID := uuid.New()
	expiresAt := time.Now().Add(5 * time.Minute)
	exitToken := "mock_jwt"
	order := &model.Order{
		ID:                 orderID,
		UserID:             userID,
		StoreID:            storeID,
		OrderNumber:        "ZP-2026-GETTOKEN",
		Status:             "PAID",
		TotalAmount:        150.00,
		ExitToken:          &exitToken,
		ExitTokenExpiresAt: &expiresAt,
	}

	tx, err := pool.Begin(ctx)
	assert.NoError(t, err)
	orderRepo := repository.NewOrderRepository(pool)
	_, _, err = orderRepo.UpsertOrder(ctx, tx, order)
	assert.NoError(t, err)
	err = tx.Commit(ctx)
	assert.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/orders/"+orderID.String()+"/exit-token", nil)
	claims := &jwt.ZippyraClaims{
		UserID:   userID.String(),
		UserType: jwt.UserTypeCustomer,
	}
	req = req.WithContext(middleware.ContextWithClaims(req.Context(), claims))

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", orderID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	handler.GetExitToken(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]string
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "mock_jwt", resp["exit_token"])
}
