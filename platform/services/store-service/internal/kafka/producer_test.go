package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	chimw "github.com/go-chi/chi/v5/middleware"
	kafkalib "github.com/segmentio/kafka-go"
)

// mockWriter is a mock kafka writer for testing
type mockWriter struct {
	messages []kafkalib.Message
	writeErr error
	closeErr error
}

func (m *mockWriter) WriteMessages(ctx context.Context, msgs ...kafkalib.Message) error {
	if m.writeErr != nil {
		return m.writeErr
	}
	m.messages = append(m.messages, msgs...)
	return nil
}

func (m *mockWriter) Close() error {
	return m.closeErr
}

func TestNewProducer(t *testing.T) {
	brokers := []string{"localhost:9092"}
	p, err := NewProducer(brokers)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if p == nil {
		t.Fatal("expected producer to be non-nil")
	}
}

func TestNewProducerWithWriter(t *testing.T) {
	mock := &mockWriter{}
	producer := NewProducerWithWriter(mock)
	if producer == nil {
		t.Error("expected producer to not be nil")
	}
	if producer.writer != mock {
		t.Error("expected writer to be mock")
	}
}

func TestNewProducer_EmptyBrokers(t *testing.T) {
	p, err := NewProducer([]string{})
	if err == nil {
		t.Error("expected error for empty brokers")
	}
	if p != nil {
		t.Error("expected nil producer")
	}
}

func TestGetCorrelationID(t *testing.T) {
	// Test with nil context
	id := getCorrelationID(nil)
	if id == "" {
		t.Error("expected non-empty correlation ID for nil context")
	}

	// Test with context that has request ID
	ctx := context.WithValue(context.Background(), chimw.RequestIDKey, "test-request-id")
	id = getCorrelationID(ctx)
	if id != "test-request-id" {
		t.Errorf("expected 'test-request-id', got '%s'", id)
	}

	// Test with context that has empty request ID
	ctx = context.WithValue(context.Background(), chimw.RequestIDKey, "")
	id = getCorrelationID(ctx)
	if id == "" {
		t.Error("expected non-empty correlation ID for empty request ID")
	}

	// Test with context that has non-string request ID
	ctx = context.WithValue(context.Background(), chimw.RequestIDKey, 123)
	id = getCorrelationID(ctx)
	if id == "" {
		t.Error("expected non-empty correlation ID for non-string request ID")
	}
}

func TestProducer_PublishCustomerEntered(t *testing.T) {
	mock := &mockWriter{}
	producer := NewProducerWithWriter(mock)

	// Use sync execution for testing
	executed := false
	producer.asyncF = func(fn func()) {
		fn()
		executed = true
	}

	ctx := context.WithValue(context.Background(), chimw.RequestIDKey, "test-correlation-id")
	producer.PublishCustomerEntered(ctx, "user-1", "store-1", "chain-1", "token-1")

	// Give async operation time to complete
	time.Sleep(100 * time.Millisecond)

	if !executed {
		t.Error("expected async function to be executed")
	}

	if len(mock.messages) != 1 {
		t.Errorf("expected 1 message, got %d", len(mock.messages))
	}

	msg := mock.messages[0]
	if msg.Topic != TopicCustomerEntered {
		t.Errorf("expected topic %s, got %s", TopicCustomerEntered, msg.Topic)
	}
	if string(msg.Key) != "store-1" {
		t.Errorf("expected key 'store-1', got '%s'", string(msg.Key))
	}

	// Verify event structure
	var event CustomerEnteredEvent
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		t.Fatalf("failed to unmarshal event: %v", err)
	}
	if event.UserID != "user-1" {
		t.Errorf("expected user_id 'user-1', got '%s'", event.UserID)
	}
	if event.StoreID != "store-1" {
		t.Errorf("expected store_id 'store-1', got '%s'", event.StoreID)
	}
	if event.EventType != TopicCustomerEntered {
		t.Errorf("expected event_type %s, got '%s'", TopicCustomerEntered, event.EventType)
	}
	if event.CorrelationID != "test-correlation-id" {
		t.Errorf("expected correlation_id 'test-correlation-id', got '%s'", event.CorrelationID)
	}
}

func TestProducer_PublishCustomerEntered_MarshalError(t *testing.T) {
	mock := &mockWriter{}
	producer := NewProducerWithWriter(mock)

	// Override marshalF to return error
	producer.marshalF = func(v any) ([]byte, error) {
		return nil, errors.New("marshal error")
	}

	producer.asyncF = func(fn func()) {
		fn()
	}

	// Should not panic, just log error
	producer.PublishCustomerEntered(context.Background(), "user-1", "store-1", "chain-1", "token-1")

	// No messages should be written
	if len(mock.messages) != 0 {
		t.Errorf("expected 0 messages, got %d", len(mock.messages))
	}
}

func TestProducer_PublishCustomerEntered_WriteError(t *testing.T) {
	mock := &mockWriter{writeErr: errors.New("write error")}
	producer := NewProducerWithWriter(mock)

	producer.asyncF = func(fn func()) {
		fn()
	}

	// Should not panic, just log error
	producer.PublishCustomerEntered(context.Background(), "user-1", "store-1", "chain-1", "token-1")

	// Wait for async
	time.Sleep(100 * time.Millisecond)

	// Message should be attempted but fail (error is logged, not returned)
	if len(mock.messages) != 0 {
		t.Errorf("expected 0 successful messages, got %d", len(mock.messages))
	}
}

func TestProducer_PublishCustomerExited(t *testing.T) {
	mock := &mockWriter{}
	producer := NewProducerWithWriter(mock)

	executed := false
	producer.asyncF = func(fn func()) {
		fn()
		executed = true
	}

	ctx := context.WithValue(context.Background(), chimw.RequestIDKey, "test-correlation-id")
	producer.PublishCustomerExited(ctx, "user-1", "store-1", "chain-1", 300)

	time.Sleep(100 * time.Millisecond)

	if !executed {
		t.Error("expected async function to be executed")
	}

	if len(mock.messages) != 1 {
		t.Errorf("expected 1 message, got %d", len(mock.messages))
	}

	msg := mock.messages[0]
	if msg.Topic != TopicCustomerExited {
		t.Errorf("expected topic %s, got %s", TopicCustomerExited, msg.Topic)
	}

	var event CustomerExitedEvent
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		t.Fatalf("failed to unmarshal event: %v", err)
	}
	if event.UserID != "user-1" {
		t.Errorf("expected user_id 'user-1', got '%s'", event.UserID)
	}
	if event.DurationSeconds != 300 {
		t.Errorf("expected duration_seconds 300, got %d", event.DurationSeconds)
	}
	if event.EventType != TopicCustomerExited {
		t.Errorf("expected event_type %s, got '%s'", TopicCustomerExited, event.EventType)
	}
}

func TestProducer_PublishCustomerExited_MarshalError(t *testing.T) {
	mock := &mockWriter{}
	producer := NewProducerWithWriter(mock)

	producer.marshalF = func(v any) ([]byte, error) {
		return nil, errors.New("marshal error")
	}

	producer.asyncF = func(fn func()) {
		fn()
	}

	producer.PublishCustomerExited(context.Background(), "user-1", "store-1", "chain-1", 300)

	if len(mock.messages) != 0 {
		t.Errorf("expected 0 messages, got %d", len(mock.messages))
	}
}

func TestProducer_PublishCustomerExited_WriteError(t *testing.T) {
	mock := &mockWriter{writeErr: errors.New("write error")}
	producer := NewProducerWithWriter(mock)

	producer.asyncF = func(fn func()) {
		fn()
	}

	producer.PublishCustomerExited(context.Background(), "user-1", "store-1", "chain-1", 300)

	time.Sleep(100 * time.Millisecond)

	if len(mock.messages) != 0 {
		t.Errorf("expected 0 successful messages, got %d", len(mock.messages))
	}
}

func TestProducer_Close(t *testing.T) {
	mock := &mockWriter{}
	producer := NewProducerWithWriter(mock)

	producer.Close()

	// Close should be called on the writer
	// Since mock doesn't track close calls, we just verify it doesn't panic
}

func TestProducer_Close_Error(t *testing.T) {
	mock := &mockWriter{closeErr: errors.New("close error")}
	producer := NewProducerWithWriter(mock)

	// Should not panic, just log error
	producer.Close()
}

// Test default asyncF behavior
func TestNewProducerWithWriter_DefaultAsync(t *testing.T) {
	mock := &mockWriter{}
	// Create producer without overriding asyncF - this tests the default closure
	producer := NewProducerWithWriter(mock)

	// Don't override asyncF - use the default one

	ctx := context.WithValue(context.Background(), chimw.RequestIDKey, "test-id")
	producer.PublishCustomerEntered(ctx, "user-1", "store-1", "chain-1", "token-1")

	// Wait for the async goroutine to complete
	time.Sleep(200 * time.Millisecond)

	// Verify message was written
	if len(mock.messages) != 1 {
		t.Errorf("expected 1 message, got %d", len(mock.messages))
	}
}
