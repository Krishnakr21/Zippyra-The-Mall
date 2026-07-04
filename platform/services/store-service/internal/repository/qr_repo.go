package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/zippyra/platform/services/store-service/internal/model"
)

// QRTokenRepository handles QR token database operations.
type QRTokenRepository struct {
	db dbPool
}

// NewQRTokenRepository creates a new QRTokenRepository.
func NewQRTokenRepository(db dbPool) *QRTokenRepository {
	return &QRTokenRepository{db: db}
}

// GetActiveToken retrieves an active QR token by its token string.
func (r *QRTokenRepository) GetActiveToken(ctx context.Context, token string) (*model.StoreQRToken, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var t model.StoreQRToken
	err := r.db.QueryRow(ctx,
		`SELECT id, store_id, token, token_type, used_count, is_active, expires_at, created_at
		 FROM store_qr_tokens WHERE token = $1 AND is_active = true`, token,
	).Scan(&t.ID, &t.StoreID, &t.Token, &t.TokenType, &t.UsedCount, &t.IsActive, &t.ExpiresAt, &t.CreatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &t, nil
}

// IncrementUsedCount increments the used_count for a QR token.
func (r *QRTokenRepository) IncrementUsedCount(ctx context.Context, tokenID uuid.UUID) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	return r.db.QueryRow(ctx,
		`UPDATE store_qr_tokens SET used_count = used_count + 1 WHERE id = $1 RETURNING id`, tokenID,
	).Scan(new(uuid.UUID))
}
