package kafka

import (
	"context"
	"encoding/json"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/segmentio/kafka-go"

	"github.com/zippyra/platform/services/catalog-service/internal/model"
)

type Producer interface {
	PublishProductUpdated(ctx context.Context, product model.Product) error
	Close() error
}

type KafkaWriter interface {
	WriteMessages(ctx context.Context, msgs ...kafka.Message) error
	Close() error
}

type producer struct {
	writer KafkaWriter
	topic  string
}

func NewProducer(brokers []string, topic string) Producer {
	writer := &kafka.Writer{
		Addr:     kafka.TCP(brokers...),
		Topic:    topic,
		Balancer: &kafka.LeastBytes{},
		Async:    true,
	}

	return NewProducerWithWriter(writer, topic)
}

func NewProducerWithWriter(writer KafkaWriter, topic string) Producer {
	return &producer{
		writer: writer,
		topic:  topic,
	}
}

func (p *producer) PublishProductUpdated(ctx context.Context, product model.Product) error {
	event := map[string]interface{}{
		"type":      "product_updated",
		"timestamp": time.Now().UTC(),
		"product":   product,
	}

	data, err := json.Marshal(event)
	if err != nil {
		log.Error().Err(err).Str("productID", product.ID.String()).Msg("failed to marshal product event")
		return err
	}

	message := kafka.Message{
		Key:   []byte(product.ID.String()),
		Value: data,
		Headers: []kafka.Header{
			{Key: "event_type", Value: []byte("product_updated")},
			{Key: "store_id", Value: []byte(product.StoreID.String())},
		},
	}

	if err := p.writer.WriteMessages(ctx, message); err != nil {
		log.Error().Err(err).Str("productID", product.ID.String()).Str("topic", p.topic).Msg("failed to publish product event")
		return err
	}

	log.Debug().Str("productID", product.ID.String()).Str("topic", p.topic).Msg("published product event")
	return nil
}

func (p *producer) Close() error {
	if err := p.writer.Close(); err != nil {
		log.Error().Err(err).Str("topic", p.topic).Msg("failed to close kafka producer")
		return err
	}
	log.Info().Str("topic", p.topic).Msg("kafka producer closed")
	return nil
}
