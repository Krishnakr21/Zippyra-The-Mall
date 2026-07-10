package main

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	pgxpool "github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/zippyra/platform/services/exit-validation-service/config"
	"github.com/zippyra/platform/services/exit-validation-service/internal/handler"
	"github.com/zippyra/platform/services/exit-validation-service/internal/kafka"
	internalmqtt "github.com/zippyra/platform/services/exit-validation-service/internal/mqtt"
	"github.com/zippyra/platform/services/exit-validation-service/internal/repository"
	"github.com/zippyra/platform/services/exit-validation-service/internal/service"
	sharedhttp "github.com/zippyra/platform/shared/http"
	"github.com/zippyra/platform/shared/middleware"
	"github.com/zippyra/platform/shared/otel"
)

func main() {
	cfg := config.Load()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := Run(ctx, cfg); err != nil {
		log.Error().Err(err).Msg("server exited with error")
		os.Exit(1)
	}
}

func Run(ctx context.Context, cfg *config.Config) error {
	// Logger Setup
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	if cfg.AppEnv == "local" || cfg.AppEnv == "development" {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	}

	// Initialize OpenTelemetry
	if cfg.JaegerEndpoint != "" {
		shutdownOtel, err := otel.Init("exit-validation-service", cfg.AppEnv, cfg.JaegerEndpoint)
		if err == nil {
			defer shutdownOtel()
		} else {
			log.Warn().Err(err).Msg("OTel/Jaeger init failed")
		}
	}

	// 1. JWT Public Key Decoding
	var pubKey ed25519.PublicKey
	if cfg.JWTPublicKey != "" {
		pubBytes, err := base64.StdEncoding.DecodeString(cfg.JWTPublicKey)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to decode JWT_PUBLIC_KEY")
		}
		pubKey = ed25519.PublicKey(pubBytes)
	} else {
		log.Warn().Msg("JWT_PUBLIC_KEY is not set, exit token verification will fail unless overridden in tests")
	}

	// 2. Database Connection Pool
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	// 3. Redis Client Connection
	opts, err := goredis.ParseURL(cfg.RedisURL)
	if err != nil {
		return err
	}
	rdb := goredis.NewClient(opts)
	defer rdb.Close()

	// Verify Redis Connection
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Warn().Err(err).Msg("Could not connect to Redis on startup")
	}

	// 4. Kafka Producer
	producer := kafka.NewProducer(cfg.KafkaBrokers)
	defer producer.Close()

	// 5. MQTT client & Gate Commander
	var gateCommander *service.GateCommander
	if cfg.AppEnv == "local" || cfg.MQTTBrokerURL == "" {
		log.Info().Msg("Starting gate commander in Mock / Local Logging Mode")
		gateCommander = service.NewMockGateCommander()
	} else {
		mqttClient, err := internalmqtt.NewMQTTClient(cfg.MQTTBrokerURL, "exit-validation-service", cfg.MQTTUsername, cfg.MQTTPassword)
		if err != nil {
			log.Error().Err(err).Msg("MQTT connection failed, starting gate commander in mock mode as fallback")
			gateCommander = service.NewMockGateCommander()
		} else {
			gateCommander = service.NewGateCommander(mqttClient, 1)
		}
	}

	// 6. Repositories
	exitRepo := repository.NewExitTokenRepository(pool)

	// 7. Services
	exitSvc := service.NewExitService(exitRepo, rdb, producer, gateCommander, pubKey, cfg.StoreServiceURL)

	// 8. Handlers
	exitHandler := handler.NewExitHandler(exitSvc)

	// 9. HTTP Router Setup
	r := chi.NewRouter()
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.Timeout(60 * time.Second))

	// Health check route
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	// Protected Routes Group
	r.Group(func(r chi.Router) {
		r.Use(middleware.Auth)

		// Customer routes
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireCustomer())
			r.Post("/v1/exit/validate", exitHandler.Validate)
			r.Get("/v1/exit/token/{order_id}", exitHandler.GetTokenStatus)
		})

		// Staff routes
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireStaff())
			r.Post("/v1/exit/validate/staff-override", exitHandler.StaffOverride)
		})
	})

	// 10. HTTP Server Initialization
	port := cfg.Port
	if !strings.HasPrefix(port, ":") {
		port = ":" + port
	}

	srv := sharedhttp.New(sharedhttp.ServerConfig{
		Addr:        port,
		Handler:     r,
		ServiceName: "exit-validation-service",
	})

	log.Info().Str("port", port).Msg("starting exit-validation-service HTTP server...")
	return srv.Run(ctx)
}
