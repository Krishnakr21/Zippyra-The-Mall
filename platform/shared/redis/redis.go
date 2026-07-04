package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RunLuaScript executes a custom Redis Lua script with keys and args.
func RunLuaScript(ctx context.Context, client redis.Cmdable, script string, keys []string, args ...interface{}) (interface{}, error) {
	s := redis.NewScript(script)
	res, err := s.Run(ctx, client, keys, args...).Result()
	if err != nil {
		return nil, fmt.Errorf("context: failed to run lua script: %w", err)
	}
	return res, nil
}

// AcquireLock uses SET NX to acquire a distributed lock.
func AcquireLock(ctx context.Context, client redis.Cmdable, key string, ttl time.Duration) (bool, error) {
	acquired, err := client.SetNX(ctx, key, "locked", ttl).Result()
	if err != nil {
		return false, fmt.Errorf("context: failed to acquire lock: %w", err)
	}
	return acquired, nil
}

// CheckRateLimit uses a Lua scripting sliding window rate limiter.
func CheckRateLimit(ctx context.Context, client redis.Cmdable, key string, limit int, window time.Duration) (bool, error) {
	script := `
		local current = redis.call("INCR", KEYS[1])
		if tonumber(current) == 1 then
			redis.call("PEXPIRE", KEYS[1], ARGV[1])
		end
		if tonumber(current) > tonumber(ARGV[2]) then
			return 0
		end
		return 1
	`
	windowMs := window.Milliseconds()
	res, err := RunLuaScript(ctx, client, script, []string{key}, windowMs, limit)
	if err != nil {
		return false, err
	}
	
	allowed, ok := res.(int64)
	if !ok {
		return false, fmt.Errorf("context: unexpected lua script result type")
	}
	return allowed == 1, nil
}

// SetCartItem sets a cart item in a Redis hash.
func SetCartItem(ctx context.Context, client redis.Cmdable, cartID, itemID string, quantity int) error {
	err := client.HSet(ctx, cartID, itemID, quantity).Err()
	if err != nil {
		return fmt.Errorf("context: failed to set cart item: %w", err)
	}
	return nil
}

// GetCart retrieves all items in a cart hash.
func GetCart(ctx context.Context, client redis.Cmdable, cartID string) (map[string]string, error) {
	cart, err := client.HGetAll(ctx, cartID).Result()
	if err != nil {
		return nil, fmt.Errorf("context: failed to get cart: %w", err)
	}
	return cart, nil
}

// DeleteCart removes the active cart.
func DeleteCart(ctx context.Context, client redis.Cmdable, cartID string) error {
	err := client.Del(ctx, cartID).Err()
	if err != nil {
		return fmt.Errorf("context: failed to delete cart: %w", err)
	}
	return nil
}
