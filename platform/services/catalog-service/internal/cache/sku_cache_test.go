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

type mockRedisClient struct {
	mock.Mock
}

func (m *mockRedisClient) Get(ctx context.Context, key string) *redis.StringCmd {
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

func (m *mockRedisClient) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd {
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

func (m *mockRedisClient) Del(ctx context.Context, keys ...string) *redis.IntCmd {
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

func (m *mockRedisClient) Pipeline() Pipeliner {
	args := m.Called()
	if args.Get(0) != nil {
		return args.Get(0).(Pipeliner)
	}
	return nil
}

type mockPipeliner struct {
	mock.Mock
}

func (m *mockPipeliner) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd {
	args := m.Called(ctx, key, value, expiration)
	cmd := redis.NewStatusCmd(ctx)
	if args.Get(0) != nil {
		cmd.SetVal(args.String(0))
	}
	return cmd
}

func (m *mockPipeliner) Exec(ctx context.Context) ([]redis.Cmder, error) {
	args := m.Called(ctx)
	if args.Get(1) != nil {
		return nil, args.Error(1)
	}
	return []redis.Cmder{}, nil
}

type mockMarshaller struct {
	mock.Mock
}

func (m *mockMarshaller) Marshal(v interface{}) ([]byte, error) {
	args := m.Called(v)
	if args.Get(0) != nil {
		return args.Get(0).([]byte), nil
	}
	return nil, args.Error(1)
}

func (m *mockMarshaller) Unmarshal(data []byte, v interface{}) error {
	args := m.Called(data, v)
	if args.Error(0) != nil {
		return args.Error(0)
	}
	// Actually unmarshal if no error or it's a "real" unmarshal test
	if string(data) == "{}" || string(data) == "[]" || (len(data) > 0 && data[0] == '{') {
		return json.Unmarshal(data, v)
	}
	return nil
}

func TestSKUCache_Get(t *testing.T) {
	mockRedis := new(mockRedisClient)
	mockMarsh := new(mockMarshaller)
	cache := &SKUCache{redis: mockRedis, marshaller: mockMarsh, ttl: time.Hour}

	storeID := uuid.New().String()
	barcode := "123456789"
	key := "sku:" + storeID + ":" + barcode

	t.Run("cache hit", func(t *testing.T) {
		product := &model.Product{Name: "Test"}
		data, _ := json.Marshal(product)
		mockRedis.On("Get", mock.Anything, key).Return(string(data), nil).Once()
		mockMarsh.On("Unmarshal", data, mock.Anything).Return(nil).Once()
		
		res, err := cache.Get(context.Background(), storeID, barcode)
		assert.NoError(t, err)
		assert.NotNil(t, res)
	})

	t.Run("cache miss", func(t *testing.T) {
		mockRedis.On("Get", mock.Anything, key).Return("", redis.Nil).Once()
		res, err := cache.Get(context.Background(), storeID, barcode)
		assert.NoError(t, err)
		assert.Nil(t, res)
	})

	t.Run("redis error", func(t *testing.T) {
		mockRedis.On("Get", mock.Anything, key).Return("", assert.AnError).Once()
		_, err := cache.Get(context.Background(), storeID, barcode)
		assert.Error(t, err)
	})

	t.Run("unmarshal error", func(t *testing.T) {
		mockRedis.On("Get", mock.Anything, key).Return("data", nil).Once()
		mockMarsh.On("Unmarshal", []byte("data"), mock.Anything).Return(assert.AnError).Once()
		_, err := cache.Get(context.Background(), storeID, barcode)
		assert.Error(t, err)
	})
}

func TestSKUCache_Set(t *testing.T) {
	mockRedis := new(mockRedisClient)
	mockMarsh := new(mockMarshaller)
	cache := &SKUCache{redis: mockRedis, marshaller: mockMarsh, ttl: time.Hour}

	storeID := uuid.New().String()
	barcode := "123"
	product := &model.Product{Name: "Test"}
	key := "sku:" + storeID + ":" + barcode

	t.Run("Success", func(t *testing.T) {
		data := []byte("{}")
		mockMarsh.On("Marshal", product).Return(data, nil).Once()
		mockRedis.On("Set", mock.Anything, key, data, cache.ttl).Return("OK", nil).Once()
		err := cache.Set(context.Background(), storeID, barcode, product)
		assert.NoError(t, err)
	})

	t.Run("Marshal Error", func(t *testing.T) {
		mockMarsh.On("Marshal", product).Return(nil, assert.AnError).Once()
		err := cache.Set(context.Background(), storeID, barcode, product)
		assert.Error(t, err)
	})

	t.Run("Redis Error", func(t *testing.T) {
		data := []byte("{}")
		mockMarsh.On("Marshal", product).Return(data, nil).Once()
		mockRedis.On("Set", mock.Anything, key, data, cache.ttl).Return("", assert.AnError).Once()
		err := cache.Set(context.Background(), storeID, barcode, product)
		assert.Error(t, err)
	})
}

func TestSKUCache_Invalidate(t *testing.T) {
	mockRedis := new(mockRedisClient)
	cache := &SKUCache{redis: mockRedis, ttl: time.Hour}
	storeID := "s"
	barcode := "b"
	key := "sku:s:b"

	t.Run("Success", func(t *testing.T) {
		mockRedis.On("Del", mock.Anything, []string{key}).Return(int64(1), nil).Once()
		err := cache.Invalidate(context.Background(), storeID, barcode)
		assert.NoError(t, err)
	})

	t.Run("Error", func(t *testing.T) {
		mockRedis.On("Del", mock.Anything, []string{key}).Return(int64(0), assert.AnError).Once()
		err := cache.Invalidate(context.Background(), storeID, barcode)
		assert.Error(t, err)
	})
}

func TestSKUCache_WarmStore(t *testing.T) {
	mockRedis := new(mockRedisClient)
	mockMarsh := new(mockMarshaller)
	mockPipe := new(mockPipeliner)
	cache := &SKUCache{redis: mockRedis, marshaller: mockMarsh, ttl: time.Hour}
	
	products := []model.Product{{Barcode: "1"}, {Barcode: "2"}}

	t.Run("Success", func(t *testing.T) {
		mockRedis.On("Pipeline").Return(mockPipe).Once()
		mockMarsh.On("Marshal", mock.Anything).Return([]byte("{}"), nil).Twice()
		mockPipe.On("Set", mock.Anything, mock.Anything, mock.Anything, cache.ttl).Return("OK", nil).Twice()
		mockPipe.On("Exec", mock.Anything).Return([]redis.Cmder{}, nil).Once()
		
		err := cache.WarmStore(context.Background(), "s", products)
		assert.NoError(t, err)
	})

	t.Run("Empty", func(t *testing.T) {
		err := cache.WarmStore(context.Background(), "s", nil)
		assert.NoError(t, err)
	})

	t.Run("Marshal Error", func(t *testing.T) {
		mockRedis.On("Pipeline").Return(mockPipe).Once()
		// First product fails marshal, second succeeds
		mockMarsh.On("Marshal", mock.Anything).Return(nil, assert.AnError).Once()
		mockMarsh.On("Marshal", mock.Anything).Return([]byte("{}"), nil).Once()
		mockPipe.On("Set", mock.Anything, mock.Anything, mock.Anything, cache.ttl).Return("OK", nil).Once()
		mockPipe.On("Exec", mock.Anything).Return([]redis.Cmder{}, nil).Once()
		
		err := cache.WarmStore(context.Background(), "s", products)
		assert.NoError(t, err) // Continues on marshal error
	})

	t.Run("Exec Error", func(t *testing.T) {
		mockRedis.On("Pipeline").Return(mockPipe).Once()
		mockMarsh.On("Marshal", mock.Anything).Return([]byte("{}"), nil).Twice()
		mockPipe.On("Set", mock.Anything, mock.Anything, mock.Anything, cache.ttl).Return("OK", nil).Twice()
		mockPipe.On("Exec", mock.Anything).Return(nil, assert.AnError).Once()
		
		err := cache.WarmStore(context.Background(), "s", products)
		assert.Error(t, err)
	})
}

func TestSKUCache_GetOffers(t *testing.T) {
	mockRedis := new(mockRedisClient)
	mockMarsh := new(mockMarshaller)
	cache := &SKUCache{redis: mockRedis, marshaller: mockMarsh, ttl: time.Hour}
	key := "o"

	t.Run("Hit", func(t *testing.T) {
		mockRedis.On("Get", mock.Anything, key).Return("{}", nil).Once()
		mockMarsh.On("Unmarshal", []byte("{}"), mock.Anything).Return(nil).Once()
		_, err := cache.GetOffers(context.Background(), key)
		assert.NoError(t, err)
	})

	t.Run("Miss", func(t *testing.T) {
		mockRedis.On("Get", mock.Anything, key).Return("", redis.Nil).Once()
		res, err := cache.GetOffers(context.Background(), key)
		assert.NoError(t, err)
		assert.Nil(t, res)
	})

	t.Run("Redis Error", func(t *testing.T) {
		mockRedis.On("Get", mock.Anything, key).Return("", assert.AnError).Once()
		_, err := cache.GetOffers(context.Background(), key)
		assert.Error(t, err)
	})

	t.Run("Unmarshal Error", func(t *testing.T) {
		mockRedis.On("Get", mock.Anything, key).Return("{}", nil).Once()
		mockMarsh.On("Unmarshal", []byte("{}"), mock.Anything).Return(assert.AnError).Once()
		_, err := cache.GetOffers(context.Background(), key)
		assert.Error(t, err)
	})
}

func TestSKUCache_SetOffers(t *testing.T) {
	mockRedis := new(mockRedisClient)
	mockMarsh := new(mockMarshaller)
	cache := &SKUCache{redis: mockRedis, marshaller: mockMarsh, ttl: time.Hour}
	key := "o"
	offers := &model.OfferResponse{}

	t.Run("Success", func(t *testing.T) {
		mockMarsh.On("Marshal", offers).Return([]byte("{}"), nil).Once()
		mockRedis.On("Set", mock.Anything, key, []byte("{}"), time.Hour).Return("OK", nil).Once()
		err := cache.SetOffers(context.Background(), key, offers)
		assert.NoError(t, err)
	})

	t.Run("Marshal Error", func(t *testing.T) {
		mockMarsh.On("Marshal", offers).Return(nil, assert.AnError).Once()
		err := cache.SetOffers(context.Background(), key, offers)
		assert.Error(t, err)
	})

	t.Run("Redis Error", func(t *testing.T) {
		mockMarsh.On("Marshal", offers).Return([]byte("{}"), nil).Once()
		mockRedis.On("Set", mock.Anything, key, []byte("{}"), time.Hour).Return("", assert.AnError).Once()
		err := cache.SetOffers(context.Background(), key, offers)
		assert.Error(t, err)
	})
}

func TestNewSKUCache(t *testing.T) {
	c := NewSKUCache(nil)
	assert.NotNil(t, c)
	assert.Nil(t, c.redis)
	
	client := redis.NewClient(&redis.Options{})
	c2 := NewSKUCache(client)
	assert.NotNil(t, c2)
	assert.NotNil(t, c2.redis)
}

func TestRealRedisClient_Pipeline(t *testing.T) {
	rc1 := &realRedisClient{Client: nil}
	assert.Nil(t, rc1.Pipeline())

	client := redis.NewClient(&redis.Options{})
	rc2 := &realRedisClient{Client: client}
	assert.NotNil(t, rc2.Pipeline())
}

func TestJsonMarshaller(t *testing.T) {
	m := &jsonMarshaller{}
	data, err := m.Marshal(map[string]string{"a": "b"})
	assert.NoError(t, err)
	assert.Contains(t, string(data), "a")

	var v map[string]string
	err = m.Unmarshal(data, &v)
	assert.NoError(t, err)
	assert.Equal(t, "b", v["a"])

	err = m.Unmarshal([]byte("invalid"), &v)
	assert.Error(t, err)
}
