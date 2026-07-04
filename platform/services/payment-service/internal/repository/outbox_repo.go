package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/zippyra/platform/services/payment-service/internal/model"
)

type OutboxRepository struct {
	db DB
}

func NewOutboxRepository(db DB) *OutboxRepository {
	return &OutboxRepository{db: db}
}

func (r *OutboxRepository) Create(ctx context.Context, tx pgx.Tx, topic string, payload []byte) error {
	query := `INSERT INTO payment_outbox (id, topic, payload, created_at)
              VALUES (gen_random_uuid(), $1, $2, NOW())`
	_, err := tx.Exec(ctx, query, topic, payload)
	return err
}

func (r *OutboxRepository) GetUnpublished(ctx context.Context, limit int) ([]model.OutboxMessage, error) {
	query := `SELECT id, topic, payload, created_at, published_at
              FROM payment_outbox
              WHERE published_at IS NULL
              ORDER BY created_at ASC
              LIMIT $1`
	
	rows, err := r.db.Query(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var msgs []model.OutboxMessage
	for rows.Next() {
		var msg model.OutboxMessage
		err := rows.Scan(&msg.ID, &msg.Topic, &msg.Payload, &msg.CreatedAt, &msg.PublishedAt)
		if err != nil {
			return nil, err
		}
		msgs = append(msgs, msg)
	}
	return msgs, nil
}

func (r *OutboxRepository) MarkPublished(ctx context.Context, id string) error {
	query := `UPDATE payment_outbox SET published_at = NOW() WHERE id = $1`
	_, err := r.db.Exec(ctx, query, id)
	return err
}
