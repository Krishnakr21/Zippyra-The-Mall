package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	kafkalib "github.com/segmentio/kafka-go"
)

const (
	TopicCustomerEntered = "store.customer_entered"
	TopicCustomerExited  = "store.customer_exited"
)

// CustomerEnteredEvent is published when a customer enters a store.
type CustomerEnteredEvent struct {
	EventType     string    `json:"event_type"`
	UserID        string    `json:"user_id"`
	StoreID       string    `json:"store_id"`
	ChainID       string    `json:"chain_id"`
	QRTokenID     string    `json:"qr_token_id"`
	Timestamp     time.Time `json:"timestamp"`
	CorrelationID string    `json:"correlation_id"`
}

// CustomerExitedEvent is published when a customer exits a store.
type CustomerExitedEvent struct {
	EventType       string    `json:"event_type"`
	UserID          string    `json:"user_id"`
	StoreID         string    `json:"store_id"`
	ChainID         string    `json:"chain_id"`
	DurationSeconds int64     `json:"duration_seconds"`
	Timestamp       time.Time `json:"timestamp"`
	CorrelationID   string    `json:"correlation_id"`
}

// Producer wraps a Kafka writer for fire-and-forget publishing.
type Producer struct {
	writer   kafkaWriter
	marshalF func(v any) ([]byte, error)
	asyncF   func(fn func())
}

type kafkaWriter interface {
	WriteMessages(ctx context.Context, msgs ...kafkalib.Message) error
	Close() error
}

// NewProducer creates a Kafka producer for store events.
func NewProducer(brokers []string) (*Producer, error) {
	if len(brokers) == 0 {
		return nil, fmt.Errorf("no kafka brokers provided")
	}
	w := &kafkalib.Writer{
		Addr:         kafkalib.TCP(brokers...),
		Balancer:     &kafkalib.LeastBytes{},
		BatchTimeout: 10 * time.Millisecond,
		Async:        true,
	}
	return NewProducerWithWriter(w), nil
}

// NewProducerWithWriter creates a Producer with a custom writer (for testing).
func NewProducerWithWriter(w kafkaWriter) *Producer {
	return &Producer{
		writer:   w,
		marshalF: json.Marshal,
		asyncF: func(fn func()) {
			go fn()
		},
	}
}

func getCorrelationID(ctx context.Context) string {
	if ctx == nil {
		return uuid.New().String()
	}
	if v := ctx.Value(chimw.RequestIDKey); v != nil {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	return uuid.New().String()
}

// PublishCustomerEntered publishes a store.customer_entered event.
// Partition key is store_id. Fire-and-forget — errors are logged, never block.
func (p *Producer) PublishCustomerEntered(ctx context.Context, userID, storeID, chainID, qrTokenID string) {
	correlationID := getCorrelationID(ctx)

	event := CustomerEnteredEvent{
		EventType:     TopicCustomerEntered,
		UserID:        userID,
		StoreID:       storeID,
		ChainID:       chainID,
		QRTokenID:     qrTokenID,
		Timestamp:     time.Now(),
		CorrelationID: correlationID,
	}

	data, err := p.marshalF(event)
	if err != nil {
		log.Error().Err(err).Msg("failed to marshal customer_entered event")
		return
	}

	headers := []kafkalib.Header{
		{Key: "correlation_id", Value: []byte(correlationID)},
	}

	p.asyncF(func() {
		wCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := p.writer.WriteMessages(wCtx, kafkalib.Message{
			Topic:   TopicCustomerEntered,
			Key:     []byte(storeID), // Partition key: always store_id
			Value:   data,
			Headers: headers,
		})
		if err != nil {
			log.Error().Err(err).Str("store_id", storeID).Msg("failed to publish customer_entered event")
		}
	})
}

// PublishCustomerExited publishes a store.customer_exited event.
// Partition key is store_id. Fire-and-forget — errors are logged, never block.
func (p *Producer) PublishCustomerExited(ctx context.Context, userID, storeID, chainID string, durationSeconds int64) {
	correlationID := getCorrelationID(ctx)

	event := CustomerExitedEvent{
		EventType:       TopicCustomerExited,
		UserID:          userID,
		StoreID:         storeID,
		ChainID:         chainID,
		DurationSeconds: durationSeconds,
		Timestamp:       time.Now(),
		CorrelationID:   correlationID,
	}

	data, err := p.marshalF(event)
	if err != nil {
		log.Error().Err(err).Msg("failed to marshal customer_exited event")
		return
	}

	headers := []kafkalib.Header{
		{Key: "correlation_id", Value: []byte(correlationID)},
	}

	p.asyncF(func() {
		wCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := p.writer.WriteMessages(wCtx, kafkalib.Message{
			Topic:   TopicCustomerExited,
			Key:     []byte(storeID),
			Value:   data,
			Headers: headers,
		})
		if err != nil {
			log.Error().Err(err).Str("store_id", storeID).Msg("failed to publish customer_exited event")
		}
	})
}

// Close closes the Kafka writer.
func (p *Producer) Close() {
	if err := p.writer.Close(); err != nil {
		log.Error().Err(err).Msg("failed to close kafka producer")
	}
}
