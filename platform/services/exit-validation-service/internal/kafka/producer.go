package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

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

func (p *Producer) PublishCustomerExited(ctx context.Context, userID, storeID, orderID, gateID, correlationID string) error {
	event := map[string]interface{}{
		"event_type":     "store.customer_exited",
		"user_id":        userID,
		"store_id":       storeID,
		"order_id":       orderID,
		"gate_id":        gateID,
		"exit_method":    "QR",
		"timestamp":      time.Now().UTC().Format(time.RFC3339),
		"correlation_id": correlationID,
	}
	return p.Publish(ctx, "store.customer_exited", storeID, event)
}

func (p *Producer) PublishAlarm(ctx context.Context, alarmType, userID, storeID, gateID string) error {
	event := map[string]interface{}{
		"event_type": "exit.alarm",
		"alarm_type": alarmType,
		"user_id":    userID,
		"store_id":   storeID,
		"gate_id":    gateID,
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
	}
	return p.Publish(ctx, "exit.alarm", storeID, event)
}

func (p *Producer) PublishStaffOverride(ctx context.Context, staffID, userID, storeID, gateID, reason string) error {
	event := map[string]interface{}{
		"event_type": "exit.staff_override",
		"staff_id":   staffID,
		"user_id":    userID,
		"store_id":   storeID,
		"gate_id":    gateID,
		"reason":     reason,
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
	}
	return p.Publish(ctx, "exit.staff_override", storeID, event)
}

func (p *Producer) Close() error {
	return nil
}
