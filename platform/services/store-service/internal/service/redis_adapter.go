package service

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisAdapter implements the RedisStore interface using the real go-redis client.
type RedisAdapter struct {
	rdb *redis.Client
}

// NewRedisAdapter creates a new RedisAdapter.
func NewRedisAdapter(rdb *redis.Client) *RedisAdapter {
	return &RedisAdapter{rdb: rdb}
}

func (a *RedisAdapter) Get(ctx context.Context, key string) (string, error) {
	return a.rdb.Get(ctx, key).Result()
}

func (a *RedisAdapter) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	return a.rdb.Set(ctx, key, value, ttl).Err()
}

func (a *RedisAdapter) Incr(ctx context.Context, key string) (int64, error) {
	return a.rdb.Incr(ctx, key).Result()
}

func (a *RedisAdapter) Decr(ctx context.Context, key string) (int64, error) {
	return a.rdb.Decr(ctx, key).Result()
}

func (a *RedisAdapter) Del(ctx context.Context, key string) error {
	return a.rdb.Del(ctx, key).Err()
}

func (a *RedisAdapter) Exists(ctx context.Context, key string) (bool, error) {
	n, err := a.rdb.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}
