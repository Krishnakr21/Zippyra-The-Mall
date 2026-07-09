package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/zippyra/platform/services/order-service/internal/model"
)

type ExitTokenRepository struct {
	pool *pgxpool.Pool
}

func NewExitTokenRepository(pool *pgxpool.Pool) *ExitTokenRepository {
	return &ExitTokenRepository{pool: pool}
}

func (r *ExitTokenRepository) Create(ctx context.Context, et *model.ExitToken) error {
	query := `
		INSERT INTO exit_tokens (
			id, order_id, user_id, store_id, token_hash,
			is_used, expires_at, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())`

	_, err := r.pool.Exec(ctx, query,
		et.ID, et.OrderID, et.UserID, et.StoreID, et.TokenHash,
		et.IsUsed, et.ExpiresAt)
	return err
}

func (r *ExitTokenRepository) GetActiveByOrderID(ctx context.Context, orderID string) (*model.ExitToken, error) {
	query := `
		SELECT id, order_id, user_id, store_id, token_hash, is_used, used_at, expires_at, created_at
		FROM exit_tokens
		WHERE order_id = $1 AND is_used = false AND expires_at > NOW()
		ORDER BY expires_at DESC LIMIT 1`

	var et model.ExitToken
	err := r.pool.QueryRow(ctx, query, orderID).Scan(
		&et.ID, &et.OrderID, &et.UserID, &et.StoreID, &et.TokenHash, &et.IsUsed, &et.UsedAt, &et.ExpiresAt, &et.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &et, nil
}
