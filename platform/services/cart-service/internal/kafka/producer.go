package kafka

import (
	"context"
	"encoding/json"
	"time"

	"github.com/segmentio/kafka-go"
)

type CartEvent struct {
	EventType     string    `json:"event_type"`
	UserID        string    `json:"user_id"`
	StoreID       string    `json:"store_id"`
	Barcode       string    `json:"barcode,omitempty"`
	ProductID     string    `json:"product_id,omitempty"`
	ProductName   string    `json:"product_name,omitempty"`
	Quantity      int       `json:"quantity,omitempty"`
	UnitPrice     float64   `json:"unit_price,omitempty"`
	GSTAmount     float64   `json:"gst_amount,omitempty"`
	TotalAmount   float64   `json:"total_amount,omitempty"`
	ItemCount     int       `json:"item_count,omitempty"`
	CheckoutID    string    `json:"checkout_id,omitempty"`
	Timestamp     time.Time `json:"timestamp"`
	CorrelationID string    `json:"correlation_id"`
}

type Producer interface {
	PublishItemScanned(ctx context.Context, event CartEvent) error
	PublishCheckoutInitiated(ctx context.Context, event CartEvent) error
	Close() error
}

type kafkaProducer struct {
	writer *kafka.Writer
}

func NewProducer(brokers []string) Producer {
	if len(brokers) == 0 {
		return nil
	}
	return &kafkaProducer{
		writer: &kafka.Writer{
			Addr:     kafka.TCP(brokers...),
			Topic:    "", // Topic will be set per message or default
			Balancer: &kafka.Hash{},
		},
	}
}

func (p *kafkaProducer) PublishItemScanned(ctx context.Context, event CartEvent) error {
	event.EventType = "cart.item_scanned"
	event.Timestamp = time.Now()
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	return p.writer.WriteMessages(ctx, kafka.Message{
		Topic: "cart.item_scanned",
		Key:   []byte(event.StoreID),
		Value: data,
	})
}

func (p *kafkaProducer) PublishCheckoutInitiated(ctx context.Context, event CartEvent) error {
	event.EventType = "cart.checkout_initiated"
	event.Timestamp = time.Now()
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	return p.writer.WriteMessages(ctx, kafka.Message{
		Topic: "cart.checkout_initiated",
		Key:   []byte(event.StoreID),
		Value: data,
	})
}

func (p *kafkaProducer) Close() error {
	return p.writer.Close()
}
