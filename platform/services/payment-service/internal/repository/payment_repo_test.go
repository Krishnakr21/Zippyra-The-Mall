package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pashagolub/pgxmock/v3"
	"github.com/stretchr/testify/assert"
	"github.com/zippyra/platform/services/payment-service/internal/model"
)

func TestPaymentRepository(t *testing.T) {
	ctx := context.Background()

	t.Run("Create", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		repo := NewPaymentRepository(mock)
		
		pID := uuid.New()
		uID := uuid.New()
		oID := uuid.New()
		sID := uuid.New()

		mock.ExpectExec("(?s).*INSERT INTO payments.*").WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).WillReturnResult(pgxmock.NewResult("INSERT", 1))
		p := &model.Payment{ID: pID, OrderID: oID, UserID: uID, StoreID: sID}
		err := repo.Create(ctx, mock, p)
		assert.NoError(t, err)
	})

	t.Run("GetByID", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		repo := NewPaymentRepository(mock)

		pID := uuid.New()
		rows := pgxmock.NewRows([]string{"id", "order_id", "user_id", "store_id", "amount", "currency", "status", "payment_method", "gateway", "gateway_order_id", "gateway_payment_id", "upi_transaction_id", "idempotency_key", "failure_reason", "webhook_received_at", "created_at", "updated_at"}).
			AddRow(pID, uuid.New(), uuid.New(), uuid.New(), float64(100.0), "INR", model.PaymentStatusPending, nil, "RAZORPAY", nil, nil, nil, "k", nil, nil, time.Now(), time.Now())
		mock.ExpectQuery("(?s).*FROM payments WHERE id = \\$1").WithArgs(pID.String()).WillReturnRows(rows)
		
		res, err := repo.GetByID(ctx, pID.String())
		assert.NoError(t, err)
		assert.NotNil(t, res)
	})

	t.Run("GetByUserID", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		repo := NewPaymentRepository(mock)

		uID := uuid.New()
		rows := pgxmock.NewRows([]string{"id", "order_id", "user_id", "store_id", "amount", "currency", "status", "payment_method", "gateway", "gateway_order_id", "gateway_payment_id", "upi_transaction_id", "idempotency_key", "failure_reason", "webhook_received_at", "created_at", "updated_at"}).
			AddRow(uuid.New(), uuid.New(), uID, uuid.New(), float64(100.0), "INR", "SUCCESS", nil, "RAZORPAY", nil, nil, nil, "k", nil, nil, time.Now(), time.Now())
		mock.ExpectQuery("(?s).*FROM payments WHERE user_id = \\$1").WithArgs(uID.String(), 10, 0).WillReturnRows(rows)
		
		res, err := repo.GetByUserID(ctx, uID.String(), 10, 0)
		assert.NoError(t, err)
		assert.Len(t, res, 1)
	})
}
