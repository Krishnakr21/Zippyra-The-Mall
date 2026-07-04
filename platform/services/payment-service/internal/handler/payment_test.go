package handler

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/pashagolub/pgxmock/v3"
	"github.com/stretchr/testify/assert"
	"github.com/zippyra/platform/services/payment-service/internal/model"
	"github.com/zippyra/platform/services/payment-service/internal/repository"
	"github.com/zippyra/platform/services/payment-service/internal/service"
	"github.com/zippyra/platform/shared/jwt"
	"github.com/zippyra/platform/shared/middleware"
)

func TestPaymentHandler(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()

	payRepo := repository.NewPaymentRepository(mock)
	outboxRepo := repository.NewOutboxRepository(mock)
	router := service.NewGatewayRouter(nil, nil)
	svc := service.NewPaymentService(mock, payRepo, outboxRepo, router)
	h := NewPaymentHandler(svc)

	uID := uuid.New()
	claims := &jwt.ZippyraClaims{
		UserID: uID.String(),
	}

	t.Run("Initiate_Success", func(t *testing.T) {
		reqBody := `{"order_id":"` + uuid.New().String() + `","amount":100,"currency":"INR","gateway":"RAZORPAY"}`
		req := httptest.NewRequest("POST", "/", strings.NewReader(reqBody))
		ctx := middleware.ContextWithClaims(req.Context(), claims)
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		mock.ExpectBegin()
		mock.ExpectExec("(?s).*INSERT INTO payments.*").WillReturnResult(pgxmock.NewResult("INSERT", 1))
		mock.ExpectCommit()

		h.Initiate(rr, req)
		assert.Equal(t, http.StatusCreated, rr.Code)
	})

	t.Run("Initiate_ServiceError", func(t *testing.T) {
		reqBody := `{"order_id":"` + uuid.New().String() + `","amount":100,"currency":"INR","gateway":"RAZORPAY"}`
		req := httptest.NewRequest("POST", "/", strings.NewReader(reqBody))
		req = req.WithContext(middleware.ContextWithClaims(req.Context(), claims))
		rr := httptest.NewRecorder()

		mock.ExpectBegin().WillReturnError(fmt.Errorf("db error"))
		h.Initiate(rr, req)
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})

	t.Run("Status_Error", func(t *testing.T) {
		pID := uuid.New()
		req := httptest.NewRequest("GET", "/"+pID.String(), nil)
		ctx := middleware.ContextWithClaims(req.Context(), claims)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", pID.String())
		ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		mock.ExpectQuery("(?s).*SELECT (.+) FROM payments.*").WithArgs(pID.String()).WillReturnError(fmt.Errorf("db error"))
		h.Status(rr, req)
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})

	t.Run("History_Error", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/history", nil)
		ctx := middleware.ContextWithClaims(req.Context(), claims)
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		mock.ExpectQuery("(?s).*SELECT (.+) FROM payments.*").WithArgs(pgxmock.AnyArg()).WillReturnError(fmt.Errorf("db error"))
		h.History(rr, req)
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})

	t.Run("Refund_ServiceError", func(t *testing.T) {
		pID := uuid.New()
		req := httptest.NewRequest("POST", "/"+pID.String()+"/refund", strings.NewReader(`{"amount":10}`))
		ctx := middleware.ContextWithClaims(req.Context(), claims)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", pID.String())
		ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		rows := pgxmock.NewRows([]string{"id", "order_id", "user_id", "store_id", "amount", "currency", "status", "payment_method", "gateway", "gateway_order_id", "gateway_payment_id", "upi_transaction_id", "idempotency_key", "failure_reason", "webhook_received_at", "created_at", "updated_at"}).
			AddRow(pID, uuid.New(), uID, uuid.New(), float64(100.0), "INR", model.PaymentStatusSuccess, nil, "RAZORPAY", "go_1", "gp_1", nil, "k", nil, nil, time.Now(), time.Now())
		mock.ExpectQuery("(?s).*SELECT (.+) FROM payments.*").WithArgs(pID.String()).WillReturnRows(rows)
		
		mock.ExpectBegin().WillReturnError(fmt.Errorf("refund error"))
		h.Refund(rr, req)
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}
