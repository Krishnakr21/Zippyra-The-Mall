package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	goredis "github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"

	"github.com/zippyra/platform/services/auth-service/config"
	"github.com/zippyra/platform/services/auth-service/internal/handler"
	"github.com/zippyra/platform/services/auth-service/internal/kafka"
	mw "github.com/zippyra/platform/services/auth-service/internal/middleware"
	"github.com/zippyra/platform/services/auth-service/internal/repository"
	"github.com/zippyra/platform/services/auth-service/internal/service"
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

var (
	loadConfig    = config.Load
	initSentry    = sentry.Init
	flushSentry   = sentry.Flush
	pgPoolNew     = pgxpool.New
	newPGPool     = func(ctx context.Context, connString string) (dbPool, error) { return pgPoolNew(ctx, connString) }
	parseRedisURL = goredis.ParseURL
	newRedis      = goredis.NewClient
	newProducer   = kafka.NewProducer
	serverNew     = sharedhttp.New
	newServer     = func(cfg sharedhttp.ServerConfig) serverRunner { return serverNew(cfg) }
	fatalLogger   = log.Fatal
	mainFatal     = func(err error) { fatalLogger().Err(err).Msg("fatal") }
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

	if cfg.SentryDSN != "" {
		if err := initSentry(sentry.ClientOptions{Dsn: cfg.SentryDSN, Environment: cfg.AppEnv}); err != nil {
			log.Error().Err(err).Msg("sentry init failed")
		}
		defer flushSentry(2 * time.Second)
	}

	// 2. Init logger
	// logger.Init("auth-service", cfg.AppEnv)

	// 3. Init OTel (tracing)
	// shutdownOtel, err := otel.Init("auth-service", cfg.AppEnv, cfg.JaegerEndpoint)
	// if err != nil { log.Fatal().Err(err).Msg("otel init failed") }
	// defer shutdownOtel()

	// 4. Init DB pool
	db, err := newPGPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("db init failed: %w", err)
	}
	defer db.Close()

	// 5. Init Redis client
	opts, err := parseRedisURL(cfg.RedisURL)
	if err != nil {
		return fmt.Errorf("redis url parse failed: %w", err)
	}
	rdb := newRedis(opts)
	defer rdb.Close()

	// 6. Init Kafka producer
	producer := newProducer(cfg.KafkaBrokers, kafka.TopicCustomerLogin)
	defer producer.Close()

	// 7. Build chi router with full middleware chain
	r := buildRouter(cfg, db, rdb, producer)

	// 8. Run server — blocks until SIGTERM
	srv := newServer(sharedhttp.ServerConfig{
		Addr:        ":" + cfg.Port,
		Handler:     r,
		ServiceName: "auth-service",
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
		ServiceName: "auth-service",
		Version:     cfg.Version,
	})
	r.Mount("/healthz", healthHandler)

	// Init repositories
	userRepo := repository.NewUserRepository(db)
	loginAttemptRepo := repository.NewLoginAttemptRepository(db)
	appVersionRepo := repository.NewAppVersionRepository(db)
	sessionStore := repository.NewSessionStoreDB(db)

	// Init middleware
	jwtMiddleware := mw.NewJWTMiddleware(cfg.JWTPublicKey, cfg.JWTPrivateKey, rdb)

	// Init services
	otpService := service.NewOTPService(rdb, loginAttemptRepo, cfg.OTPSalt, cfg.AppEnv)
	authService := service.NewAuthService(rdb, userRepo, loginAttemptRepo, sessionStore, jwtMiddleware, producer)
	sessionService := service.NewSessionService(rdb, sessionStore, jwtMiddleware)

	// Init handlers
	otpHandler := handler.NewOTPHandler(otpService, authService)
	sessionHandler := handler.NewSessionHandler(authService, sessionService, jwtMiddleware)
	versionHandler := handler.NewVersionHandler(appVersionRepo)

	// Public routes — no auth required
	r.Post("/v1/auth/otp/send", otpHandler.SendOTP)
	r.Post("/v1/auth/otp/verify", otpHandler.VerifyOTP)
	r.Get("/v1/auth/version-check", versionHandler.VersionCheck)

	// Protected routes — require valid CUSTOMER JWT
	r.Group(func(r chi.Router) {
		r.Use(jwtMiddleware.RequireCustomer())
		r.Post("/v1/auth/refresh", sessionHandler.Refresh)
		r.Post("/v1/auth/logout", sessionHandler.Logout)
		r.Get("/v1/auth/sessions", sessionHandler.ListSessions)
		r.Delete("/v1/auth/sessions/{session_id}", sessionHandler.RevokeSession)
		r.Delete("/v1/auth/sessions", sessionHandler.RevokeAllSessions)
	})

	// Metrics endpoint
	r.Handle("/metrics", promhttp.Handler())

	return r
}
