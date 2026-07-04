package db

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

type CircuitBreakerHook struct {
	mu       sync.Mutex
	failures int
	openAt   time.Time
}

func (h *CircuitBreakerHook) DialHook(next redis.DialHook) redis.DialHook {
	return next
}

func (h *CircuitBreakerHook) check() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.failures >= 5 {
		if time.Since(h.openAt) < 30*time.Second {
			return fmt.Errorf("context: redis circuit breaker is open")
		}
		h.failures = 0
	}
	return nil
}

func (h *CircuitBreakerHook) record(err error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	if err != nil && err != redis.Nil {
		h.failures++
		if h.failures == 5 {
			h.openAt = time.Now()
		}
	} else {
		h.failures = 0
	}
}

func (h *CircuitBreakerHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		if err := h.check(); err != nil {
			cmd.SetErr(err)
			return err
		}
		err := next(ctx, cmd)
		h.record(err)
		return err
	}
}

func (h *CircuitBreakerHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		if err := h.check(); err != nil {
			for _, cmd := range cmds {
				cmd.SetErr(err)
			}
			return err
		}
		err := next(ctx, cmds)
		h.record(err)
		return err
	}
}

// NewRedisClient creates a new single-node Redis client with circuit breaker.
func NewRedisClient(addr, password string, db int) *redis.Client {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
	client.AddHook(&CircuitBreakerHook{})
	return client
}

// NewRedisCluster creates a new Redis Cluster client with circuit breaker.
func NewRedisCluster(addrs []string, password string) *redis.ClusterClient {
	client := redis.NewClusterClient(&redis.ClusterOptions{
		Addrs:    addrs,
		Password: password,
	})
	client.AddHook(&CircuitBreakerHook{})
	return client
}
