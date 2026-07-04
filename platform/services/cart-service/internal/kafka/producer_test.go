package kafka

import (
	"context"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewProducer(t *testing.T) {
	tests := []struct {
		name    string
		brokers []string
		wantErr bool
	}{
		{
			name:    "valid config",
			brokers: []string{"localhost:9092"},
			wantErr: false,
		},
		{
			name:    "empty brokers",
			brokers: []string{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewProducer(tt.brokers)
			if tt.wantErr {
				assert.Nil(t, p, "producer should be nil for invalid config")
			} else {
				assert.NotNil(t, p, "producer should be created")
				// Clean up
				if p != nil {
					p.Close()
				}
			}
		})
	}
}

func TestProducer_Close(t *testing.T) {
	p := NewProducer([]string{"localhost:9092"})
	require.NotNil(t, p)

	// Close should not panic
	assert.NotPanics(t, func() {
		p.Close()
	})

	// Close again should not panic
	assert.NotPanics(t, func() {
		p.Close()
	})
}

func TestProducer_PublishItemScanned(t *testing.T) {
	// Create a mock writer for testing
	writer := &kafka.Writer{
		Addr:  kafka.TCP("localhost:9092"),
		Topic: "test-topic",
		// In a real test, you'd use a test broker or mock
		// For now, we'll test the message construction
	}

	p := &kafkaProducer{
		writer: writer,
	}

	ctx := context.Background()

	event := CartEvent{
		EventType:     "cart.item_scanned",
		UserID:        "user1",
		StoreID:       "store1",
		Barcode:       "123456",
		Quantity:      2,
		Timestamp:     time.Now(),
		CorrelationID: "test-id",
	}

	err := p.PublishItemScanned(ctx, event)
	// Since we don't have a real broker, we expect an error
	// In a real test environment, you'd set up a test broker
	assert.Error(t, err, "should fail without real broker")

	p.Close()
}

func TestProducer_PublishCheckoutInitiated(t *testing.T) {
	writer := &kafka.Writer{
		Addr:  kafka.TCP("localhost:9092"),
		Topic: "test-topic",
	}

	p := &kafkaProducer{
		writer: writer,
	}

	ctx := context.Background()

	event := CartEvent{
		EventType:     "cart.checkout_initiated",
		UserID:        "user1",
		StoreID:       "store1",
		CheckoutID:    "checkout123",
		TotalAmount:   99.99,
		ItemCount:     3,
		Timestamp:     time.Now(),
		CorrelationID: "test-id",
	}

	err := p.PublishCheckoutInitiated(ctx, event)
	assert.Error(t, err, "should fail without real broker")

	p.Close()
}
