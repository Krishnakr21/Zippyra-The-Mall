package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/zippyra/platform/services/order-service/config"
	"github.com/zippyra/platform/services/order-service/internal/handler"
	"github.com/zippyra/platform/services/order-service/internal/kafka"
	"github.com/zippyra/platform/services/order-service/internal/repository"
	"github.com/zippyra/platform/services/order-service/internal/service"
	sharedhttp "github.com/zippyra/platform/shared/http"
	"github.com/zippyra/platform/shared/middleware"
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
	if cfg.Environment == "development" {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	}

	// 1. Database Pool
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	// 2. Redis Client
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	defer rdb.Close()

	// Verify Redis Connection
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Warn().Err(err).Msg("could not ping Redis on startup")
	}

	// 3. Kafka Producer
	producer := kafka.NewProducer(cfg.KafkaBrokers)
	defer producer.Close()

	// 4. Repositories
	orderRepo := repository.NewOrderRepository(pool)
	orderItemRepo := repository.NewOrderItemRepository(pool)
	exitTokenRepo := repository.NewExitTokenRepository(pool)
	returnRepo := repository.NewReturnRepository(pool)

	// 5. Services
	jwtSvc := service.RealJWTService{}
	exitTokenSvc := service.NewExitTokenService(exitTokenRepo, orderRepo, rdb, jwtSvc)
	invoiceSvc := service.NewInvoiceService(orderRepo, orderItemRepo, producer)
	orderSvc := service.NewOrderService(pool, orderRepo, orderItemRepo, exitTokenSvc, invoiceSvc, producer)
	returnSvc := service.NewReturnService(orderRepo, orderItemRepo, returnRepo, rdb, producer, cfg.PaymentServiceURL)

	// 6. Kafka Consumer
	consumer := kafka.NewConsumer(cfg.KafkaBrokers, "order-service-group", "payment.confirmed", orderSvc, producer)
	go func() {
		log.Info().Msg("starting order-service kafka consumer...")
		if err := consumer.Start(ctx); err != nil && ctx.Err() == nil {
			log.Error().Err(err).Msg("kafka consumer stopped unexpectedly")
		}
	}()
	defer consumer.Close()

	// 7. Handlers
	orderHandler := handler.NewOrderHandler(orderSvc, exitTokenSvc)
	returnHandler := handler.NewReturnHandler(returnSvc)

	// 8. Router Setup
	r := chi.NewRouter()
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.Timeout(60 * time.Second))

	// Unauthenticated health endpoint
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	// Authenticated routes group
	r.Group(func(r chi.Router) {
		r.Use(middleware.Auth)

		// Customer routes
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireCustomer())
			r.Get("/v1/order/{id}", orderHandler.GetByID)
			r.Get("/v1/order/history", orderHandler.GetHistory)
			r.Get("/v1/order/{id}/exit-token", orderHandler.GetExitToken)
			r.Post("/v1/order/{id}/return", returnHandler.Create)
		})

		// Staff routes
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireStaff())
			r.Post("/v1/order/{id}/return/accept", returnHandler.Accept)
		})
	})

	// 9. HTTP Server Initialization
	srv := sharedhttp.New(sharedhttp.ServerConfig{
		Addr:        cfg.AppPort,
		Handler:     r,
		ServiceName: "order-service",
	})

	log.Info().Str("port", cfg.AppPort).Msg("starting order-service http server...")
	return srv.Run(ctx)
}
