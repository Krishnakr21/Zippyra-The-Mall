package repository

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/zippyra/platform/services/order-service/internal/model"
)

type ReturnRepository struct {
	pool *pgxpool.Pool
}

func NewReturnRepository(pool *pgxpool.Pool) *ReturnRepository {
	return &ReturnRepository{pool: pool}
}

func (r *ReturnRepository) CreateReturnRequest(ctx context.Context, req *model.ReturnRequest) error {
	query := `
		INSERT INTO return_requests (
			id, order_id, user_id, store_id, status, reason, items, refund_initiated, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW())`

	itemsJSON, err := json.Marshal(req.Items)
	if err != nil {
		return err
	}

	_, err = r.pool.Exec(ctx, query,
		req.ID, req.OrderID, req.UserID, req.StoreID, req.Status, req.Reason, itemsJSON, req.RefundInitiated)
	return err
}

func (r *ReturnRepository) GetReturnRequestByID(ctx context.Context, returnID string) (*model.ReturnRequest, error) {
	query := `
		SELECT id, order_id, user_id, store_id, status, reason, items, refund_initiated, created_at, updated_at
		FROM return_requests WHERE id = $1`

	var req model.ReturnRequest
	var itemsJSON []byte
	err := r.pool.QueryRow(ctx, query, returnID).Scan(
		&req.ID, &req.OrderID, &req.UserID, &req.StoreID, &req.Status, &req.Reason, &itemsJSON, &req.RefundInitiated, &req.CreatedAt, &req.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(itemsJSON, &req.Items); err != nil {
		return nil, err
	}
	return &req, nil
}

func (r *ReturnRepository) UpdateReturnStatus(ctx context.Context, returnID string, status string, refundInitiated bool) error {
	query := `
		UPDATE return_requests
		SET status = $1, refund_initiated = $2, updated_at = NOW()
		WHERE id = $3`
	_, err := r.pool.Exec(ctx, query, status, refundInitiated, returnID)
	return err
}

func (r *ReturnRepository) HasPendingReturn(ctx context.Context, orderID uuid.UUID) (bool, error) {
	query := `
		SELECT EXISTS (
			SELECT 1 FROM return_requests WHERE order_id = $1 AND status = 'PENDING_STAFF_APPROVAL'
		)`
	var exists bool
	err := r.pool.QueryRow(ctx, query, orderID).Scan(&exists)
	return exists, err
}
