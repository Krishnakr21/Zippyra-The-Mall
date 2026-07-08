package service

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/zippyra/platform/services/payment-service/internal/kafka"
	"github.com/zippyra/platform/services/payment-service/internal/repository"
	"github.com/zippyra/platform/services/payment-service/internal/testutil"
)

func newTestRedis(t *testing.T) (*redis.Client, func()) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "zippyra_local",
		DB:       2,
	})
	ctx := context.Background()
	err := rdb.Ping(ctx).Err()
	assert.NoError(t, err, "failed to connect/ping real redis")
	cleanup := func() {
		_ = rdb.FlushDB(ctx)
		_ = rdb.Close()
	}
	return rdb, cleanup
}

func TestOutboxRelay_Poll_PublishesUnpublished(t *testing.T) {
	pool, dbCleanup := testutil.SetupDB()
	defer dbCleanup()

	_, redisCleanup := newTestRedis(t)
	defer redisCleanup()

	ctx := context.Background()
	outboxRepo := repository.NewOutboxRepository(pool)

	// Setup: insert 3 unpublished rows
	tx, err := pool.Begin(ctx)
	assert.NoError(t, err)
	err = outboxRepo.Create(ctx, tx, "topic1", []byte(`{"val":"payload1"}`))
	assert.NoError(t, err)
	err = outboxRepo.Create(ctx, tx, "topic2", []byte(`{"val":"payload2"}`))
	assert.NoError(t, err)
	err = outboxRepo.Create(ctx, tx, "topic3", []byte(`{"val":"payload3"}`))
	assert.NoError(t, err)
	err = tx.Commit(ctx)
	assert.NoError(t, err)

	// Mock Kafka to record published messages
	var lock sync.Mutex
	publishedMsgs := make(map[string][]byte)
	mockProducer := kafka.NewProducer(nil)
	mockProducer.PublishFn = func(ctx context.Context, topic, key string, value interface{}) error {
		lock.Lock()
		defer lock.Unlock()
		publishedMsgs[topic] = value.([]byte)
		return nil
	}

	relay := NewOutboxRelay(pool, mockProducer)

	// Run poll()
	err = relay.poll(ctx)
	assert.NoError(t, err)

	// Assert: all 3 published to mock Kafka
	lock.Lock()
	assert.Len(t, publishedMsgs, 3)
	assert.Equal(t, []byte(`{"val": "payload1"}`), publishedMsgs["topic1"])
	assert.Equal(t, []byte(`{"val": "payload2"}`), publishedMsgs["topic2"])
	assert.Equal(t, []byte(`{"val": "payload3"}`), publishedMsgs["topic3"])
	lock.Unlock()

	// Assert: all 3 marked published_at IS NOT NULL in DB
	msgs, err := outboxRepo.GetUnpublished(ctx, 10)
	assert.NoError(t, err)
	assert.Len(t, msgs, 0)
}

func TestOutboxRelay_Poll_KafkaFail_RowStaysUnpublished(t *testing.T) {
	pool, dbCleanup := testutil.SetupDB()
	defer dbCleanup()

	_, redisCleanup := newTestRedis(t)
	defer redisCleanup()

	ctx := context.Background()
	outboxRepo := repository.NewOutboxRepository(pool)

	// Setup: insert 1 unpublished row
	tx, err := pool.Begin(ctx)
	assert.NoError(t, err)
	err = outboxRepo.Create(ctx, tx, "fail-topic", []byte(`{"val":"payload-fail"}`))
	assert.NoError(t, err)
	err = tx.Commit(ctx)
	assert.NoError(t, err)

	// Mock Kafka to fail
	mockProducer := kafka.NewProducer(nil)
	mockProducer.PublishFn = func(ctx context.Context, topic, key string, value interface{}) error {
		return errors.New("kafka error")
	}

	relay := NewOutboxRelay(pool, mockProducer)

	// Run poll()
	err = relay.poll(ctx)
	assert.NoError(t, err)

	// Assert: row still has published_at IS NULL
	msgs, err := outboxRepo.GetUnpublished(ctx, 10)
	assert.NoError(t, err)
	assert.Len(t, msgs, 1)
	assert.Equal(t, "fail-topic", msgs[0].Topic)
}

func TestOutboxRelay_Poll_EmptyOutbox_NoOp(t *testing.T) {
	pool, dbCleanup := testutil.SetupDB()
	defer dbCleanup()

	_, redisCleanup := newTestRedis(t)
	defer redisCleanup()

	ctx := context.Background()

	kafkaCalled := false
	mockProducer := kafka.NewProducer(nil)
	mockProducer.PublishFn = func(ctx context.Context, topic, key string, value interface{}) error {
		kafkaCalled = true
		return nil
	}

	relay := NewOutboxRelay(pool, mockProducer)

	// Run poll() with empty outbox
	err := relay.poll(ctx)
	assert.NoError(t, err)
	assert.False(t, kafkaCalled)
}
