package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/zippyra/platform/services/payment-service/internal/model"
	"github.com/zippyra/platform/services/payment-service/internal/repository"
	"github.com/zippyra/platform/services/payment-service/internal/service"
	"github.com/zippyra/platform/services/payment-service/internal/testutil"
	"github.com/zippyra/platform/shared/jwt"
	"github.com/zippyra/platform/shared/middleware"
)

type MockRoundTripper struct {
	RoundTripFn func(req *http.Request) (*http.Response, error)
}

func (m *MockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.RoundTripFn(req)
}

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

func setupTestHandler(t *testing.T) (*pgxpool.Pool, *PaymentHandler, func()) {
	pool, dbCleanup := testutil.SetupDB()
	_, redisCleanup := newTestRedis(t)

	payRepo := repository.NewPaymentRepository(pool)
	outboxRepo := repository.NewOutboxRepository(pool)

	mockTransport := &MockRoundTripper{
		RoundTripFn: func(req *http.Request) (*http.Response, error) {
			if req.URL.Path == "/v1/orders" {
				order := service.RazorpayOrder{
					ID:       "order_rzp_mock",
					Entity:   "order",
					Amount:   50000,
					Currency: "INR",
					Status:   "created",
				}
				b, _ := json.Marshal(order)
				return &http.Response{
					StatusCode: 200,
					Body:       io.NopCloser(bytes.NewReader(b)),
					Header:     make(http.Header),
				}, nil
			}
			if req.URL.Path == "/v1/payments/gateway_pay_id/refund" || (req.URL.Path != "" && req.URL.Path[len(req.URL.Path)-7:] == "/refund") {
				refund := service.RazorpayRefund{
					ID:        "refund_rzp_mock",
					Entity:    "refund",
					Amount:    50000,
					Currency:  "INR",
					PaymentID: "gateway_pay_id",
				}
				b, _ := json.Marshal(refund)
				return &http.Response{
					StatusCode: 200,
					Body:       io.NopCloser(bytes.NewReader(b)),
					Header:     make(http.Header),
				}, nil
			}
			return &http.Response{
				StatusCode: 400,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{"error":"bad request"}`))),
				Header:     make(http.Header),
			}, nil
		},
	}

	rzpClient := service.NewRazorpayClient("key", "secret")
	rzpClient.SetHTTPClient(&http.Client{Transport: mockTransport})

	cfClient := service.NewCashfreeClient("appid", "secret")
	cfClient.SetHTTPClient(&http.Client{Transport: mockTransport})

	router := service.NewGatewayRouter(rzpClient, cfClient)
	svc := service.NewPaymentService(pool, payRepo, outboxRepo, router)
	h := NewPaymentHandler(svc)

	cleanup := func() {
		dbCleanup()
		redisCleanup()
	}

	return pool, h, cleanup
}

func TestPaymentHandler_Initiate_Success(t *testing.T) {
	pool, h, cleanup := setupTestHandler(t)
	defer cleanup()

	ctx := context.Background()
	userID, storeID, orderID, err := testutil.SeedBaseData(ctx, pool)
	assert.NoError(t, err)

	claims := &jwt.ZippyraClaims{
		UserID: userID.String(),
	}

	reqBody := `{"order_id":"` + orderID.String() + `","store_id":"` + storeID.String() + `","amount_paise":50000,"payment_method":"UPI","idempotency_key":"idem_handler_initiate"}`
	req := httptest.NewRequest("POST", "/", strings.NewReader(reqBody))
	req = req.WithContext(middleware.ContextWithClaims(req.Context(), claims))
	rr := httptest.NewRecorder()

	h.Initiate(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp model.Payment
	err = json.NewDecoder(rr.Body).Decode(&resp)
	assert.NoError(t, err)
	assert.Equal(t, orderID, resp.OrderID)
	assert.Equal(t, userID, resp.UserID)
	assert.Equal(t, int64(50000), resp.AmountPaise)
	assert.Equal(t, "order_rzp_mock", *resp.GatewayOrderID)
}

func TestPaymentHandler_Status_Success(t *testing.T) {
	pool, h, cleanup := setupTestHandler(t)
	defer cleanup()

	ctx := context.Background()
	userID, storeID, orderID, err := testutil.SeedBaseData(ctx, pool)
	assert.NoError(t, err)

	pID := uuid.New()
	p := &model.Payment{
		ID:             pID,
		OrderID:        orderID,
		UserID:         userID,
		StoreID:        storeID,
		AmountPaise:    12000,
		Currency:       "INR",
		Status:         model.PaymentStatusPending,
		Gateway:        "RAZORPAY",
		IdempotencyKey: "idem_handler_status",
	}

	tx, err := pool.Begin(ctx)
	assert.NoError(t, err)
	payRepo := repository.NewPaymentRepository(pool)
	err = payRepo.Create(ctx, tx, p)
	assert.NoError(t, err)
	err = tx.Commit(ctx)
	assert.NoError(t, err)

	claims := &jwt.ZippyraClaims{
		UserID: userID.String(),
	}

	req := httptest.NewRequest("GET", "/"+pID.String(), nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("payment_id", pID.String())
	reqCtx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	req = req.WithContext(middleware.ContextWithClaims(reqCtx, claims))
	rr := httptest.NewRecorder()

	h.Status(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp model.Payment
	err = json.NewDecoder(rr.Body).Decode(&resp)
	assert.NoError(t, err)
	assert.Equal(t, pID, resp.ID)
	assert.Equal(t, model.PaymentStatusPending, resp.Status)
}

func TestPaymentHandler_History_Success(t *testing.T) {
	pool, h, cleanup := setupTestHandler(t)
	defer cleanup()

	ctx := context.Background()
	userID, storeID, orderID, err := testutil.SeedBaseData(ctx, pool)
	assert.NoError(t, err)

	pID := uuid.New()
	p := &model.Payment{
		ID:             pID,
		OrderID:        orderID,
		UserID:         userID,
		StoreID:        storeID,
		AmountPaise:    12000,
		Currency:       "INR",
		Status:         model.PaymentStatusPending,
		Gateway:        "RAZORPAY",
		IdempotencyKey: "idem_handler_history",
	}

	tx, err := pool.Begin(ctx)
	assert.NoError(t, err)
	payRepo := repository.NewPaymentRepository(pool)
	err = payRepo.Create(ctx, tx, p)
	assert.NoError(t, err)
	err = tx.Commit(ctx)
	assert.NoError(t, err)

	claims := &jwt.ZippyraClaims{
		UserID: userID.String(),
	}

	req := httptest.NewRequest("GET", "/history?limit=10&offset=0", nil)
	req = req.WithContext(middleware.ContextWithClaims(req.Context(), claims))
	rr := httptest.NewRecorder()

	h.History(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp []model.Payment
	err = json.NewDecoder(rr.Body).Decode(&resp)
	assert.NoError(t, err)
	assert.NotEmpty(t, resp)
	assert.Equal(t, pID, resp[0].ID)
}

func TestPaymentHandler_Refund_Success(t *testing.T) {
	pool, h, cleanup := setupTestHandler(t)
	defer cleanup()

	ctx := context.Background()
	userID, storeID, orderID, err := testutil.SeedBaseData(ctx, pool)
	assert.NoError(t, err)

	pID := uuid.New()
	p := &model.Payment{
		ID:               pID,
		OrderID:          orderID,
		UserID:           userID,
		StoreID:          storeID,
		AmountPaise:      25000,
		Currency:         "INR",
		Status:           model.PaymentStatusSuccess,
		Gateway:          "RAZORPAY",
		GatewayPaymentID: stringPtr("gateway_pay_id"),
		IdempotencyKey:   "idem_handler_refund",
	}

	tx, err := pool.Begin(ctx)
	assert.NoError(t, err)
	payRepo := repository.NewPaymentRepository(pool)
	err = payRepo.Create(ctx, tx, p)
	assert.NoError(t, err)
	err = tx.Commit(ctx)
	assert.NoError(t, err)

	// Update it to SUCCESS
	err = payRepo.UpdateStatus(ctx, pID.String(), string(model.PaymentStatusSuccess), "")
	assert.NoError(t, err)

	claims := &jwt.ZippyraClaims{
		UserID: userID.String(),
	}

	reqBody := `{"amount_paise":25000}`
	req := httptest.NewRequest("POST", "/"+pID.String()+"/refund", strings.NewReader(reqBody))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("payment_id", pID.String())
	reqCtx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	req = req.WithContext(middleware.ContextWithClaims(reqCtx, claims))
	rr := httptest.NewRecorder()

	h.Refund(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func stringPtr(s string) *string {
	return &s
}
