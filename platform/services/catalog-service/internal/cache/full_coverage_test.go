package cache

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/zippyra/platform/services/catalog-service/internal/model"
)

type mockRedisClientFull struct {
	mock.Mock
}

func (m *mockRedisClientFull) Get(ctx context.Context, key string) *redis.StringCmd {
	args := m.Called(ctx, key)
	cmd := redis.NewStringCmd(ctx)
	if args.Get(0) != nil {
		cmd.SetVal(args.String(0))
	}
	if args.Get(1) != nil {
		cmd.SetErr(args.Error(1))
	}
	return cmd
}

func (m *mockRedisClientFull) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd {
	args := m.Called(ctx, key, value, expiration)
	cmd := redis.NewStatusCmd(ctx)
	if args.Get(0) != nil {
		cmd.SetVal(args.String(0))
	}
	if args.Get(1) != nil {
		cmd.SetErr(args.Error(1))
	}
	return cmd
}

func (m *mockRedisClientFull) Del(ctx context.Context, keys ...string) *redis.IntCmd {
	args := m.Called(ctx, keys)
	cmd := redis.NewIntCmd(ctx)
	if args.Get(0) != nil {
		cmd.SetVal(args.Get(0).(int64))
	}
	if args.Get(1) != nil {
		cmd.SetErr(args.Error(1))
	}
	return cmd
}

func (m *mockRedisClientFull) Pipeline() Pipeliner {
	args := m.Called()
	if args.Get(0) != nil {
		return args.Get(0).(Pipeliner)
	}
	return nil
}

type mockPipelinerFull struct {
	mock.Mock
}

func (m *mockPipelinerFull) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd {
	args := m.Called(ctx, key, value, expiration)
	cmd := redis.NewStatusCmd(ctx)
	if args.Get(0) != nil {
		cmd.SetVal(args.String(0))
	}
	return cmd
}

func (m *mockPipelinerFull) Exec(ctx context.Context) ([]redis.Cmder, error) {
	args := m.Called(ctx)
	if args.Get(1) != nil {
		return nil, args.Error(1)
	}
	return []redis.Cmder{}, nil
}

func TestSKUCache_FullCoverage(t *testing.T) {
	mockRedis := new(mockRedisClientFull)
	cache := &SKUCache{
		redis:      mockRedis,
		marshaller: &jsonMarshaller{},
		ttl:        time.Hour,
	}

	storeID := uuid.New().String()
	barcode := "123456789"
	key := "sku:" + storeID + ":" + barcode

	// Test Get - cache hit
	product := &model.Product{
		ID:       uuid.New(),
		StoreID:  uuid.MustParse(storeID),
		Barcode:  barcode,
		Name:     "Test Product",
		IsActive: true,
	}
	data, _ := json.Marshal(product)
	mockRedis.On("Get", mock.Anything, key).Return(string(data), nil)

	result, err := cache.Get(context.Background(), storeID, barcode)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, product.Name, result.Name)
	mockRedis.ExpectedCalls = nil

	// Test Get - cache miss
	mockRedis.On("Get", mock.Anything, key).Return("", redis.Nil)
	result, err = cache.Get(context.Background(), storeID, barcode)
	assert.NoError(t, err)
	assert.Nil(t, result)
	mockRedis.ExpectedCalls = nil

	// Test Get - Redis error
	mockRedis.On("Get", mock.Anything, key).Return("", assert.AnError)
	result, err = cache.Get(context.Background(), storeID, barcode)
	assert.Error(t, err)
	assert.Nil(t, result)
	mockRedis.ExpectedCalls = nil

	// Test Set
	mockRedis.On("Set", mock.Anything, key, mock.AnythingOfType("[]uint8"), cache.ttl).Return("OK", nil)
	err = cache.Set(context.Background(), storeID, barcode, product)
	assert.NoError(t, err)
	mockRedis.ExpectedCalls = nil

	// Test Set - Redis error
	mockRedis.On("Set", mock.Anything, key, mock.AnythingOfType("[]uint8"), cache.ttl).Return("", assert.AnError)
	err = cache.Set(context.Background(), storeID, barcode, product)
	assert.Error(t, err)
	mockRedis.ExpectedCalls = nil

	// Test Invalidate
	mockRedis.On("Del", mock.Anything, []string{key}).Return(int64(1), nil)
	err = cache.Invalidate(context.Background(), storeID, barcode)
	assert.NoError(t, err)
	mockRedis.ExpectedCalls = nil

	// Test Invalidate - Redis error
	mockRedis.On("Del", mock.Anything, []string{key}).Return(int64(0), assert.AnError)
	err = cache.Invalidate(context.Background(), storeID, barcode)
	assert.Error(t, err)
	mockRedis.ExpectedCalls = nil

	// Test WarmStore - successful
	products := []model.Product{
		{ID: uuid.New(), Barcode: "123456789", Name: "Product 1"},
		{ID: uuid.New(), Barcode: "987654321", Name: "Product 2"},
	}
	mockPipeline := new(mockPipelinerFull)
	mockRedis.On("Pipeline").Return(mockPipeline)
	mockPipeline.On("Set", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("[]uint8"), cache.ttl).Return("OK", nil).Times(len(products))
	mockPipeline.On("Exec", mock.Anything).Return([]redis.Cmder{}, nil)

	err = cache.WarmStore(context.Background(), storeID, products)
	assert.NoError(t, err)
	mockRedis.ExpectedCalls = nil
	mockPipeline.ExpectedCalls = nil

	// Test WarmStore - pipeline error
	mockRedis.On("Pipeline").Return(mockPipeline)
	mockPipeline.On("Set", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("[]uint8"), cache.ttl).Return("OK", nil).Times(len(products))
	mockPipeline.On("Exec", mock.Anything).Return([]redis.Cmder{}, assert.AnError)

	err = cache.WarmStore(context.Background(), storeID, products)
	assert.Error(t, err)
	mockRedis.ExpectedCalls = nil
	mockPipeline.ExpectedCalls = nil

	// Test WarmStore - empty products
	err = cache.WarmStore(context.Background(), storeID, []model.Product{})
	assert.NoError(t, err)

	// Test GetOffers - cache hit
	offerKey := "offer_rules:" + uuid.New().String()
	offers := &model.OfferResponse{
		Offers: []model.OfferRule{
			{
				ID:        uuid.New(),
				StoreID:   uuid.New(),
				Name:      "Test Offer",
				Type:      "discount",
				Value:     10.0,
				IsActive:  true,
				ValidFrom: time.Now(),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
		},
	}
	offerData, _ := json.Marshal(offers)
	mockRedis.On("Get", mock.Anything, offerKey).Return(string(offerData), nil)

	resultOffers, err := cache.GetOffers(context.Background(), offerKey)
	assert.NoError(t, err)
	assert.NotNil(t, resultOffers)
	assert.Equal(t, 1, len(resultOffers.Offers))
	mockRedis.ExpectedCalls = nil

	// Test GetOffers - cache miss
	mockRedis.On("Get", mock.Anything, offerKey).Return("", redis.Nil)
	resultOffers, err = cache.GetOffers(context.Background(), offerKey)
	assert.NoError(t, err)
	assert.Nil(t, resultOffers)
	mockRedis.ExpectedCalls = nil

	// Test GetOffers - Redis error
	mockRedis.On("Get", mock.Anything, offerKey).Return("", assert.AnError)
	resultOffers, err = cache.GetOffers(context.Background(), offerKey)
	assert.Error(t, err)
	assert.Nil(t, resultOffers)
	mockRedis.ExpectedCalls = nil

	// Test SetOffers
	mockRedis.On("Set", mock.Anything, offerKey, mock.AnythingOfType("[]uint8"), time.Hour).Return("OK", nil)
	err = cache.SetOffers(context.Background(), offerKey, offers)
	assert.NoError(t, err)
	mockRedis.ExpectedCalls = nil

	// Test SetOffers - Redis error
	mockRedis.On("Set", mock.Anything, offerKey, mock.AnythingOfType("[]uint8"), time.Hour).Return("", assert.AnError)
	err = cache.SetOffers(context.Background(), offerKey, offers)
	assert.Error(t, err)
}

func TestNewSKUCache_FullCoverage(t *testing.T) {
	// Test with nil client
	cache := NewSKUCache(nil)
	assert.NotNil(t, cache)
	assert.Nil(t, cache.redis)
	assert.Equal(t, 24*time.Hour, cache.ttl)

	// Test with interface (cannot use mock directly due to type mismatch)
	cache = NewSKUCache(nil)
	assert.NotNil(t, cache)
	assert.Equal(t, 24*time.Hour, cache.ttl)
}
