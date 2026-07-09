package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/zippyra/platform/services/order-service/internal/model"
)

type OrderRepository struct {
	pool *pgxpool.Pool
}

func NewOrderRepository(pool *pgxpool.Pool) *OrderRepository {
	return &OrderRepository{pool: pool}
}

func (r *OrderRepository) UpsertOrder(ctx context.Context, tx pgx.Tx, o *model.Order) (*model.Order, bool, error) {
	query := `
		INSERT INTO orders (
			id, order_number, user_id, store_id, session_id,
			status, supply_type, subtotal, gst_total, cgst_total,
			sgst_total, igst_total, discount_total, total_amount,
			payment_method, payment_id, exit_token, exit_token_expires_at,
			invoice_url, return_window_ends_at, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, NOW(), NOW())
		ON CONFLICT (payment_id) DO NOTHING
		RETURNING id, order_number, user_id, store_id, session_id, status, supply_type, subtotal, gst_total, cgst_total, sgst_total, igst_total, discount_total, total_amount, payment_method, payment_id, exit_token, exit_token_expires_at, invoice_url, return_window_ends_at, created_at, updated_at`

	var created model.Order
	err := tx.QueryRow(ctx, query,
		o.ID, o.OrderNumber, o.UserID, o.StoreID, o.SessionID,
		o.Status, o.SupplyType, o.Subtotal, o.GSTTotal, o.CGSTTotal,
		o.SGSTTotal, o.IGSTTotal, o.DiscountTotal, o.TotalAmount,
		o.PaymentMethod, o.PaymentID, o.ExitToken, o.ExitTokenExpiresAt,
		o.InvoiceURL, o.ReturnWindowEndsAt).Scan(
			&created.ID, &created.OrderNumber, &created.UserID, &created.StoreID, &created.SessionID,
			&created.Status, &created.SupplyType, &created.Subtotal, &created.GSTTotal, &created.CGSTTotal,
			&created.SGSTTotal, &created.IGSTTotal, &created.DiscountTotal, &created.TotalAmount,
			&created.PaymentMethod, &created.PaymentID, &created.ExitToken, &created.ExitTokenExpiresAt,
			&created.InvoiceURL, &created.ReturnWindowEndsAt, &created.CreatedAt, &created.UpdatedAt)

	if err == pgx.ErrNoRows {
		// Conflicted - order already exists
		if o.PaymentID == nil {
			return nil, false, errors.New("conflict occurred but payment ID is nil")
		}
		existing, err := r.GetByPaymentID(ctx, tx, *o.PaymentID)
		if err != nil {
			return nil, false, err
		}
		return existing, false, nil
	}
	if err != nil {
		return nil, false, err
	}

	return &created, true, nil
}

func (r *OrderRepository) GetByPaymentID(ctx context.Context, db DB, paymentID uuid.UUID) (*model.Order, error) {
	query := `
		SELECT id, order_number, user_id, store_id, session_id, status, supply_type,
		       subtotal, gst_total, cgst_total, sgst_total, igst_total, discount_total,
		       total_amount, payment_method, payment_id, exit_token, exit_token_expires_at,
		       invoice_url, return_window_ends_at, created_at, updated_at
		FROM orders WHERE payment_id = $1`

	var o model.Order
	err := db.QueryRow(ctx, query, paymentID).Scan(
		&o.ID, &o.OrderNumber, &o.UserID, &o.StoreID, &o.SessionID, &o.Status, &o.SupplyType,
		&o.Subtotal, &o.GSTTotal, &o.CGSTTotal, &o.SGSTTotal, &o.IGSTTotal, &o.DiscountTotal,
		&o.TotalAmount, &o.PaymentMethod, &o.PaymentID, &o.ExitToken, &o.ExitTokenExpiresAt,
		&o.InvoiceURL, &o.ReturnWindowEndsAt, &o.CreatedAt, &o.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &o, nil
}

func (r *OrderRepository) GetByID(ctx context.Context, id string) (*model.Order, error) {
	query := `
		SELECT id, order_number, user_id, store_id, session_id, status, supply_type,
		       subtotal, gst_total, cgst_total, sgst_total, igst_total, discount_total,
		       total_amount, payment_method, payment_id, exit_token, exit_token_expires_at,
		       invoice_url, return_window_ends_at, created_at, updated_at
		FROM orders WHERE id = $1`

	var o model.Order
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&o.ID, &o.OrderNumber, &o.UserID, &o.StoreID, &o.SessionID, &o.Status, &o.SupplyType,
		&o.Subtotal, &o.GSTTotal, &o.CGSTTotal, &o.SGSTTotal, &o.IGSTTotal, &o.DiscountTotal,
		&o.TotalAmount, &o.PaymentMethod, &o.PaymentID, &o.ExitToken, &o.ExitTokenExpiresAt,
		&o.InvoiceURL, &o.ReturnWindowEndsAt, &o.CreatedAt, &o.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &o, nil
}

func (r *OrderRepository) UpdateExitToken(ctx context.Context, orderID uuid.UUID, token string, expiresAt time.Time) error {
	query := `
		UPDATE orders
		SET exit_token = $1, exit_token_expires_at = $2, updated_at = NOW()
		WHERE id = $3`
	_, err := r.pool.Exec(ctx, query, token, expiresAt, orderID)
	return err
}

func (r *OrderRepository) UpdateInvoiceURL(ctx context.Context, orderID uuid.UUID, invoiceURL string) error {
	query := `
		UPDATE orders
		SET invoice_url = $1, updated_at = NOW()
		WHERE id = $2`
	_, err := r.pool.Exec(ctx, query, invoiceURL, orderID)
	return err
}

func (r *OrderRepository) GetHistory(ctx context.Context, userID string, storeID *uuid.UUID, limit, offset int) ([]model.Order, error) {
	var rows pgx.Rows
	var err error

	if storeID != nil {
		query := `
			SELECT id, order_number, user_id, store_id, session_id, status, supply_type,
			       subtotal, gst_total, cgst_total, sgst_total, igst_total, discount_total,
			       total_amount, payment_method, payment_id, exit_token, exit_token_expires_at,
			       invoice_url, return_window_ends_at, created_at, updated_at
			FROM orders
			WHERE user_id = $1 AND store_id = $2
			ORDER BY created_at DESC
			LIMIT $3 OFFSET $4`
		rows, err = r.pool.Query(ctx, query, userID, *storeID, limit, offset)
	} else {
		query := `
			SELECT id, order_number, user_id, store_id, session_id, status, supply_type,
			       subtotal, gst_total, cgst_total, sgst_total, igst_total, discount_total,
			       total_amount, payment_method, payment_id, exit_token, exit_token_expires_at,
			       invoice_url, return_window_ends_at, created_at, updated_at
			FROM orders
			WHERE user_id = $1
			ORDER BY created_at DESC
			LIMIT $2 OFFSET $3`
		rows, err = r.pool.Query(ctx, query, userID, limit, offset)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []model.Order
	for rows.Next() {
		var o model.Order
		err = rows.Scan(
			&o.ID, &o.OrderNumber, &o.UserID, &o.StoreID, &o.SessionID, &o.Status, &o.SupplyType,
			&o.Subtotal, &o.GSTTotal, &o.CGSTTotal, &o.SGSTTotal, &o.IGSTTotal, &o.DiscountTotal,
			&o.TotalAmount, &o.PaymentMethod, &o.PaymentID, &o.ExitToken, &o.ExitTokenExpiresAt,
			&o.InvoiceURL, &o.ReturnWindowEndsAt, &o.CreatedAt, &o.UpdatedAt)
		if err != nil {
			return nil, err
		}
		orders = append(orders, o)
	}
	return orders, nil
}
