package service

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/zippyra/platform/services/payment-service/internal/repository"
)

type OutboxProducer interface {
	Publish(ctx context.Context, topic, key string, value interface{}) error
}

type OutboxRelay struct {
	db       repository.DB
	repo     *repository.OutboxRepository
	producer OutboxProducer
	interval time.Duration
	backoff  time.Duration
	maxBack  time.Duration
}

func NewOutboxRelay(db repository.DB, producer OutboxProducer) *OutboxRelay {
	return &OutboxRelay{
		db:       db,
		repo:     repository.NewOutboxRepository(db),
		producer: producer,
		interval: 1 * time.Second,
		backoff:  1 * time.Second,
		maxBack:  30 * time.Second,
	}
}

func (r *OutboxRelay) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(r.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				// Drain remaining outbox before exit
				drainCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				r.poll(drainCtx)
				return
			case <-ticker.C:
				if err := r.poll(ctx); err != nil {
					log.Error().Err(err).Msg("outbox poll failed")
					// Exponential backoff on failure
					time.Sleep(r.backoff)
					r.backoff = minDuration(r.backoff*2, r.maxBack)
				} else {
					r.backoff = r.interval // Reset on success
				}
			}
		}
	}()
}

func (r *OutboxRelay) poll(ctx context.Context) error {
	msgs, err := r.repo.GetUnpublished(ctx, 100)
	if err != nil {
		return fmt.Errorf("get unpublished: %w", err)
	}
	for _, msg := range msgs {
		if err := r.producer.Publish(ctx, msg.Topic, msg.ID, msg.Payload); err != nil {
			log.Error().Err(err).Str("outbox_id", msg.ID).Msg("kafka publish failed")
			continue // Skip, retry next poll
		}
		if err := r.repo.MarkPublished(ctx, msg.ID); err != nil {
			log.Error().Err(err).Str("outbox_id", msg.ID).Msg("mark published failed")
		}
	}
	return nil
}

func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}
