package kafka

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/zippyra/platform/services/catalog-service/internal/model"
)

func TestProducer(t *testing.T) {
	// Test producer creation with nil brokers
	producer := NewProducer(nil, "test-topic")
	assert.NotNil(t, producer)

	// Test that methods exist (they will fail with nil brokers, but we're testing interface compliance)
	ctx := context.Background()
	product := model.Product{
		Name: "Test Product",
	}

	// These calls should not panic with nil brokers
	assert.NotPanics(t, func() {
		producer.PublishProductUpdated(ctx, product)
	})

	assert.NotPanics(t, func() {
		producer.Close()
	})
}
