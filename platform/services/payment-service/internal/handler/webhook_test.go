package handler

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pashagolub/pgxmock/v3"
	"github.com/stretchr/testify/assert"
	"github.com/zippyra/platform/services/payment-service/internal/model"
	"github.com/zippyra/platform/services/payment-service/internal/repository"
	"github.com/zippyra/platform/services/payment-service/internal/service"
)

func TestWebhookHandler(t *testing.T) {
	rzpSecret := "razor-secret"
	cfSecret := "cf-secret"

	t.Run("Razorpay_Success", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		payRepo := repository.NewPaymentRepository(mock)
		webhookRepo := repository.NewWebhookRepository(mock)
		outboxRepo := repository.NewOutboxRepository(mock)
		svc := service.NewPaymentService(mock, payRepo, outboxRepo, nil)
		h := NewWebhookHandler(svc, webhookRepo, rzpSecret, cfSecret)

		pID := uuid.New()
		event := model.RazorpayWebhookEvent{
			EventID: "evt_1",
			Event:   "payment.captured",
		}
		event.Payload.Payment.Entity.ID = "pay_1"
		event.Payload.Payment.Entity.Status = "captured"
		event.Payload.Payment.Entity.Notes.PaymentID = pID.String()
		b, _ := json.Marshal(event)
		
		mac := hmac.New(sha256.New, []byte(rzpSecret))
		mac.Write(b)
		sig := hex.EncodeToString(mac.Sum(nil))

		req := httptest.NewRequest("POST", "/razorpay", bytes.NewReader(b))
		req.Header.Set("X-Razorpay-Signature", sig)
		rr := httptest.NewRecorder()

		mock.ExpectExec("(?s).*INSERT INTO payment_webhook_events.*").WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).WillReturnResult(pgxmock.NewResult("INSERT", 1))
		mock.ExpectBegin()
		rows := pgxmock.NewRows([]string{"id", "order_id", "user_id", "store_id", "amount", "currency", "status", "payment_method", "gateway", "gateway_order_id", "gateway_payment_id", "upi_transaction_id", "idempotency_key", "failure_reason", "webhook_received_at", "created_at", "updated_at"}).
			AddRow(pID, uuid.New(), uuid.New(), uuid.New(), 10.0, "INR", model.PaymentStatusPending, nil, "RAZORPAY", nil, nil, nil, "k", nil, nil, time.Now(), time.Now())
		mock.ExpectQuery("(?s).*SELECT (.+) FROM payments.*").WithArgs(pgxmock.AnyArg()).WillReturnRows(rows)
		mock.ExpectExec("(?s).*UPDATE payments.*").WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).WillReturnResult(pgxmock.NewResult("UPDATE", 1))
		mock.ExpectExec("(?s).*INSERT INTO payment_outbox.*").WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).WillReturnResult(pgxmock.NewResult("INSERT", 1))
		mock.ExpectCommit()
		mock.ExpectExec("(?s).*UPDATE payment_webhook_events.*").WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).WillReturnResult(pgxmock.NewResult("UPDATE", 1))

		h.Razorpay(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("Cashfree_Success", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		webhookRepo := repository.NewWebhookRepository(mock)
		payRepo := repository.NewPaymentRepository(mock)
		outboxRepo := repository.NewOutboxRepository(mock)
		svc := service.NewPaymentService(mock, payRepo, outboxRepo, nil)
		h := NewWebhookHandler(svc, webhookRepo, rzpSecret, cfSecret)

		pID := uuid.New()
		event := model.CashfreeWebhookEvent{
			EventType: "PAYMENT_SUCCESS_WEBHOOK",
		}
		event.Data.Order.OrderID = pID.String()
		event.Data.Payment.PaymentStatus = "SUCCESS"
		event.Data.Payment.CfPaymentID = "123"
		b, _ := json.Marshal(event)
		
		ts := "1234567890"
		mac := hmac.New(sha256.New, []byte(cfSecret))
		mac.Write([]byte(ts + string(b)))
		sig := base64.StdEncoding.EncodeToString(mac.Sum(nil))

		req := httptest.NewRequest("POST", "/cashfree", bytes.NewReader(b))
		req.Header.Set("x-webhook-signature", sig)
		req.Header.Set("x-webhook-timestamp", ts)
		rr := httptest.NewRecorder()

		mock.ExpectExec("(?s).*INSERT INTO payment_webhook_events.*").WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).WillReturnResult(pgxmock.NewResult("INSERT", 1))
		mock.ExpectBegin()
		rows := pgxmock.NewRows([]string{"id", "order_id", "user_id", "store_id", "amount", "currency", "status", "payment_method", "gateway", "gateway_order_id", "gateway_payment_id", "upi_transaction_id", "idempotency_key", "failure_reason", "webhook_received_at", "created_at", "updated_at"}).
			AddRow(pID, uuid.New(), uuid.New(), uuid.New(), 10.0, "INR", model.PaymentStatusPending, nil, "CASHFREE", nil, nil, nil, "k", nil, nil, time.Now(), time.Now())
		mock.ExpectQuery("(?s).*SELECT (.+) FROM payments.*").WithArgs(pgxmock.AnyArg()).WillReturnRows(rows)
		mock.ExpectExec("(?s).*UPDATE payments.*").WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).WillReturnResult(pgxmock.NewResult("UPDATE", 1))
		mock.ExpectExec("(?s).*INSERT INTO payment_outbox.*").WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).WillReturnResult(pgxmock.NewResult("INSERT", 1))
		mock.ExpectCommit()
		mock.ExpectExec("(?s).*UPDATE payment_webhook_events.*").WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).WillReturnResult(pgxmock.NewResult("UPDATE", 1))

		h.Cashfree(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("Razorpay_Failed", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		payRepo := repository.NewPaymentRepository(mock)
		webhookRepo := repository.NewWebhookRepository(mock)
		outboxRepo := repository.NewOutboxRepository(mock)
		svc := service.NewPaymentService(mock, payRepo, outboxRepo, nil)
		h := NewWebhookHandler(svc, webhookRepo, rzpSecret, cfSecret)

		pID := uuid.New()
		event := model.RazorpayWebhookEvent{
			EventID: "evt_fail",
			Event:   "payment.failed",
		}
		event.Payload.Payment.Entity.Status = "failed"
		event.Payload.Payment.Entity.Notes.PaymentID = pID.String()
		event.Payload.Payment.Entity.ErrorDescription = "insufficient funds"
		b, _ := json.Marshal(event)
		
		mac := hmac.New(sha256.New, []byte(rzpSecret))
		mac.Write(b)
		sig := hex.EncodeToString(mac.Sum(nil))

		req := httptest.NewRequest("POST", "/razorpay", bytes.NewReader(b))
		req.Header.Set("X-Razorpay-Signature", sig)
		rr := httptest.NewRecorder()

		mock.ExpectExec("(?s).*INSERT INTO payment_webhook_events.*").WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).WillReturnResult(pgxmock.NewResult("INSERT", 1))
		mock.ExpectBegin()
		rows := pgxmock.NewRows([]string{"id", "order_id", "user_id", "store_id", "amount", "currency", "status", "payment_method", "gateway", "gateway_order_id", "gateway_payment_id", "upi_transaction_id", "idempotency_key", "failure_reason", "webhook_received_at", "created_at", "updated_at"}).
			AddRow(pID, uuid.New(), uuid.New(), uuid.New(), 10.0, "INR", model.PaymentStatusPending, nil, "RAZORPAY", nil, nil, nil, "k", nil, nil, time.Now(), time.Now())
		mock.ExpectQuery("(?s).*SELECT (.+) FROM payments.*").WithArgs(pgxmock.AnyArg()).WillReturnRows(rows)
		mock.ExpectExec("(?s).*UPDATE payments.*").WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).WillReturnResult(pgxmock.NewResult("UPDATE", 1))
		mock.ExpectExec("(?s).*INSERT INTO payment_outbox.*").WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).WillReturnResult(pgxmock.NewResult("INSERT", 1))
		mock.ExpectCommit()
		mock.ExpectExec("(?s).*UPDATE payment_webhook_events.*").WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).WillReturnResult(pgxmock.NewResult("UPDATE", 1))

		h.Razorpay(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("Razorpay_Authorized", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		payRepo := repository.NewPaymentRepository(mock)
		webhookRepo := repository.NewWebhookRepository(mock)
		svc := service.NewPaymentService(mock, payRepo, nil, nil)
		h := NewWebhookHandler(svc, webhookRepo, rzpSecret, cfSecret)

		pID := uuid.New()
		event := model.RazorpayWebhookEvent{
			EventID: "evt_auth",
			Event:   "payment.authorized",
		}
		event.Payload.Payment.Entity.Notes.PaymentID = pID.String()
		b, _ := json.Marshal(event)
		
		mac := hmac.New(sha256.New, []byte(rzpSecret))
		mac.Write(b)
		sig := hex.EncodeToString(mac.Sum(nil))

		req := httptest.NewRequest("POST", "/razorpay", bytes.NewReader(b))
		req.Header.Set("X-Razorpay-Signature", sig)
		rr := httptest.NewRecorder()

		mock.ExpectExec("(?s).*INSERT INTO payment_webhook_events.*").WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).WillReturnResult(pgxmock.NewResult("INSERT", 1))
		rows := pgxmock.NewRows([]string{"id", "order_id", "user_id", "store_id", "amount", "currency", "status", "payment_method", "gateway", "gateway_order_id", "gateway_payment_id", "upi_transaction_id", "idempotency_key", "failure_reason", "webhook_received_at", "created_at", "updated_at"}).
			AddRow(pID, uuid.New(), uuid.New(), uuid.New(), 10.0, "INR", model.PaymentStatusPending, nil, "RAZORPAY", nil, nil, nil, "k", nil, nil, time.Now(), time.Now())
		mock.ExpectQuery("(?s).*SELECT (.+) FROM payments.*").WithArgs(pgxmock.AnyArg()).WillReturnRows(rows)
		mock.ExpectExec("(?s).*UPDATE payments.*").WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).WillReturnResult(pgxmock.NewResult("UPDATE", 1))
		mock.ExpectExec("(?s).*UPDATE payment_webhook_events.*").WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).WillReturnResult(pgxmock.NewResult("UPDATE", 1))

		h.Razorpay(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}
