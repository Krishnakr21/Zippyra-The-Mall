package main

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	kafkalib "github.com/segmentio/kafka-go"

	"github.com/zippyra/platform/services/auth-service/config"
	"github.com/zippyra/platform/services/auth-service/internal/kafka"
	sharedhttp "github.com/zippyra/platform/shared/http"
)

type fakeServer struct {
	err error
}

func TestRun_SentryInitErrorBranch(t *testing.T) {
	old := snapshotMainDeps()
	defer old.restore()

	pub, priv, _ := ed25519.GenerateKey(nil)
	loadConfig = func() *config.Config {
		return &config.Config{
			Port:          "8080",
			DatabaseURL:   "x",
			RedisURL:      "redis://localhost:6379/0",
			KafkaBrokers:  []string{""},
			JWTPublicKey:  base64.StdEncoding.EncodeToString(pub),
			JWTPrivateKey: base64.StdEncoding.EncodeToString(priv),
			OTPSalt:       "test-salt-that-is-at-least-32-characters-long",
			AppEnv:        "local",
			Version:       "1.0.0",
			SentryDSN:     "dsn",
		}
	}
	newPGPool = func(ctx context.Context, connString string) (dbPool, error) { return fakeDB{}, nil }
	parseRedisURL = func(redisURL string) (*goredis.Options, error) { return &goredis.Options{}, nil }
	newRedis = func(opt *goredis.Options) *goredis.Client {
		return goredis.NewClient(&goredis.Options{Addr: "localhost:0"})
	}
	newProducer = func(brokers []string, topic string) *kafka.Producer {
		return kafka.NewProducerWithWriter(nopKafkaWriter{})
	}
	newServer = func(cfg sharedhttp.ServerConfig) serverRunner { return fakeServer{err: nil} }

	initCalled := false
	initSentry = func(opts sentry.ClientOptions) error { initCalled = true; return errors.New("init") }
	flushCalled := false
	flushSentry = func(d time.Duration) bool { flushCalled = true; return true }

	if err := run(context.Background()); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if !initCalled || !flushCalled {
		t.Fatal("expected init+flush")
	}
}

func TestDefaultClosures_Coverage(t *testing.T) {
	oldPoolNew := pgPoolNew
	oldServerNew := serverNew
	oldFatal := fatalLogger
	defer func() {
		pgPoolNew = oldPoolNew
		serverNew = oldServerNew
		fatalLogger = oldFatal
	}()

	pgPoolNew = func(ctx context.Context, connString string) (*pgxpool.Pool, error) { return &pgxpool.Pool{}, nil }
	_, _ = newPGPool(context.Background(), "x")

	serverNew = func(cfg sharedhttp.ServerConfig) *sharedhttp.Server { return sharedhttp.New(cfg) }
	_ = newServer(sharedhttp.ServerConfig{Addr: ":0", Handler: http.NewServeMux(), ServiceName: "t"})

	fatalCalled := false
	fatalLogger = func() *zerolog.Event { fatalCalled = true; return log.Error() }
	mainFatal(errors.New("x"))
	if !fatalCalled {
		t.Fatal("expected fatal logger")
	}
}

func TestMain_CallsMainFatalOnError(t *testing.T) {
	old := snapshotMainDeps()
	defer old.restore()

	called := false
	mainFatal = func(err error) { called = true }
	loadConfig = func() *config.Config { return &config.Config{DatabaseURL: "x"} }
	newPGPool = func(ctx context.Context, connString string) (dbPool, error) { return nil, errors.New("db") }

	main()
	if !called {
		t.Fatal("expected mainFatal to be called")
	}
}
func TestBuildRouter_HealthDBTypeAssertBranch(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	cfg := &config.Config{
		Port:           "8080",
		Version:        "1.0.0",
		KafkaBrokers:   []string{""},
		JWTPublicKey:   base64.StdEncoding.EncodeToString(pub),
		JWTPrivateKey:  base64.StdEncoding.EncodeToString(priv),
		OTPSalt:        "test-salt-that-is-at-least-32-characters-long",
		AllowedOrigins: []string{"*"},
		AppEnv:         "local",
	}

	// db is a *pgxpool.Pool (nil pointer) so type assertion branch is executed.
	r := buildRouter(cfg, (*pgxpool.Pool)(nil), (*goredis.Client)(nil), kafka.NewProducerWithWriter(nopKafkaWriter{}))
	req := httptest.NewRequest(http.MethodGet, "/healthz/live", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func (f fakeServer) Run(ctx context.Context) error { return f.err }

type fakeDB struct{}

func (f fakeDB) Close() {}
func (f fakeDB) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}
func (f fakeDB) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return nil, errors.New("not implemented")
}
func (f fakeDB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return nil
}

type nopKafkaWriter struct{}

func (n nopKafkaWriter) WriteMessages(ctx context.Context, msgs ...kafkalib.Message) error {
	return nil
}
func (n nopKafkaWriter) Close() error { return nil }

func TestBuildRouter_HealthLive(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	cfg := &config.Config{
		Port:           "8080",
		Version:        "1.0.0",
		KafkaBrokers:   []string{""},
		JWTPublicKey:   base64.StdEncoding.EncodeToString(pub),
		JWTPrivateKey:  base64.StdEncoding.EncodeToString(priv),
		OTPSalt:        "test-salt-that-is-at-least-32-characters-long",
		AllowedOrigins: []string{"*"},
		AppEnv:         "local",
	}

	r := buildRouter(cfg, nil, (*goredis.Client)(nil), kafka.NewProducerWithWriter(nopKafkaWriter{}))

	req := httptest.NewRequest(http.MethodGet, "/healthz/live", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestRun_DBInitError(t *testing.T) {
	old := snapshotMainDeps()
	defer old.restore()

	loadConfig = func() *config.Config {
		return &config.Config{DatabaseURL: "x"}
	}
	newPGPool = func(ctx context.Context, connString string) (dbPool, error) {
		return nil, errors.New("db")
	}

	err := run(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRun_RedisParseError(t *testing.T) {
	old := snapshotMainDeps()
	defer old.restore()

	loadConfig = func() *config.Config {
		return &config.Config{DatabaseURL: "x", RedisURL: "bad"}
	}
	newPGPool = func(ctx context.Context, connString string) (dbPool, error) {
		return fakeDB{}, nil
	}
	parseRedisURL = func(redisURL string) (*goredis.Options, error) {
		return nil, errors.New("parse")
	}

	err := run(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRun_ServerError(t *testing.T) {
	old := snapshotMainDeps()
	defer old.restore()

	pub, priv, _ := ed25519.GenerateKey(nil)
	loadConfig = func() *config.Config {
		return &config.Config{
			Port:          "8080",
			DatabaseURL:   "x",
			RedisURL:      "redis://localhost:6379/0",
			KafkaBrokers:  []string{""},
			JWTPublicKey:  base64.StdEncoding.EncodeToString(pub),
			JWTPrivateKey: base64.StdEncoding.EncodeToString(priv),
			OTPSalt:       "test-salt-that-is-at-least-32-characters-long",
			AppEnv:        "local",
			Version:       "1.0.0",
		}
	}
	newPGPool = func(ctx context.Context, connString string) (dbPool, error) { return fakeDB{}, nil }
	parseRedisURL = func(redisURL string) (*goredis.Options, error) { return &goredis.Options{}, nil }
	newRedis = func(opt *goredis.Options) *goredis.Client {
		return goredis.NewClient(&goredis.Options{Addr: "localhost:0"})
	}
	newProducer = func(brokers []string, topic string) *kafka.Producer {
		return kafka.NewProducerWithWriter(nopKafkaWriter{})
	}
	newServer = func(cfg sharedhttp.ServerConfig) serverRunner { return fakeServer{err: errors.New("srv")} }

	err := run(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRun_Success_WithSentry(t *testing.T) {
	old := snapshotMainDeps()
	defer old.restore()

	pub, priv, _ := ed25519.GenerateKey(nil)
	loadConfig = func() *config.Config {
		return &config.Config{
			Port:          "8080",
			DatabaseURL:   "x",
			RedisURL:      "redis://localhost:6379/0",
			KafkaBrokers:  []string{""},
			JWTPublicKey:  base64.StdEncoding.EncodeToString(pub),
			JWTPrivateKey: base64.StdEncoding.EncodeToString(priv),
			OTPSalt:       "test-salt-that-is-at-least-32-characters-long",
			AppEnv:        "local",
			Version:       "1.0.0",
			SentryDSN:     "dsn",
		}
	}
	newPGPool = func(ctx context.Context, connString string) (dbPool, error) { return fakeDB{}, nil }
	parseRedisURL = func(redisURL string) (*goredis.Options, error) { return &goredis.Options{}, nil }
	newRedis = func(opt *goredis.Options) *goredis.Client {
		return goredis.NewClient(&goredis.Options{Addr: "localhost:0"})
	}
	newProducer = func(brokers []string, topic string) *kafka.Producer {
		return kafka.NewProducerWithWriter(nopKafkaWriter{})
	}
	newServer = func(cfg sharedhttp.ServerConfig) serverRunner { return fakeServer{err: nil} }

	inited := false
	flushed := false
	initSentry = func(opts sentry.ClientOptions) error { inited = true; return nil }
	flushSentry = func(d time.Duration) bool { flushed = true; return true }

	err := run(context.Background())
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if !inited || !flushed {
		t.Fatal("expected sentry init+flush")
	}
}

type depsSnapshot struct {
	loadConfig    func() *config.Config
	initSentry    func(opts sentry.ClientOptions) error
	flushSentry   func(timeout time.Duration) bool
	newPGPool     func(ctx context.Context, connString string) (dbPool, error)
	parseRedisURL func(redisURL string) (*goredis.Options, error)
	newRedis      func(opt *goredis.Options) *goredis.Client
	newProducer   func(brokers []string, topic string) *kafka.Producer
	newServer     func(cfg sharedhttp.ServerConfig) serverRunner
	mainFatal     func(err error)
}

func snapshotMainDeps() depsSnapshot {
	return depsSnapshot{
		loadConfig:    loadConfig,
		initSentry:    initSentry,
		flushSentry:   flushSentry,
		newPGPool:     newPGPool,
		parseRedisURL: parseRedisURL,
		newRedis:      newRedis,
		newProducer:   newProducer,
		newServer:     newServer,
		mainFatal:     mainFatal,
	}
}

func (d depsSnapshot) restore() {
	loadConfig = d.loadConfig
	initSentry = d.initSentry
	flushSentry = d.flushSentry
	newPGPool = d.newPGPool
	parseRedisURL = d.parseRedisURL
	newRedis = d.newRedis
	newProducer = d.newProducer
	newServer = d.newServer
	mainFatal = d.mainFatal
}
