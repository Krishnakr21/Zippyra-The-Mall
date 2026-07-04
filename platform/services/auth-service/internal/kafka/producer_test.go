package kafka

import (
	"context"
	"errors"
	"testing"
	"time"

	chimw "github.com/go-chi/chi/v5/middleware"
	kafkalib "github.com/segmentio/kafka-go"
)

type mockWriter struct {
	msgs []kafkalib.Message
	err  error
}

func TestProducer_PublishLoginEvent_RequestIDFallbackKey(t *testing.T) {
	w := &mockWriter{}
	p := NewProducerWithWriter(w)
	p.asyncF = func(fn func()) { fn() }

	ctx := context.WithValue(context.Background(), "X-Request-ID", "req-xyz")
	p.PublishLoginEvent(ctx, "user-1", true)

	if len(w.msgs) != 1 {
		t.Fatalf("expected 1 msg, got %d", len(w.msgs))
	}
	if len(w.msgs[0].Headers) != 1 {
		t.Fatalf("expected 1 header, got %d", len(w.msgs[0].Headers))
	}
	if string(w.msgs[0].Headers[0].Value) != "req-xyz" {
		t.Fatalf("unexpected header value: %q", string(w.msgs[0].Headers[0].Value))
	}
}

func TestProducer_PublishLoginEvent_NoRequestID_NoHeaders(t *testing.T) {
	w := &mockWriter{}
	p := NewProducerWithWriter(w)
	p.asyncF = func(fn func()) { fn() }

	p.PublishLoginEvent(context.Background(), "user-1", true)
	if len(w.msgs) != 1 {
		t.Fatalf("expected 1 msg, got %d", len(w.msgs))
	}
	if len(w.msgs[0].Headers) != 0 {
		t.Fatalf("expected 0 headers, got %d", len(w.msgs[0].Headers))
	}
}

func (m *mockWriter) WriteMessages(ctx context.Context, msgs ...kafkalib.Message) error {
	m.msgs = append(m.msgs, msgs...)
	return m.err
}

func (m *mockWriter) Close() error { return nil }

func TestProducer_PublishLoginEvent_AddsRequestIDHeader(t *testing.T) {
	w := &mockWriter{}
	p := NewProducerWithWriter(w)
	p.asyncF = func(fn func()) { fn() }

	ctx := context.WithValue(context.Background(), chimw.RequestIDKey, "req-123")
	p.PublishLoginEvent(ctx, "user-1", true)

	if len(w.msgs) != 1 {
		t.Fatalf("expected 1 msg, got %d", len(w.msgs))
	}
	if len(w.msgs[0].Headers) != 1 {
		t.Fatalf("expected 1 header, got %d", len(w.msgs[0].Headers))
	}
	if w.msgs[0].Headers[0].Key != "X-Request-ID" {
		t.Fatalf("unexpected header key: %q", w.msgs[0].Headers[0].Key)
	}
	if string(w.msgs[0].Headers[0].Value) != "req-123" {
		t.Fatalf("unexpected header value: %q", string(w.msgs[0].Headers[0].Value))
	}
}

func TestProducer_PublishLoginEvent_MarshalError(t *testing.T) {
	w := &mockWriter{}
	p := NewProducerWithWriter(w)
	p.asyncF = func(fn func()) { fn() }
	p.marshalF = func(v any) ([]byte, error) { return nil, errors.New("marshal") }

	p.PublishLoginEvent(context.Background(), "user-1", false)
	if len(w.msgs) != 0 {
		t.Fatal("expected no kafka messages")
	}
}

func TestProducer_PublishLoginEvent_WriteError_DoesNotPanic(t *testing.T) {
	w := &mockWriter{err: errors.New("write")}
	p := NewProducerWithWriter(w)
	p.asyncF = func(fn func()) { fn() }

	p.PublishLoginEvent(context.Background(), "user-1", false)
}

type closeErrWriter struct{}

func (c closeErrWriter) WriteMessages(ctx context.Context, msgs ...kafkalib.Message) error {
	return nil
}
func (c closeErrWriter) Close() error { return errors.New("close") }

func TestProducer_Close_DoesNotPanicOnError(t *testing.T) {
	p := NewProducerWithWriter(closeErrWriter{})
	p.Close()
}

func TestNewProducer_DefaultAsyncRunnerExecutes(t *testing.T) {
	p := NewProducer([]string{"localhost:9092"}, "topic")
	if p == nil {
		t.Fatal("expected producer")
	}
	if p.asyncF == nil {
		t.Fatal("expected async runner")
	}

	done := make(chan struct{})
	p.asyncF(func() {
		close(done)
	})

	select {
	case <-done:
		// ok
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for async runner")
	}
}
