package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	goredis "github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"

	"github.com/zippyra/platform/services/store-service/config"
	"github.com/zippyra/platform/services/store-service/internal/handler"
	"github.com/zippyra/platform/services/store-service/internal/kafka"
	mw "github.com/zippyra/platform/services/store-service/internal/middleware"
	"github.com/zippyra/platform/services/store-service/internal/model"
	"github.com/zippyra/platform/services/store-service/internal/repository"
	"github.com/zippyra/platform/services/store-service/internal/service"
	"github.com/zippyra/platform/shared/health"
	sharedhttp "github.com/zippyra/platform/shared/http"
	sharedconfig "github.com/zippyra/platform/shared/config"
)

type serverRunner interface {
	Run(ctx context.Context) error
}

type dbPool interface {
	Close()
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// storeRepo is the interface needed by chainLookupAdapter
type storeRepo interface {
	GetByID(ctx context.Context, storeID uuid.UUID) (*model.Store, error)
}

// chainLookupAdapter adapts StoreRepository for the ChainScope middleware.
type chainLookupAdapter struct {
	repo storeRepo
}

func (a *chainLookupAdapter) GetStoreChainID(ctx context.Context, storeID uuid.UUID) (uuid.UUID, error) {
	store, err := a.repo.GetByID(ctx, storeID)
	if err != nil {
		return uuid.Nil, err
	}
	return store.ChainID, nil
}

var (
	loadConfig    = config.Load
	sentryInit    = sentry.Init
	flushSentry   = sentry.Flush
	pgPoolNew     = pgxpool.New
	newPGPoolBase = pgxpool.New
	newPGPool = func(ctx context.Context, connString string) (dbPool, error) {
		return pgxpool.New(ctx, connString)
	}
	parseRedisURL = goredis.ParseURL
	newRedis      = goredis.NewClient
	newProducer   = kafka.NewProducer
	serverNew     = sharedhttp.New
	newServerBase = sharedhttp.New
	newServer     = func(cfg sharedhttp.ServerConfig) serverRunner { return newServerBase(cfg) }
	fatalLogger   = log.Fatal
	osExit        = os.Exit
	mainFatal     = func(err error) { fatalLogger().Err(err).Msg("fatal"); osExit(1) }
)

func main() {
	sharedconfig.LoadEnv()
	if err := run(context.Background()); err != nil {
		mainFatal(err)
	}
}

func run(ctx context.Context) error {
	// 1. Load config
	cfg := loadConfig()

	// 1. Sentry
	if cfg.SentryDSN != "" {
		if err := sentryInit(sentry.ClientOptions{
			Dsn:              cfg.SentryDSN,
			Environment:      cfg.AppEnv,
			TracesSampleRate: 1.0,
		}); err != nil {
			log.Error().Err(err).Msg("sentry init failed")
		}
		defer flushSentry(2 * time.Second)
	}

	// 2. Init DB pool
	db, err := newPGPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("db init failed: %w", err)
	}
	defer db.Close()

	// 3. Init Redis client
	opts, err := parseRedisURL(cfg.RedisURL)
	if err != nil {
		return fmt.Errorf("redis url parse failed: %w", err)
	}
	rdb := newRedis(opts)
	defer rdb.Close()

	// 4. Kafka
	producer, err := newProducer(cfg.KafkaBrokers)
	if err != nil {
		return fmt.Errorf("kafka producer init failed: %w", err)
	}
	defer producer.Close()

	// 5. Build chi router with full middleware chain
	r := buildRouter(cfg, db, rdb, producer)

	// 6. Run server — blocks until SIGTERM
	srv := newServer(sharedhttp.ServerConfig{
		Addr:        ":" + cfg.Port,
		Handler:     r,
		ServiceName: "store-service",
	})
	if err := srv.Run(ctx); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server error: %w", err)
	}

	return nil
}

func buildRouter(cfg *config.Config, db dbPool, rdb *goredis.Client, producer *kafka.Producer) http.Handler {
	r := chi.NewRouter()

	// Global middleware chain
	r.Use(chimw.Recoverer)
	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(chimw.Logger)
	r.Use(chimw.Timeout(30 * time.Second))

	// Health probes — no auth required
	var healthDB *pgxpool.Pool
	if p, ok := db.(*pgxpool.Pool); ok {
		healthDB = p
	}
	healthHandler := health.NewHealthHandler(health.HealthConfig{
		DB:          healthDB,
		Redis:       rdb,
		KafkaBroker: cfg.KafkaBrokers[0],
		ServiceName: "store-service",
		Version:     cfg.Version,
	})
	r.Mount("/healthz", healthHandler)

	// Init repositories
	storeRepo := repository.NewStoreRepository(db)
	qrRepo := repository.NewQRTokenRepository(db)
	deviceRepo := repository.NewDeviceRepository(db)

	// Init Redis Adapter for interfaces
	redisAdapter := service.NewRedisAdapter(rdb)

	// Init middleware
	jwtMiddleware := mw.NewJWTMiddleware(cfg.JWTPublicKey, cfg.JWTPrivateKey, redisAdapter)
	chainLookup := &chainLookupAdapter{repo: storeRepo}
	chainScopeMiddleware := mw.ChainScope(chainLookup)

	// Init services
	storeSvc := service.NewStoreService(storeRepo, qrRepo, redisAdapter, producer)
	deviceSvc := service.NewDeviceService(deviceRepo, redisAdapter)

	// Init handlers
	storeHandler := handler.NewStoreHandler(storeSvc)
	deviceHandler := handler.NewDeviceHandler(deviceSvc)

	// Public
	r.Get("/v1/store/nearby", storeHandler.Nearby)

	// Customer routes
	r.Group(func(r chi.Router) {
		r.Use(jwtMiddleware.RequireCustomer())
		r.Post("/v1/store/bind", storeHandler.Bind)
		r.Post("/v1/store/{id}/exit", storeHandler.Exit)
		r.Get("/v1/store/{id}/occupancy", storeHandler.Occupancy)
	})

	// Any authenticated user
	r.Group(func(r chi.Router) {
		r.Use(jwtMiddleware.RequireAuth())
		r.Use(chainScopeMiddleware)
		r.Get("/v1/store/{id}", storeHandler.GetStore)
		r.Get("/v1/store/{id}/hours", storeHandler.Hours)
	})

	// Staff only
	r.Group(func(r chi.Router) {
		r.Use(jwtMiddleware.RequireStaff())
		r.Use(chainScopeMiddleware)
		r.Put("/v1/store/{id}/capacity", storeHandler.UpdateCapacity)
		r.Get("/v1/store/{id}/devices", deviceHandler.ListDevices)
	})

	// Health & Metrics
	r.Handle("/metrics", promhttp.Handler())

	return r
}
