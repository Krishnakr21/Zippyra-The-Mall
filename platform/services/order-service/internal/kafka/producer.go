package kafka

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/segmentio/kafka-go"
)

type Producer struct {
	brokers   []string
	PublishFn func(ctx context.Context, topic, key string, value interface{}) error
}

func NewProducer(brokers []string) *Producer {
	return &Producer{brokers: brokers}
}

func (p *Producer) Publish(ctx context.Context, topic, key string, value interface{}) error {
	if p.PublishFn != nil {
		return p.PublishFn(ctx, topic, key, value)
	}

	w := &kafka.Writer{
		Addr:                   kafka.TCP(p.brokers...),
		Topic:                  topic,
		RequiredAcks:           kafka.RequireAll,
		MaxAttempts:            3,
		AllowAutoTopicCreation: true,
	}
	defer w.Close()

	var payload []byte
	var err error
	switch v := value.(type) {
	case []byte:
		payload = v
	default:
		payload, err = json.Marshal(v)
		if err != nil {
			return fmt.Errorf("marshal value: %w", err)
		}
	}

	err = w.WriteMessages(ctx, kafka.Message{
		Key:   []byte(key),
		Value: payload,
	})
	if err != nil {
		return fmt.Errorf("kafka write to topic %s: %w", topic, err)
	}
	return nil
}

func (p *Producer) PublishOrderCompleted(ctx context.Context, topic, key string, val interface{}) error {
	return p.Publish(ctx, topic, key, val)
}

func (p *Producer) Close() error {
	return nil
}
