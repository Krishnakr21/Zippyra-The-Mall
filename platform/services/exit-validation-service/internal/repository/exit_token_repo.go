package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ExitTokenRepository struct {
	pool *pgxpool.Pool
}

func NewExitTokenRepository(pool *pgxpool.Pool) *ExitTokenRepository {
	return &ExitTokenRepository{
		pool: pool,
	}
}

// UpdateTokenUsed updates the token's used status atomically and returns the token record ID.
// If no rows are updated (already used or non-existent), it returns pgx.ErrNoRows.
func (r *ExitTokenRepository) UpdateTokenUsed(ctx context.Context, tokenHash string) (uuid.UUID, error) {
	query := `
		UPDATE exit_tokens
		SET is_used = true, used_at = NOW()
		WHERE token_hash = $1 AND is_used = false
		RETURNING id
	`
	var id uuid.UUID
	err := r.pool.QueryRow(ctx, query, tokenHash).Scan(&id)
	if err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

// StoreHasRFID checks if there is any device of type 'RFID_PAD' registered for the store.
func (r *ExitTokenRepository) StoreHasRFID(ctx context.Context, storeIDStr string) (bool, error) {
	storeID, err := uuid.Parse(storeIDStr)
	if err != nil {
		return false, err
	}

	query := `
		SELECT EXISTS (
			SELECT 1 FROM devices
			WHERE store_id = $1 AND device_type = 'RFID_PAD'
		)
	`
	var hasRFID bool
	err = r.pool.QueryRow(ctx, query, storeID).Scan(&hasRFID)
	if err != nil {
		return false, err
	}
	return hasRFID, nil
}

// GetTokenStatusByOrderID retrieves the latest exit token status for the given order.
func (r *ExitTokenRepository) GetTokenStatusByOrderID(ctx context.Context, orderIDStr string) (uuid.UUID, bool, time.Time, error) {
	orderID, err := uuid.Parse(orderIDStr)
	if err != nil {
		return uuid.Nil, false, time.Time{}, err
	}

	query := `
		SELECT order_id, is_used, expires_at
		FROM exit_tokens
		WHERE order_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`
	var retOrderID uuid.UUID
	var isUsed bool
	var expiresAt time.Time

	err = r.pool.QueryRow(ctx, query, orderID).Scan(&retOrderID, &isUsed, &expiresAt)
	if err != nil {
		return uuid.Nil, false, time.Time{}, err
	}

	return retOrderID, isUsed, expiresAt, nil
}
