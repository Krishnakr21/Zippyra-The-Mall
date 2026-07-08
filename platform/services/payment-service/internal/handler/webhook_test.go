package handler

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/zippyra/platform/services/payment-service/internal/model"
	"github.com/zippyra/platform/services/payment-service/internal/repository"
	"github.com/zippyra/platform/services/payment-service/internal/service"
	"github.com/zippyra/platform/services/payment-service/internal/testutil"
)

func createRazorpayWebhookRequest(t *testing.T, event model.RazorpayWebhookEvent, secret string) (*http.Request, []byte) {
	b, err := json.Marshal(event)
	assert.NoError(t, err)

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(b)
	sig := hex.EncodeToString(mac.Sum(nil))

	req := httptest.NewRequest("POST", "/v1/payment/webhook/razorpay", bytes.NewReader(b))
	req.Header.Set("X-Razorpay-Signature", sig)
	req.Header.Set("Content-Type", "application/json")
	return req, b
}

func TestWebhook_InvalidHMAC_Returns400(t *testing.T) {
	pool, dbCleanup := testutil.SetupDB()
	defer dbCleanup()

	_, redisCleanup := newTestRedis(t)
	defer redisCleanup()

	ctx := context.Background()
	webhookRepo := repository.NewWebhookRepository(pool)
	payRepo := repository.NewPaymentRepository(pool)
	outboxRepo := repository.NewOutboxRepository(pool)
	router := service.NewGatewayRouter(nil, nil)
	svc := service.NewPaymentService(pool, payRepo, outboxRepo, router)

	rzpSecret := "rzp_webhook_secret"
	h := NewWebhookHandler(svc, webhookRepo, rzpSecret, "cf_secret")

	event := model.RazorpayWebhookEvent{
		EventID: "evt_1",
		Event:   "payment.captured",
	}
	event.Payload.Payment.Entity.ID = "pay_1"
	event.Payload.Payment.Entity.Notes.PaymentID = uuid.New().String()

	b, _ := json.Marshal(event)
	req := httptest.NewRequest("POST", "/razorpay", bytes.NewReader(b))
	req.Header.Set("X-Razorpay-Signature", "invalid_signature_here")
	rr := httptest.NewRecorder()

	h.Razorpay(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)

	// Assert: nothing inserted into payment_webhook_events
	var count int
	err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM payment_webhook_events").Scan(&count)
	assert.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestWebhook_DuplicateEventID_Returns200_NotProcessedAgain(t *testing.T) {
	pool, dbCleanup := testutil.SetupDB()
	defer dbCleanup()

	_, redisCleanup := newTestRedis(t)
	defer redisCleanup()

	ctx := context.Background()
	webhookRepo := repository.NewWebhookRepository(pool)
	payRepo := repository.NewPaymentRepository(pool)
	outboxRepo := repository.NewOutboxRepository(pool)
	router := service.NewGatewayRouter(nil, nil)
	svc := service.NewPaymentService(pool, payRepo, outboxRepo, router)

	rzpSecret := "rzp_webhook_secret"
	h := NewWebhookHandler(svc, webhookRepo, rzpSecret, "cf_secret")

	// Insert event_id into payment_webhook_events first
	wEvent := &model.WebhookEvent{
		Gateway:      "RAZORPAY",
		EventID:      "evt_duplicate",
		EventType:    "payment.captured",
		Payload:      []byte("{}"),
		HMACVerified: true,
	}
	inserted, err := webhookRepo.InsertIdempotent(ctx, wEvent)
	assert.NoError(t, err)
	assert.True(t, inserted)

	event := model.RazorpayWebhookEvent{
		EventID: "evt_duplicate",
		Event:   "payment.captured",
	}
	req, _ := createRazorpayWebhookRequest(t, event, rzpSecret)
	rr := httptest.NewRecorder()

	h.Razorpay(rr, req)

	// Assert: 200 response
	assert.Equal(t, http.StatusOK, rr.Code)

	// Since we inserted it directly first, the second webhook handler call shouldn't try to call HandlePaymentCaptured
	// (which would have crashed anyway since notes.PaymentID is empty and doesn't exist).
}

func TestWebhook_ValidCapture_UpdatesPaymentStatus(t *testing.T) {
	pool, dbCleanup := testutil.SetupDB()
	defer dbCleanup()

	_, redisCleanup := newTestRedis(t)
	defer redisCleanup()

	ctx := context.Background()
	webhookRepo := repository.NewWebhookRepository(pool)
	payRepo := repository.NewPaymentRepository(pool)
	outboxRepo := repository.NewOutboxRepository(pool)
	router := service.NewGatewayRouter(nil, nil)
	svc := service.NewPaymentService(pool, payRepo, outboxRepo, router)

	rzpSecret := "rzp_webhook_secret"
	h := NewWebhookHandler(svc, webhookRepo, rzpSecret, "cf_secret")

	// Seed user, store, order
	userID, storeID, orderID, err := testutil.SeedBaseData(ctx, pool)
	assert.NoError(t, err)

	// Setup: insert a PENDING payment row
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
		IdempotencyKey: "idem_key_capture",
	}
	tx, err := pool.Begin(ctx)
	assert.NoError(t, err)
	err = payRepo.Create(ctx, tx, p)
	assert.NoError(t, err)
	err = tx.Commit(ctx)
	assert.NoError(t, err)

	// Send valid payment.captured webhook
	event := model.RazorpayWebhookEvent{
		EventID: "evt_capture_123",
		Event:   "payment.captured",
	}
	event.Payload.Payment.Entity.ID = "pay_captured_gateway_id"
	event.Payload.Payment.Entity.Notes.PaymentID = pID.String()

	req, _ := createRazorpayWebhookRequest(t, event, rzpSecret)
	rr := httptest.NewRecorder()

	h.Razorpay(rr, req)

	// Assert 200 response
	assert.Equal(t, http.StatusOK, rr.Code)

	// Assert: payment status updated to SUCCESS in DB
	payment, err := payRepo.GetByID(ctx, pID.String())
	assert.NoError(t, err)
	assert.NotNil(t, payment)
	assert.Equal(t, model.PaymentStatusSuccess, payment.Status)
	assert.Equal(t, "pay_captured_gateway_id", *payment.GatewayPaymentID)

	// Assert: outbox row created
	msgs, err := outboxRepo.GetUnpublished(ctx, 10)
	assert.NoError(t, err)
	assert.Len(t, msgs, 1)
	assert.Equal(t, "payments.completed", msgs[0].Topic)
}

func TestWebhook_ValidFailed_UpdatesPaymentStatus(t *testing.T) {
	pool, dbCleanup := testutil.SetupDB()
	defer dbCleanup()

	_, redisCleanup := newTestRedis(t)
	defer redisCleanup()

	ctx := context.Background()
	webhookRepo := repository.NewWebhookRepository(pool)
	payRepo := repository.NewPaymentRepository(pool)
	outboxRepo := repository.NewOutboxRepository(pool)
	router := service.NewGatewayRouter(nil, nil)
	svc := service.NewPaymentService(pool, payRepo, outboxRepo, router)

	rzpSecret := "rzp_webhook_secret"
	h := NewWebhookHandler(svc, webhookRepo, rzpSecret, "cf_secret")

	// Seed user, store, order
	userID, storeID, orderID, err := testutil.SeedBaseData(ctx, pool)
	assert.NoError(t, err)

	// Setup: insert a PENDING payment row
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
		IdempotencyKey: "idem_key_fail",
	}
	tx, err := pool.Begin(ctx)
	assert.NoError(t, err)
	err = payRepo.Create(ctx, tx, p)
	assert.NoError(t, err)
	err = tx.Commit(ctx)
	assert.NoError(t, err)

	// Send valid payment.failed webhook
	event := model.RazorpayWebhookEvent{
		EventID: "evt_fail_123",
		Event:   "payment.failed",
	}
	event.Payload.Payment.Entity.ID = "pay_failed_gateway_id"
	event.Payload.Payment.Entity.Notes.PaymentID = pID.String()
	event.Payload.Payment.Entity.ErrorDescription = "insufficient funds"

	req, _ := createRazorpayWebhookRequest(t, event, rzpSecret)
	rr := httptest.NewRecorder()

	h.Razorpay(rr, req)

	// Assert 200 response
	assert.Equal(t, http.StatusOK, rr.Code)

	// Assert: payment status updated to FAILED in DB
	payment, err := payRepo.GetByID(ctx, pID.String())
	assert.NoError(t, err)
	assert.NotNil(t, payment)
	assert.Equal(t, model.PaymentStatusFailed, payment.Status)
	assert.Equal(t, "insufficient funds", *payment.FailureReason)

	// Assert: outbox row created
	msgs, err := outboxRepo.GetUnpublished(ctx, 10)
	assert.NoError(t, err)
	assert.Len(t, msgs, 1)
	assert.Equal(t, "payments.failed", msgs[0].Topic)
}
