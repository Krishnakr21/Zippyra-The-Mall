package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/zippyra/platform/services/payment-service/internal/model"
)

type PaymentRepository struct {
	db DB
}

func NewPaymentRepository(db DB) *PaymentRepository {
	return &PaymentRepository{db: db}
}

func (r *PaymentRepository) Create(ctx context.Context, tx pgx.Tx, p *model.Payment) error {
	query := `
        INSERT INTO payments (
            id, order_id, user_id, store_id, idempotency_key,
            gateway, gateway_order_id, amount, currency, status,
            payment_method, created_at, updated_at
        ) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,NOW(),NOW())`

	amount := float64(p.AmountPaise) / 100.0

	_, err := tx.Exec(ctx, query,
		p.ID, p.OrderID, p.UserID, p.StoreID, p.IdempotencyKey,
		p.Gateway, p.GatewayOrderID, amount, p.Currency, p.Status,
		p.PaymentMethod)
	return err
}

func (r *PaymentRepository) GetByIdempotencyKey(ctx context.Context, key string) (*model.Payment, error) {
	query := `
        SELECT id, order_id, user_id, store_id, amount, currency, status, payment_method,
               gateway, gateway_order_id, gateway_payment_id, upi_transaction_id,
               idempotency_key, failure_reason, refund_id, refund_amount, webhook_received_at, created_at, updated_at
        FROM payments WHERE idempotency_key = $1`

	var p model.Payment
	var amount float64
	var refundAmount *float64

	err := r.db.QueryRow(ctx, query, key).Scan(
		&p.ID, &p.OrderID, &p.UserID, &p.StoreID, &amount, &p.Currency, &p.Status, &p.PaymentMethod,
		&p.Gateway, &p.GatewayOrderID, &p.GatewayPaymentID, &p.UPITransactionID,
		&p.IdempotencyKey, &p.FailureReason, &p.RefundID, &refundAmount, &p.WebhookReceivedAt, &p.CreatedAt, &p.UpdatedAt)

	if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	p.AmountPaise = int64(amount * 100)
	if refundAmount != nil {
		p.RefundAmountPaise = int64(*refundAmount * 100)
	}
	return &p, nil
}

func (r *PaymentRepository) GetByID(ctx context.Context, id string) (*model.Payment, error) {
	query := `
        SELECT id, order_id, user_id, store_id, amount, currency, status, payment_method,
               gateway, gateway_order_id, gateway_payment_id, upi_transaction_id,
               idempotency_key, failure_reason, refund_id, refund_amount, webhook_received_at, created_at, updated_at
        FROM payments WHERE id = $1`

	var p model.Payment
	var amount float64
	var refundAmount *float64

	err := r.db.QueryRow(ctx, query, id).Scan(
		&p.ID, &p.OrderID, &p.UserID, &p.StoreID, &amount, &p.Currency, &p.Status, &p.PaymentMethod,
		&p.Gateway, &p.GatewayOrderID, &p.GatewayPaymentID, &p.UPITransactionID,
		&p.IdempotencyKey, &p.FailureReason, &p.RefundID, &refundAmount, &p.WebhookReceivedAt, &p.CreatedAt, &p.UpdatedAt)

	if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	p.AmountPaise = int64(amount * 100)
	if refundAmount != nil {
		p.RefundAmountPaise = int64(*refundAmount * 100)
	}
	return &p, nil
}

func (r *PaymentRepository) Update(ctx context.Context, tx pgx.Tx, p *model.Payment) error {
	query := `UPDATE payments SET 
                status = $2, 
                gateway_payment_id = $3, 
                failure_reason = $4, 
                refund_id = $5,
                refund_amount = $6,
                updated_at = NOW() 
              WHERE id = $1`

	var refundAmt *float64
	if p.RefundAmountPaise > 0 {
		amt := float64(p.RefundAmountPaise) / 100.0
		refundAmt = &amt
	}

	var err error
	if tx != nil {
		_, err = tx.Exec(ctx, query, p.ID, p.Status, p.GatewayPaymentID, p.FailureReason, p.RefundID, refundAmt)
	} else {
		_, err = r.db.Exec(ctx, query, p.ID, p.Status, p.GatewayPaymentID, p.FailureReason, p.RefundID, refundAmt)
	}
	return err
}

func (r *PaymentRepository) UpdateStatus(ctx context.Context, id, status, failureReason string) error {
	query := `UPDATE payments SET status = $2, failure_reason = $3, updated_at = NOW() WHERE id = $1`
	var fr *string
	if failureReason != "" {
		fr = &failureReason
	}
	_, err := r.db.Exec(ctx, query, id, status, fr)
	return err
}

func (r *PaymentRepository) UpdateGatewayPaymentID(ctx context.Context, id, gatewayPaymentID string) error {
	query := `UPDATE payments SET gateway_payment_id = $2, updated_at = NOW() WHERE id = $1`
	_, err := r.db.Exec(ctx, query, id, gatewayPaymentID)
	return err
}

func (r *PaymentRepository) GetByUserID(ctx context.Context, userID string, limit, offset int) ([]model.Payment, error) {
	query := `
        SELECT id, order_id, user_id, store_id, amount, currency, status, payment_method,
               gateway, gateway_order_id, gateway_payment_id, upi_transaction_id,
               idempotency_key, failure_reason, refund_id, refund_amount, webhook_received_at, created_at, updated_at
        FROM payments 
        WHERE user_id = $1
        ORDER BY created_at DESC
        LIMIT $2 OFFSET $3`

	rows, err := r.db.Query(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var payments []model.Payment
	for rows.Next() {
		var p model.Payment
		var amount float64
		var refundAmount *float64
		err := rows.Scan(
			&p.ID, &p.OrderID, &p.UserID, &p.StoreID, &amount, &p.Currency, &p.Status, &p.PaymentMethod,
			&p.Gateway, &p.GatewayOrderID, &p.GatewayPaymentID, &p.UPITransactionID,
			&p.IdempotencyKey, &p.FailureReason, &p.RefundID, &refundAmount, &p.WebhookReceivedAt, &p.CreatedAt, &p.UpdatedAt)
		if err != nil {
			return nil, err
		}
		p.AmountPaise = int64(amount * 100)
		if refundAmount != nil {
			p.RefundAmountPaise = int64(*refundAmount * 100)
		}
		payments = append(payments, p)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return payments, nil
}
