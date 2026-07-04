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
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/zippyra/platform/services/payment-service/config"
	"github.com/zippyra/platform/services/payment-service/internal/handler"
	"github.com/zippyra/platform/services/payment-service/internal/kafka"
	"github.com/zippyra/platform/services/payment-service/internal/repository"
	"github.com/zippyra/platform/services/payment-service/internal/service"
	sharedhttp "github.com/zippyra/platform/shared/http"
	"github.com/zippyra/platform/shared/middleware"
)

func main() {
	cfg := config.Load()

	// 1. Context for graceful shutdown
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

	// 2. Database Pool (Production PGX)
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	// 3. Components Wiring
	payRepo := repository.NewPaymentRepository(pool)
	outboxRepo := repository.NewOutboxRepository(pool)
	webhookRepo := repository.NewWebhookRepository(pool)

	producer := kafka.NewProducer(cfg.KafkaBrokers)
	defer producer.Close()

	rzpClient := service.NewRazorpayClient(cfg.RazorpayKeyID, cfg.RazorpayKeySecret)
	cfClient := service.NewCashfreeClient(cfg.CashfreeAppID, cfg.CashfreeSecretKey)

	routerSvc := service.NewGatewayRouter(rzpClient, cfClient)
	paySvc := service.NewPaymentService(pool, payRepo, outboxRepo, routerSvc)
	
	relay := service.NewOutboxRelay(pool, producer)
	relay.Start(ctx)

	// 4. Handlers
	payHandler := handler.NewPaymentHandler(paySvc)
	webhookHandler := handler.NewWebhookHandler(paySvc, webhookRepo, cfg.RazorpayWebhookSecret, cfg.CashfreeSecretKey)

	// 5. Router Setup
	r := chi.NewRouter()
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.Timeout(60 * time.Second))

	// Routes
	r.Post("/v1/payment/webhook/razorpay", webhookHandler.Razorpay)
	r.Post("/v1/payment/webhook/cashfree", webhookHandler.Cashfree)

	r.Group(func(r chi.Router) {
		r.Use(middleware.Auth)
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireCustomer())
			r.Post("/v1/payment/initiate", payHandler.Initiate)
			r.Get("/v1/payment/status/{payment_id}", payHandler.Status)
			r.Get("/v1/payment/history", payHandler.History)
		})
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireStaff())
			r.Post("/v1/payment/refund/{payment_id}", payHandler.Refund)
		})
	})

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	// 6. Server Initialization
	srv := sharedhttp.New(sharedhttp.ServerConfig{
		Addr:        cfg.AppPort,
		Handler:     r,
		ServiceName: "payment-service",
	})

	return srv.Run(ctx)
}
