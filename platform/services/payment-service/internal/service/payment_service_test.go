package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v3"
	"github.com/stretchr/testify/assert"
	"github.com/zippyra/platform/services/payment-service/internal/model"
	"github.com/zippyra/platform/services/payment-service/internal/repository"
)

type MockGateway struct {
	CreateOrderFn func(ctx context.Context, p *model.Payment) (*model.GatewayOrderResponse, error)
	RefundFn      func(ctx context.Context, p *model.Payment, amount int64) (*model.GatewayRefundResponse, error)
}

func (m *MockGateway) CreateOrder(ctx context.Context, p *model.Payment) (*model.GatewayOrderResponse, error) {
	return m.CreateOrderFn(ctx, p)
}

func (m *MockGateway) Refund(ctx context.Context, p *model.Payment, amount int64) (*model.GatewayRefundResponse, error) {
	return m.RefundFn(ctx, p, amount)
}

func setup(t *testing.T) (pgxmock.PgxPoolIface, *PaymentService, *MockGateway) {
	mock, _ := pgxmock.NewPool()
	payRepo := repository.NewPaymentRepository(mock)
	outboxRepo := repository.NewOutboxRepository(mock)
	mg := &MockGateway{
		CreateOrderFn: func(ctx context.Context, p *model.Payment) (*model.GatewayOrderResponse, error) {
			return &model.GatewayOrderResponse{GatewayOrderID: "go_1"}, nil
		},
		RefundFn: func(ctx context.Context, p *model.Payment, amount int64) (*model.GatewayRefundResponse, error) {
			return &model.GatewayRefundResponse{GatewayRefundID: "gr_1"}, nil
		},
	}
	router := NewGatewayRouter(mg, mg)
	svc := NewPaymentService(mock, payRepo, outboxRepo, router)
	return mock, svc, mg
}

func TestPaymentService_Full(t *testing.T) {
	ctx := context.Background()

	t.Run("InitiatePayment_Success", func(t *testing.T) {
		mock, svc, _ := setup(t)
		defer mock.Close()

		pID := uuid.New()
		p := &model.Payment{ID: pID, IdempotencyKey: "k1"}
		mock.ExpectQuery("SELECT (.+) FROM payments WHERE idempotency_key").WithArgs("k1").WillReturnError(pgx.ErrNoRows)
		mock.ExpectBegin()
		mock.ExpectExec("(?s).*INSERT INTO payments.*").WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).WillReturnResult(pgxmock.NewResult("INSERT", 1))
		mock.ExpectExec("(?s).*INSERT INTO payment_outbox.*").WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).WillReturnResult(pgxmock.NewResult("INSERT", 1))
		mock.ExpectCommit()
		
		res, err := svc.InitiatePayment(ctx, p)
		assert.NoError(t, err)
		assert.NotNil(t, res)
	})

	t.Run("HandlePaymentCaptured_Success", func(t *testing.T) {
		mock, svc, _ := setup(t)
		defer mock.Close()

		pID := uuid.New()
		event := model.RazorpayWebhookEvent{}
		event.Payload.Payment.Entity.ID = "gp_1"
		event.Payload.Payment.Entity.Notes.PaymentID = pID.String()

		rows := pgxmock.NewRows([]string{"id", "order_id", "user_id", "store_id", "amount", "currency", "status", "payment_method", "gateway", "gateway_order_id", "gateway_payment_id", "upi_transaction_id", "idempotency_key", "failure_reason", "webhook_received_at", "created_at", "updated_at"}).
			AddRow(pID.String(), uuid.New().String(), uuid.New().String(), uuid.New().String(), float64(100.0), "INR", model.PaymentStatusPending, nil, "RAZORPAY", nil, nil, nil, "k", nil, nil, time.Now(), time.Now())
		mock.ExpectQuery("SELECT (.+) FROM payments WHERE id").WithArgs(pID.String()).WillReturnRows(rows)
		
		mock.ExpectBegin()
		mock.ExpectExec("(?s).*UPDATE payments.*").WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).WillReturnResult(pgxmock.NewResult("UPDATE", 1))
		mock.ExpectExec("(?s).*INSERT INTO payment_outbox.*").WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).WillReturnResult(pgxmock.NewResult("INSERT", 1))
		mock.ExpectCommit()

		err := svc.HandlePaymentCaptured(ctx, event)
		assert.NoError(t, err)
	})
}
