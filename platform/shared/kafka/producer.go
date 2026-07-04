package kafka

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/segmentio/kafka-go"
	"github.com/zippyra/platform/shared/logger"
)

// Producer provides a simple wrapper over kafka-go with DLQ support.
type Producer struct {
	writer    *kafka.Writer
	dlqWriter *kafka.Writer
	topic     string
}

// NewProducer creates a new Producer with required acks=all and DLQ setup.
func NewProducer(brokers []string, topic string) *Producer {
	writer := &kafka.Writer{
		Addr:                   kafka.TCP(brokers...),
		Topic:                  topic,
		RequiredAcks:           kafka.RequireAll,
		MaxAttempts:            3,
		AllowAutoTopicCreation: true,
	}

	dlqWriter := &kafka.Writer{
		Addr:                   kafka.TCP(brokers...),
		Topic:                  topic + ".dlq",
		RequiredAcks:           kafka.RequireAll,
		MaxAttempts:            3,
		AllowAutoTopicCreation: true,
	}

	return &Producer{
		writer:    writer,
		dlqWriter: dlqWriter,
		topic:     topic,
	}
}

// Publish serializes and writes a message, routing it to the DLQ if writing to the main topic fails.
// Store_id is typically passed as the partition key.
func (p *Producer) Publish(ctx context.Context, key string, value interface{}) error {
	b, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("context: failed to marshal value: %w", err)
	}

	msg := kafka.Message{
		Key:   []byte(key),
		Value: b,
	}

	err = p.writer.WriteMessages(ctx, msg)
	if err != nil {
		logger.Ctx(ctx).Warn().Err(err).Str("topic", p.topic).Msg("write failed, routing to DLQ")
		
		dlqErr := p.dlqWriter.WriteMessages(ctx, msg)
		if dlqErr != nil {
			return fmt.Errorf("context: failed to write to main topic and DLQ: dlq_error=%w, original_error=%v", dlqErr, err)
		}
		
		return fmt.Errorf("context: message routed to DLQ after main topic write failure: %w", err)
	}

	return nil
}

// Close gracefully shuts down the producers.
func (p *Producer) Close() error {
	dlqErr := p.dlqWriter.Close()
	err := p.writer.Close()
	if err != nil {
		return err
	}
	return dlqErr
}
