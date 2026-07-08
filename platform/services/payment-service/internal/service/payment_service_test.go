package service

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/zippyra/platform/services/payment-service/internal/model"
	"github.com/zippyra/platform/services/payment-service/internal/repository"
	"github.com/zippyra/platform/services/payment-service/internal/testutil"
)

type MockRoundTripper struct {
	RoundTripFn func(req *http.Request) (*http.Response, error)
}

func (m *MockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.RoundTripFn(req)
}

func setupTestService(t *testing.T) (*pgxpool.Pool, *PaymentService, func()) {
	pool, dbCleanup := testutil.SetupDB()
	_, redisCleanup := newTestRedis(t)

	payRepo := repository.NewPaymentRepository(pool)
	outboxRepo := repository.NewOutboxRepository(pool)

	// Custom HTTP transport to mock outer API responses
	mockTransport := &MockRoundTripper{
		RoundTripFn: func(req *http.Request) (*http.Response, error) {
			if req.URL.Path == "/v1/orders" {
				order := RazorpayOrder{
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
				refund := RazorpayRefund{
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

	rzpClient := NewRazorpayClient("key", "secret")
	rzpClient.client = &http.Client{Transport: mockTransport}

	cfClient := NewCashfreeClient("appid", "secret")
	cfClient.client = &http.Client{Transport: mockTransport}

	router := NewGatewayRouter(rzpClient, cfClient)
	svc := NewPaymentService(pool, payRepo, outboxRepo, router)

	cleanup := func() {
		dbCleanup()
		redisCleanup()
	}

	return pool, svc, cleanup
}

func TestPaymentService_InitiatePayment_Success(t *testing.T) {
	pool, svc, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()
	userID, storeID, orderID, err := testutil.SeedBaseData(ctx, pool)
	assert.NoError(t, err)

	paymentMethod := model.PaymentMethodUPI
	p := &model.Payment{
		OrderID:        orderID,
		UserID:         userID,
		StoreID:        storeID,
		AmountPaise:    50000,
		Currency:       "INR",
		Status:         model.PaymentStatusPending,
		PaymentMethod:  &paymentMethod,
		IdempotencyKey: "idem_key_service_initiate",
	}

	res, err := svc.InitiatePayment(ctx, p)
	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.Equal(t, model.PaymentStatusPending, res.Status)
	assert.Equal(t, "order_rzp_mock", *res.GatewayOrderID)
	assert.Equal(t, "RAZORPAY", res.Gateway)

	// Test idempotency returning same payment
	res2, err := svc.InitiatePayment(ctx, p)
	assert.NoError(t, err)
	assert.Equal(t, res.ID, res2.ID)
}

func TestPaymentService_InitiateRefund_Success(t *testing.T) {
	pool, svc, cleanup := setupTestService(t)
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
		AmountPaise:      50000,
		Currency:         "INR",
		Status:           model.PaymentStatusSuccess,
		Gateway:          "RAZORPAY",
		GatewayPaymentID: stringPtr("gateway_pay_id"),
		IdempotencyKey:   "idem_key_refund",
	}

	tx, err := svc.pool.Begin(ctx)
	assert.NoError(t, err)
	err = svc.repo.Create(ctx, tx, p)
	assert.NoError(t, err)
	err = tx.Commit(ctx)
	assert.NoError(t, err)

	// Since we inserted the payment as PENDING, let's update it to SUCCESS first so we can refund it
	err = svc.repo.UpdateStatus(ctx, pID.String(), string(model.PaymentStatusSuccess), "")
	assert.NoError(t, err)

	// Now run refund
	err = svc.InitiateRefund(ctx, pID.String(), 50000)
	assert.NoError(t, err)

	// Verify payment updated to REFUNDED in DB
	updated, err := svc.repo.GetByID(ctx, pID.String())
	assert.NoError(t, err)
	assert.Equal(t, model.PaymentStatusRefunded, updated.Status)
	assert.Equal(t, "refund_rzp_mock", *updated.RefundID)
	assert.Equal(t, int64(50000), updated.RefundAmountPaise)
}

func TestPaymentService_HandlePaymentCaptured_Success(t *testing.T) {
	pool, svc, cleanup := setupTestService(t)
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
		AmountPaise:    10000,
		Currency:       "INR",
		Status:         model.PaymentStatusPending,
		Gateway:        "RAZORPAY",
		IdempotencyKey: "idem_key_service_capture",
	}

	tx, err := svc.pool.Begin(ctx)
	assert.NoError(t, err)
	err = svc.repo.Create(ctx, tx, p)
	assert.NoError(t, err)
	err = tx.Commit(ctx)
	assert.NoError(t, err)

	event := model.RazorpayWebhookEvent{}
	event.EventID = "evt_id_service_cap"
	event.Payload.Payment.Entity.ID = "rzp_payment_success"
	event.Payload.Payment.Entity.Notes.PaymentID = pID.String()

	err = svc.HandlePaymentCaptured(ctx, event)
	assert.NoError(t, err)

	// Verify DB state
	payment, err := svc.repo.GetByID(ctx, pID.String())
	assert.NoError(t, err)
	assert.Equal(t, model.PaymentStatusSuccess, payment.Status)
	assert.Equal(t, "rzp_payment_success", *payment.GatewayPaymentID)
}

func stringPtr(s string) *string {
	return &s
}
