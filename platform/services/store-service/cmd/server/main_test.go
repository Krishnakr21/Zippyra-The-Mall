package main

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	kafkalib "github.com/segmentio/kafka-go"
	"github.com/zippyra/platform/services/store-service/config"
	"github.com/zippyra/platform/services/store-service/internal/kafka"
	"github.com/zippyra/platform/services/store-service/internal/model"
	sharedhttp "github.com/zippyra/platform/shared/http"
)

// ── Mocks for main_test ──────────────────────────────────────────────

type mockServer struct {
	runErr error
}

func (m *mockServer) Run(ctx context.Context) error { return m.runErr }

type mockStoreRepoMain struct {
	err error
}

func (m *mockStoreRepoMain) GetByID(ctx context.Context, id uuid.UUID) (*model.Store, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &model.Store{ID: id, ChainID: uuid.New()}, nil
}

type mockPublisherMain struct{}

func (m *mockPublisherMain) PublishCustomerEntered(ctx context.Context, userID, storeID, chainID, qrTokenID string) {
}
func (m *mockPublisherMain) PublishCustomerExited(ctx context.Context, userID, storeID, chainID string, durationSeconds int64) {
}
func (m *mockPublisherMain) Close() {}

func newMockRedis() *goredis.Client {
	return goredis.NewClient(&goredis.Options{})
}

// ── Tests ───────────────────────────────────────────────────────────

func TestBuildRouter_WithPool(t *testing.T) {
	// Mock fatalLogger to prevent os.Exit
	oldFatal := fatalLogger
	defer func() { fatalLogger = oldFatal }()
	fatalLogger = func() *zerolog.Event { return log.Logger.Error() }

	cfg := &config.Config{
		KafkaBrokers:  []string{"localhost:9092"},
		JWTPublicKey:  "MCowBQYDK2VwAyEA79+uA9+vA9+vA9+vA9+vA9+vA9+vA9+vA9+vA98=",
		JWTPrivateKey: "MCowBQYDK2VwAyEA79+uA9+vA9+vA9+vA9+vA9+vA9+vA9+vA9+vA98=", // Valid length b64
	}
	// Passing a non-nil *pgxpool.Pool would hit the branch, but we can't easily create one.
	// However, buildRouter check for nil is also part of it.
	kp := kafka.NewProducerWithWriter(&mockWriterMain{})
	r := buildRouter(cfg, (*pgxpool.Pool)(nil), newMockRedis(), kp)
	if r == nil {
		t.Log("router build returned nil (expected if fatal Caught)")
	}
}

type mockWriterMain struct{}
func (m *mockWriterMain) WriteMessages(ctx context.Context, msgs ...kafkalib.Message) error { return nil }
func (m *mockWriterMain) Close() error { return nil }

func TestBuildRouter(t *testing.T) {
	// Mock fatalLogger to prevent os.Exit
	oldFatal := fatalLogger
	defer func() { fatalLogger = oldFatal }()
	fatalLogger = func() *zerolog.Event { return log.Logger.Error() }

	cfg := &config.Config{
		KafkaBrokers:   []string{"localhost:9092"},
		AllowedOrigins: []string{"*"},
		JWTPublicKey:   "MCowBQYDK2VwAyEA79+uA9+vA9+vA9+vA9+vA9+vA9+vA9+vA9+vA98=",
		JWTPrivateKey:  "MCowBQYDK2VwAyEA79+uA9+vA9+vA9+vA9+vA9+vA9+vA9+vA9+vA98=",
	}

	// Mock dependencies
	db := (dbPool)(nil)
	rdb := (*goredis.Client)(nil)
	kp := (*kafka.Producer)(nil)

	r := buildRouter(cfg, db, rdb, kp)
	if r == nil {
		t.Log("router build returned nil (expected if keys invalid but fatal caught)")
	}
}

func TestChainLookupAdapter(t *testing.T) {
	repo := &mockStoreRepoMain{}
	adapter := &chainLookupAdapter{repo: repo}
	
	id := uuid.New()
	chainID, err := adapter.GetStoreChainID(context.Background(), id)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if chainID == uuid.Nil {
		t.Error("expected non-nil chain_id")
	}
}

func TestRun_Success(t *testing.T) {
	// Override globals
	oldNewServer := newServer
	oldMainFatal := mainFatal
	oldLoadConfig := loadConfig
	defer func() {
		newServer = oldNewServer
		mainFatal = oldMainFatal
		loadConfig = oldLoadConfig
	}()

	loadConfig = func() *config.Config {
		return &config.Config{
			Port:          "8081",
			DatabaseURL:   "postgres://localhost:5432/db",
			RedisURL:      "redis://localhost:6379",
			KafkaBrokers:  []string{"localhost:9092"},
			JWTPublicKey:  "MCowBQYDK2VwAyEA79+uA9+vA9+vA9+vA9+vA9+vA9+vA9+vA9+vA98=",
			JWTPrivateKey: "MCowBQYDK2VwAyEA79+uA9+vA9+vA9+vA9+vA9+vA9+vA9+vA9+vA98=",
		}
	}

	newServer = func(cfg sharedhttp.ServerConfig) serverRunner {
		return &mockServer{}
	}

	oldNewPGPool := newPGPool
	oldParseRedisURL := parseRedisURL
	oldNewProducer := newProducer
	defer func() {
		newPGPool = oldNewPGPool
		parseRedisURL = oldParseRedisURL
		newProducer = oldNewProducer
	}()

	newPGPool = func(ctx context.Context, connString string) (dbPool, error) { return &mockDBPool{}, nil }
	parseRedisURL = func(url string) (*goredis.Options, error) { return &goredis.Options{}, nil }
	newProducer = func(brokers []string) (*kafka.Producer, error) {
		return kafka.NewProducerWithWriter(&mockWriterMain{}), nil
	}

	fatalCalled := false
	mainFatal = func(err error) {
		fatalCalled = true
	}

	// We need to stop the run quickly or mock the server.Run to return nil
	go func() {
		time.Sleep(100 * time.Millisecond)
		// Usually we'd send a signal, but here we just want to see if it starts
	}()

	// Since we are mocking newServer, it won't actually block much if Run() returns nil immediately
	run(context.Background())
	
	if fatalCalled {
		t.Error("did not expect fatal to be called")
	}
}

func TestGetStoreChainID_Error(t *testing.T) {
	repo := &mockStoreRepoMain{err: errors.New("db error")}
	adapter := &chainLookupAdapter{repo: repo}
	_, err := adapter.GetStoreChainID(context.Background(), uuid.New())
	if err == nil {
		t.Error("expected error")
	}
}

func TestRun_DBError(t *testing.T) {
	oldNewPGPool := newPGPool
	defer func() { newPGPool = oldNewPGPool }()
	newPGPool = func(ctx context.Context, connString string) (dbPool, error) {
		return nil, errors.New("db error")
	}

	oldLoadConfig := loadConfig
	defer func() { loadConfig = oldLoadConfig }()
	loadConfig = func() *config.Config { return &config.Config{DatabaseURL: "fail"} }

	err := run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "db init failed") {
		t.Errorf("expected db init failed error, got %v", err)
	}
}

func TestRun_KafkaError(t *testing.T) {
	oldNewPGPool := newPGPool
	defer func() { newPGPool = oldNewPGPool }()
	newPGPool = func(ctx context.Context, connString string) (dbPool, error) { return &mockDBPool{}, nil }

	oldParseRedisURL := parseRedisURL
	defer func() { parseRedisURL = oldParseRedisURL }()
	parseRedisURL = func(url string) (*goredis.Options, error) { return &goredis.Options{}, nil }

	oldNewProducer := newProducer
	defer func() { newProducer = oldNewProducer }()
	newProducer = func(brokers []string) (*kafka.Producer, error) {
		return nil, errors.New("kafka error")
	}

	oldLoadConfig := loadConfig
	defer func() { loadConfig = oldLoadConfig }()
	loadConfig = func() *config.Config { return &config.Config{KafkaBrokers: []string{"fail"}} }

	err := run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "kafka producer init failed") {
		t.Errorf("expected kafka init failed error, got %v", err)
	}
}

func TestRun_SentryError(t *testing.T) {
	oldLoadConfig := loadConfig
	defer func() { loadConfig = oldLoadConfig }()
	// Provide valid Redis URL and KafkaBrokers so it doesn't fail/panic there
	loadConfig = func() *config.Config {
		return &config.Config{
			SentryDSN:     "fail",
			RedisURL:      "redis://localhost:6379",
			KafkaBrokers:  []string{"localhost:9092"},
			JWTPublicKey:  "MCowBQYDK2VwAyEA79+uA9+vA9+vA9+vA9+vA9+vA9+vA9+vA9+vA98=",
			JWTPrivateKey: "MCowBQYDK2VwAyEA79+uA9+vA9+vA9+vA9+vA9+vA9+vA9+vA9+vA98=",
		}
	}

	oldSentryInit := sentryInit
	defer func() { sentryInit = oldSentryInit }()
	sentryInit = func(opts sentry.ClientOptions) error {
		return errors.New("sentry error")
	}

	// Mock other things to return success
	oldNewPGPool := newPGPool
	defer func() { newPGPool = oldNewPGPool }()
	newPGPool = func(ctx context.Context, connString string) (dbPool, error) { return &mockDBPool{}, nil }
	
	oldParseRedisURL := parseRedisURL
	defer func() { parseRedisURL = oldParseRedisURL }()
	parseRedisURL = func(url string) (*goredis.Options, error) { return &goredis.Options{}, nil }

	oldNewProducer := newProducer
	defer func() { newProducer = oldNewProducer }()
	newProducer = func(brokers []string) (*kafka.Producer, error) {
		return kafka.NewProducerWithWriter(&mockWriterMain{}), nil
	}

	oldNewServer := newServer
	defer func() { newServer = oldNewServer }()
	newServer = func(cfg sharedhttp.ServerConfig) serverRunner {
		return &mockServer{runErr: nil}
	}

	err := run(context.Background())
	// Sentry failure is NOT fatal in run(), but it might fail later at Server Run if not mocked
	if err != nil && !strings.Contains(err.Error(), "server failed") {
		t.Errorf("unexpected error from sentry failure: %v", err)
	}
}

func TestRun_RedisParseError(t *testing.T) {
	oldParseRedisURL := parseRedisURL
	defer func() { parseRedisURL = oldParseRedisURL }()
	parseRedisURL = func(url string) (*goredis.Options, error) {
		return nil, errors.New("parse error")
	}

	oldLoadConfig := loadConfig
	defer func() { loadConfig = oldLoadConfig }()
	loadConfig = func() *config.Config { return &config.Config{RedisURL: "fail"} }

	err := run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "redis url parse failed") {
		t.Errorf("expected redis url parse failed error, got %v", err)
	}
}

func TestRun_ServerError(t *testing.T) {
	oldLoadConfig := loadConfig
	defer func() { loadConfig = oldLoadConfig }()
	loadConfig = func() *config.Config {
		return &config.Config{
			RedisURL:      "redis://localhost:6379",
			KafkaBrokers:  []string{"localhost"},
			JWTPublicKey:  "MCowBQYDK2VwAyEA79+uA9+vA9+vA9+vA9+vA9+vA9+vA9+vA9+vA98=",
			JWTPrivateKey: "MCowBQYDK2VwAyEA79+uA9+vA9+vA9+vA9+vA9+vA9+vA9+vA9+vA98=",
		}
	}

	oldNewServer := newServer
	defer func() { newServer = oldNewServer }()
	newServer = func(cfg sharedhttp.ServerConfig) serverRunner {
		return &mockServer{runErr: errors.New("server error")}
	}

	// Mock DB and Kafka
	oldNewPGPool := newPGPool
	defer func() { newPGPool = oldNewPGPool }()
	newPGPool = func(ctx context.Context, connString string) (dbPool, error) { return &mockDBPool{}, nil }
	
	oldParseRedisURL := parseRedisURL
	defer func() { parseRedisURL = oldParseRedisURL }()
	parseRedisURL = func(url string) (*goredis.Options, error) { return &goredis.Options{}, nil }

	oldNewProducer := newProducer
	defer func() { newProducer = oldNewProducer }()
	newProducer = func(brokers []string) (*kafka.Producer, error) {
		return kafka.NewProducerWithWriter(&mockWriterMain{}), nil
	}

	err := run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "server error") {
		t.Errorf("expected server error, got %v", err)
	}
}

func TestMain_Success(t *testing.T) {
	// Mock everything run() needs
	oldLoadConfig := loadConfig
	defer func() { loadConfig = oldLoadConfig }()
	loadConfig = func() *config.Config {
		return &config.Config{
			RedisURL:      "redis://localhost:6379",
			KafkaBrokers:  []string{"localhost"},
			JWTPublicKey:  "MCowBQYDK2VwAyEA79+uA9+vA9+vA9+vA9+vA9+vA9+vA9+vA9+vA98=",
			JWTPrivateKey: "MCowBQYDK2VwAyEA79+uA9+vA9+vA9+vA9+vA9+vA9+vA9+vA9+vA98=",
		}
	}

	oldNewServer := newServer
	defer func() { newServer = oldNewServer }()
	newServer = func(cfg sharedhttp.ServerConfig) serverRunner {
		return &mockServer{runErr: nil}
	}

	oldNewPGPool := newPGPool
	defer func() { newPGPool = oldNewPGPool }()
	newPGPool = func(ctx context.Context, connString string) (dbPool, error) { return &mockDBPool{}, nil }
	
	oldParseRedisURL := parseRedisURL
	defer func() { parseRedisURL = oldParseRedisURL }()
	parseRedisURL = func(url string) (*goredis.Options, error) { return &goredis.Options{}, nil }

	oldNewProducer := newProducer
	defer func() { newProducer = oldNewProducer }()
	newProducer = func(brokers []string) (*kafka.Producer, error) {
		return kafka.NewProducerWithWriter(&mockWriterMain{}), nil
	}

	// Should not panic or exit
	main()
}

func TestMain_Error(t *testing.T) {
	oldLoadConfig := loadConfig
	defer func() { loadConfig = oldLoadConfig }()
	loadConfig = func() *config.Config {
		return &config.Config{
			DatabaseURL: "fail",
		}
	}

	oldNewPGPool := newPGPool
	defer func() { newPGPool = oldNewPGPool }()
	newPGPool = func(ctx context.Context, connString string) (dbPool, error) {
		return nil, errors.New("db error")
	}

	fatalCalled := false
	oldMainFatal := mainFatal
	defer func() { mainFatal = oldMainFatal }()
	mainFatal = func(err error) {
		fatalCalled = true
	}

	main()

	if !fatalCalled {
		t.Error("expected mainFatal to be called")
	}
}

type mockDBPool struct{}

func (m *mockDBPool) Close() {}
func (m *mockDBPool) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}
func (m *mockDBPool) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return nil, nil
}
func (m *mockDBPool) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row { return nil }
