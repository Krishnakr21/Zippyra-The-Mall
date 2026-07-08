package kafka

import (
	"context"
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
		Addr:     kafka.TCP(p.brokers...),
		Topic:    topic,
		Balancer: &kafka.LeastBytes{},
	}
	defer w.Close()

	var payload []byte
	switch v := value.(type) {
	case []byte:
		payload = v
	default:
		// JSON encoding could be added here if needed, but repository already stores []byte payload
		payload = []byte(fmt.Sprintf("%v", v))
	}

	err := w.WriteMessages(ctx, kafka.Message{
		Key:   []byte(key),
		Value: payload,
	})
	if err != nil {
		return fmt.Errorf("kafka write: %w", err)
	}
	return nil
}

func (p *Producer) Close() error {
	return nil
}
