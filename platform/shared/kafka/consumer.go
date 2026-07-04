package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/zippyra/platform/shared/logger"
)

// Consumer provides a wrapper over kafka.Reader with automatic DLQ routing.
type Consumer struct {
	reader *kafka.Reader
	dlq    *Producer
}

// NewConsumer initializes a consumer group reader with an attached DLQ producer.
func NewConsumer(brokers []string, topic, groupID string) *Consumer {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers: brokers,
		GroupID: groupID,
		Topic:   topic,
	})

	return &Consumer{
		reader: reader,
		dlq:    NewProducer(brokers, topic+".dlq"),
	}
}

// Consume processes messages using the provided handler. Includes 3 retries and DLQ routing.
func (c *Consumer) Consume(ctx context.Context, handler func(msg kafka.Message) error) error {
	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			return fmt.Errorf("context: fetch message failed: %w", err)
		}

		var processErr error
		for i := 0; i < 3; i++ {
			processErr = handler(msg)
			if processErr == nil {
				break
			}
			select {
			case <-ctx.Done():
				return fmt.Errorf("context: context cancelled during retry: %w", ctx.Err())
			case <-time.After(100 * time.Millisecond):
			}
		}

		if processErr != nil {
			logger.Ctx(ctx).Error().Err(processErr).
				Str("topic", msg.Topic).
				Int("partition", msg.Partition).
				Int64("offset", msg.Offset).
				Msg("failed to process message after 3 retries, routing to DLQ")

			// Write raw message to DLQ using json.RawMessage
			dlqErr := c.dlq.Publish(ctx, string(msg.Key), json.RawMessage(msg.Value))
			if dlqErr != nil {
				logger.Ctx(ctx).Error().Err(dlqErr).Msg("failed to write to DLQ")
			}
		}

		if err := c.reader.CommitMessages(ctx, msg); err != nil {
			logger.Ctx(ctx).Error().Err(err).Msg("failed to commit message")
		}
	}
}

// Close gracefully shuts down the consumer and its DLQ writer.
func (c *Consumer) Close() error {
	err := c.reader.Close()
	dlqErr := c.dlq.Close()
	if err != nil {
		return err
	}
	return dlqErr
}
