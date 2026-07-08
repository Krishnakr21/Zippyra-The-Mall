package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/zippyra/platform/services/payment-service/internal/model"
)

type WebhookRepository struct {
	db DB
}

func NewWebhookRepository(db DB) *WebhookRepository {
	return &WebhookRepository{db: db}
}

// InsertIdempotent returns (true, nil) if inserted, (false, nil) if already exists
func (r *WebhookRepository) InsertIdempotent(ctx context.Context, w *model.WebhookEvent) (bool, error) {
	query := `
        INSERT INTO payment_webhook_events
            (id, gateway, event_id, event_type, payload, hmac_verified, processed, created_at)
        VALUES ($1,$2,$3,$4,$5,$6,false,NOW())
        ON CONFLICT (event_id) DO NOTHING`
	result, err := r.db.Exec(ctx, query,
		uuid.New(), w.Gateway, w.EventID, w.EventType, w.Payload, w.HMACVerified)
	if err != nil {
		return false, err
	}
	return result.RowsAffected() == 1, nil
}

func (r *WebhookRepository) MarkProcessed(ctx context.Context, eventID string, errMsg string) error {
	query := `UPDATE payment_webhook_events SET processed = true, error_message = $2, processed_at = NOW() WHERE event_id = $1`
	var msg *string
	if errMsg != "" {
		msg = &errMsg
	}
	_, err := r.db.Exec(ctx, query, eventID, msg)
	return err
}
