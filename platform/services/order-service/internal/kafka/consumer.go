package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/zippyra/platform/services/order-service/internal/model"
)

type OrderService interface {
	CreateOrderFromPayment(ctx context.Context, event *model.PaymentConfirmedEvent) error
}

type Consumer struct {
	reader       *kafka.Reader
	orderService OrderService
	producer     *Producer
}

func NewConsumer(brokers []string, groupID, topic string, orderService OrderService, producer *Producer) *Consumer {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:          brokers,
		GroupID:          groupID,
		Topic:            topic,
		CommitInterval:   0, // Disable auto commit for manual commit control
		StartOffset:      kafka.FirstOffset,
		RebalanceTimeout: 10 * time.Second,
	})

	return &Consumer{
		reader:       reader,
		orderService: orderService,
		producer:     producer,
	}
}

func (c *Consumer) Start(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			msg, err := c.reader.FetchMessage(ctx)
			if err != nil {
				// Context cancelled or closed reader
				if ctx.Err() != nil {
					return ctx.Err()
				}
				continue
			}

			err = c.HandlePaymentConfirmed(ctx, msg)
			if err != nil {
				// Log error and publish saga compensation
				var event model.PaymentConfirmedEvent
				if jsonErr := json.Unmarshal(msg.Value, &event); jsonErr == nil {
					comp := model.OrderCreationFailedEvent{
						EventType: "order.creation_failed",
						PaymentID: event.PaymentID.String(),
						UserID:    event.UserID.String(),
						Reason:    err.Error(),
						Timestamp: time.Now().Format(time.RFC3339),
					}
					_ = c.producer.Publish(ctx, "order.creation_failed", event.PaymentID.String(), comp)
				}
			}

			// Commit offset only after processing (or after handling failure and sending saga event)
			if commitErr := c.reader.CommitMessages(ctx, msg); commitErr != nil {
				// Log commit failure, but do not crash
				fmt.Printf("failed to commit message: %v\n", commitErr)
			}
		}
	}
}

func (c *Consumer) HandlePaymentConfirmed(ctx context.Context, msg kafka.Message) error {
	var event model.PaymentConfirmedEvent
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		return fmt.Errorf("unmarshal payment event: %w", err)
	}

	// Retry up to 3 times
	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		lastErr = c.orderService.CreateOrderFromPayment(ctx, &event)
		if lastErr == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}

	return fmt.Errorf("order creation failed after 3 attempts: %w", lastErr)
}

func (c *Consumer) Close() error {
	return c.reader.Close()
}
