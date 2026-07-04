package kafka

import (
	"context"
	"fmt"
	"math"
	"testing"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
	"github.com/zippyra/platform/services/catalog-service/internal/model"
)

func TestProducer_PublishProductUpdated(t *testing.T) {
	ctx := context.Background()
	topic := "product-updates"
	product := model.Product{
		ID:      uuid.New(),
		StoreID: uuid.New(),
		Name:    "Test Product",
	}

	t.Run("Success", func(t *testing.T) {
		mockWriter := &mockKafkaWriter{
			writeMessagesFunc: func(ctx context.Context, msgs ...kafka.Message) error {
				assert.Equal(t, 1, len(msgs))
				assert.Equal(t, product.ID.String(), string(msgs[0].Key))
				return nil
			},
		}

		p := NewProducerWithWriter(mockWriter, topic)
		err := p.PublishProductUpdated(ctx, product)
		assert.NoError(t, err)
	})

	t.Run("Write Error", func(t *testing.T) {
		mockWriter := &mockKafkaWriter{
			writeMessagesFunc: func(ctx context.Context, msgs ...kafka.Message) error {
				return fmt.Errorf("kafka error")
			},
		}

		p := NewProducerWithWriter(mockWriter, topic)
		err := p.PublishProductUpdated(ctx, product)
		assert.Error(t, err)
	})

	t.Run("Marshal Error", func(t *testing.T) {
		p := NewProducerWithWriter(nil, topic) // writer not needed for marshal check
		invalidProduct := product
		invalidProduct.SellingPrice = math.NaN()
		err := p.PublishProductUpdated(ctx, invalidProduct)
		assert.Error(t, err)
	})
}

func TestProducer_Close(t *testing.T) {
	topic := "product-updates"

	t.Run("Success", func(t *testing.T) {
		mockWriter := &mockKafkaWriter{
			closeFunc: func() error {
				return nil
			},
		}

		p := NewProducerWithWriter(mockWriter, topic)
		err := p.Close()
		assert.NoError(t, err)
	})

	t.Run("Error", func(t *testing.T) {
		mockWriter := &mockKafkaWriter{
			closeFunc: func() error {
				return fmt.Errorf("close error")
			},
		}

		p := NewProducerWithWriter(mockWriter, topic)
		err := p.Close()
		assert.Error(t, err)
	})
}

func TestNewProducer(t *testing.T) {
	p := NewProducer([]string{"localhost:9092"}, "test-topic")
	assert.NotNil(t, p)
	// We don't want to actually connect in unit tests, so we just check if it returns
}
