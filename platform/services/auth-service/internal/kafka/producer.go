package kafka

import (
	"context"
	"encoding/json"
	"time"

	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog/log"
	kafkalib "github.com/segmentio/kafka-go"
)

const (
	TopicCustomerLogin = "customer.login"
)

// CustomerLoginEvent is published when a customer logs in.
type CustomerLoginEvent struct {
	UserID    string    `json:"user_id"`
	IsNewUser bool      `json:"is_new_user"`
	Timestamp time.Time `json:"timestamp"`
	StoreID   *string   `json:"store_id"`
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

// NewProducer creates a Kafka producer for the given topic.
func NewProducer(brokers []string, topic string) *Producer {
	w := &kafkalib.Writer{
		Addr:         kafkalib.TCP(brokers...),
		Topic:        topic,
		Balancer:     &kafkalib.LeastBytes{},
		BatchTimeout: 10 * time.Millisecond,
		Async:        true, // Fire-and-forget — never blocks the response
	}
	return NewProducerWithWriter(w)
}

func NewProducerWithWriter(w kafkaWriter) *Producer {
	return &Producer{
		writer:   w,
		marshalF: json.Marshal,
		asyncF: func(fn func()) {
			go fn()
		},
	}
}

// PublishLoginEvent publishes a customer login event.
// This is fire-and-forget — errors are logged but never block the caller.
func (p *Producer) PublishLoginEvent(ctx context.Context, userID string, isNewUser bool) {
	event := CustomerLoginEvent{
		UserID:    userID,
		IsNewUser: isNewUser,
		Timestamp: time.Now(),
		StoreID:   nil,
	}

	data, err := p.marshalF(event)
	if err != nil {
		log.Error().Err(err).Msg("failed to marshal login event")
		return
	}

	requestID := ""
	if ctx != nil {
		if v := ctx.Value(chimw.RequestIDKey); v != nil {
			if s, ok := v.(string); ok {
				requestID = s
			}
		}
		if requestID == "" {
			if v := ctx.Value("X-Request-ID"); v != nil {
				if s, ok := v.(string); ok {
					requestID = s
				}
			}
		}
	}

	var headers []kafkalib.Header
	if requestID != "" {
		headers = append(headers, kafkalib.Header{Key: "X-Request-ID", Value: []byte(requestID)})
	}

	// Use background context since this is async fire-and-forget
	p.asyncF(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := p.writer.WriteMessages(ctx, kafkalib.Message{
			Key:     []byte(userID),
			Value:   data,
			Headers: headers,
		})
		if err != nil {
			log.Error().Err(err).Str("user_id", userID).Msg("failed to publish login event")
		}
	})
}

// Close closes the Kafka writer.
func (p *Producer) Close() {
	if err := p.writer.Close(); err != nil {
		log.Error().Err(err).Msg("failed to close kafka producer")
	}
}
