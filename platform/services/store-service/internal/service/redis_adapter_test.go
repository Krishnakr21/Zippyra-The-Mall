package service

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func setupTestRedis(t *testing.T) (*RedisAdapter, *miniredis.Miniredis) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	adapter := NewRedisAdapter(rdb)
	return adapter, mr
}

func TestRedisAdapter_Get(t *testing.T) {
	adapter, mr := setupTestRedis(t)
	defer mr.Close()

	ctx := context.Background()

	// Test Get on existing key
	mr.Set("testkey", "testvalue")
	val, err := adapter.Get(ctx, "testkey")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "testvalue" {
		t.Errorf("expected 'testvalue', got '%s'", val)
	}

	// Test Get on non-existing key
	_, err = adapter.Get(ctx, "nonexistent")
	if err == nil {
		t.Error("expected error for non-existent key")
	}
}

func TestRedisAdapter_Set(t *testing.T) {
	adapter, mr := setupTestRedis(t)
	defer mr.Close()

	ctx := context.Background()

	err := adapter.Set(ctx, "testkey", "testvalue", time.Minute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	val, err := adapter.Get(ctx, "testkey")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "testvalue" {
		t.Errorf("expected 'testvalue', got '%s'", val)
	}
}

func TestRedisAdapter_Incr(t *testing.T) {
	adapter, mr := setupTestRedis(t)
	defer mr.Close()

	ctx := context.Background()

	// Increment new key
	val, err := adapter.Incr(ctx, "counter")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != 1 {
		t.Errorf("expected 1, got %d", val)
	}

	// Increment existing key
	val, err = adapter.Incr(ctx, "counter")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != 2 {
		t.Errorf("expected 2, got %d", val)
	}
}

func TestRedisAdapter_Decr(t *testing.T) {
	adapter, mr := setupTestRedis(t)
	defer mr.Close()

	ctx := context.Background()

	// Set initial value
	mr.Set("counter", "5")

	val, err := adapter.Decr(ctx, "counter")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != 4 {
		t.Errorf("expected 4, got %d", val)
	}
}

func TestRedisAdapter_Del(t *testing.T) {
	adapter, mr := setupTestRedis(t)
	defer mr.Close()

	ctx := context.Background()

	mr.Set("testkey", "testvalue")

	err := adapter.Del(ctx, "testkey")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify key is deleted
	if mr.Exists("testkey") {
		t.Error("expected key to be deleted")
	}
}

func TestRedisAdapter_Exists(t *testing.T) {
	adapter, mr := setupTestRedis(t)
	defer mr.Close()

	ctx := context.Background()

	// Test non-existing key
	exists, err := adapter.Exists(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exists {
		t.Error("expected false for non-existent key")
	}

	// Test existing key
	mr.Set("testkey", "testvalue")
	exists, err = adapter.Exists(ctx, "testkey")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !exists {
		t.Error("expected true for existing key")
	}
}

func TestRedisAdapter_Exists_Error(t *testing.T) {
	// Create adapter with a closed server to trigger error
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	adapter := NewRedisAdapter(rdb)

	// Close the server to trigger connection error
	mr.Close()

	ctx := context.Background()
	// This should return error due to closed connection
	_, err := adapter.Exists(ctx, "testkey")
	if err == nil {
		t.Error("expected error for closed connection")
	}
}

func TestNewRedisAdapter(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	adapter := NewRedisAdapter(rdb)

	if adapter == nil {
		t.Error("expected adapter to not be nil")
	}
	if adapter.rdb != rdb {
		t.Error("expected rdb to be set correctly")
	}
}
