package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"

	"github.com/zippyra/platform/services/catalog-service/internal/model"
)
type Pipeliner interface {
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd
	Exec(ctx context.Context) ([]redis.Cmder, error)
}

type RedisClient interface {
	Get(ctx context.Context, key string) *redis.StringCmd
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd
	Del(ctx context.Context, keys ...string) *redis.IntCmd
	Pipeline() Pipeliner
}

type realRedisClient struct {
	*redis.Client
}

func (c *realRedisClient) Pipeline() Pipeliner {
	if c.Client == nil {
		return nil
	}
	return c.Client.Pipeline()
}

type Marshaller interface {
	Marshal(v interface{}) ([]byte, error)
	Unmarshal(data []byte, v interface{}) error
}

type jsonMarshaller struct{}

func (j *jsonMarshaller) Marshal(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

func (j *jsonMarshaller) Unmarshal(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

type SKUCache struct {
	redis      RedisClient
	marshaller Marshaller
	ttl        time.Duration
}

func NewSKUCache(client *redis.Client) *SKUCache {
	var redisClient RedisClient
	if client != nil {
		redisClient = &realRedisClient{client}
	}
	return &SKUCache{
		redis:      redisClient,
		marshaller: &jsonMarshaller{},
		ttl:        24 * time.Hour,
	}
}

func (c *SKUCache) Get(ctx context.Context, storeID, barcode string) (*model.Product, error) {
	key := fmt.Sprintf("sku:%s:%s", storeID, barcode)

	ctx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()

	val, err := c.redis.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		log.Error().Err(err).Str("key", key).Msg("failed to get product from cache")
		return nil, err
	}

	var product model.Product
	if err := c.marshaller.Unmarshal([]byte(val), &product); err != nil {
		log.Error().Err(err).Str("key", key).Msg("failed to unmarshal product from cache")
		return nil, err
	}

	return &product, nil
}

func (c *SKUCache) Set(ctx context.Context, storeID, barcode string, p *model.Product) error {
	key := fmt.Sprintf("sku:%s:%s", storeID, barcode)

	data, err := c.marshaller.Marshal(p)
	if err != nil {
		log.Error().Err(err).Str("key", key).Msg("failed to marshal product for cache")
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()

	if err := c.redis.Set(ctx, key, data, c.ttl).Err(); err != nil {
		log.Error().Err(err).Str("key", key).Msg("failed to set product in cache")
		return err
	}

	return nil
}

func (c *SKUCache) Invalidate(ctx context.Context, storeID, barcode string) error {
	key := fmt.Sprintf("sku:%s:%s", storeID, barcode)

	ctx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()

	if err := c.redis.Del(ctx, key).Err(); err != nil {
		log.Error().Err(err).Str("key", key).Msg("failed to invalidate product from cache")
		return err
	}

	return nil
}

func (c *SKUCache) WarmStore(ctx context.Context, storeID string, products []model.Product) error {
	if len(products) == 0 {
		return nil
	}

	pipe := c.redis.Pipeline()

	for _, product := range products {
		key := fmt.Sprintf("sku:%s:%s", storeID, product.Barcode)
		data, err := c.marshaller.Marshal(product)
		if err != nil {
			log.Error().Err(err).Str("key", key).Msg("failed to marshal product for cache warming")
			continue
		}
		pipe.Set(ctx, key, data, c.ttl)
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if _, err := pipe.Exec(ctx); err != nil {
		log.Error().Err(err).Str("storeID", storeID).Int("count", len(products)).Msg("failed to warm store cache")
		return err
	}

	log.Info().Str("storeID", storeID).Int("count", len(products)).Msg("SKU cache warmed for store")
	return nil
}

func (c *SKUCache) GetOffers(ctx context.Context, key string) (*model.OfferResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()

	val, err := c.redis.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		log.Error().Err(err).Str("key", key).Msg("failed to get offers from cache")
		return nil, err
	}

	var offers model.OfferResponse
	if err := c.marshaller.Unmarshal([]byte(val), &offers); err != nil {
		log.Error().Err(err).Str("key", key).Msg("failed to unmarshal offers from cache")
		return nil, err
	}

	return &offers, nil
}

func (c *SKUCache) SetOffers(ctx context.Context, key string, offers *model.OfferResponse) error {
	data, err := c.marshaller.Marshal(offers)
	if err != nil {
		log.Error().Err(err).Str("key", key).Msg("failed to marshal offers for cache")
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()

	if err := c.redis.Set(ctx, key, data, time.Hour).Err(); err != nil {
		log.Error().Err(err).Str("key", key).Msg("failed to set offers in cache")
		return err
	}

	return nil
}
