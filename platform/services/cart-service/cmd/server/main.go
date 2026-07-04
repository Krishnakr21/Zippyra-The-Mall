package main

import (
	"context"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"

	"github.com/zippyra/platform/services/cart-service/config"
	"github.com/zippyra/platform/services/cart-service/internal/handler"
	"github.com/zippyra/platform/services/cart-service/internal/kafka"
	"github.com/zippyra/platform/services/cart-service/internal/repository"
	"github.com/zippyra/platform/services/cart-service/internal/service"
	"github.com/zippyra/platform/shared/db"
	sharedhttp "github.com/zippyra/platform/shared/http"
	"github.com/zippyra/platform/shared/logger"
)

// fatalExit is the function called when main() encounters an error
// It can be overridden in tests to avoid process termination
var fatalExit = func(err error) {
	log.Fatal().Err(err).Msg("service failed")
}

func main() {
	if err := run(); err != nil {
		fatalExit(err)
	}
}

func run() error {
	// 1. Load config
	cfg := config.Load()
	logger.Init(cfg.LogLevel, "cart-service")

	ctx := context.Background()

	// 2. Initialize Infrastructure
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	if os.Getenv("TEST_MODE") != "1" {
		if err := rdb.Ping(ctx).Err(); err != nil {
			return err
		}
	}
	defer rdb.Close()

	pgPool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil && os.Getenv("TEST_MODE") != "1" {
		return err
	}
	if pgPool != nil {
		defer pgPool.Close()
	}

	producer := kafka.NewProducer(cfg.KafkaBrokers)

	// 3. Initialize Repositories
	cartRepo := repository.NewCartRepository(rdb, pgPool)
	productRepo := repository.NewProductRepository(rdb, cfg.CatalogServiceURL)

	// 4. Initialize Services
	cartService := service.NewCartService(cartRepo, productRepo)
	checkoutService := service.NewCheckoutService(cartRepo, productRepo, cartService)
	couponService := service.NewCouponService(productRepo, cartService)

	// 5. Initialize Handlers
	cartHandler := handler.NewCartHandler(cartService, producer)
	checkoutHandler := handler.NewCheckoutHandler(checkoutService, producer)
	couponHandler := handler.NewCouponHandler(couponService)

	// 6. Setup Router
	r := setupRouter(cartHandler, checkoutHandler, couponHandler)

	// 7. Start Server
	srv := sharedhttp.New(sharedhttp.ServerConfig{
		Addr:        cfg.AppPort,
		Handler:     r,
		ServiceName: "cart-service",
	})

	log.Info().Str("port", cfg.AppPort).Msg("cart-service starting")
	if os.Getenv("TEST_MODE") == "1" {
		return nil // Skip server run in test mode
	}
	return srv.Run(ctx)
}

func setupRouter(cartHandler *handler.CartHandler, checkoutHandler *handler.CheckoutHandler, couponHandler *handler.CouponHandler) *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	r.Route("/v1", func(r chi.Router) {
		r.Route("/cart", func(r chi.Router) {
			r.Post("/scan", cartHandler.ScanItem)
			r.Get("/", cartHandler.GetCart)
			r.Delete("/", cartHandler.ClearCart)
			r.Delete("/item/{barcode}", cartHandler.RemoveItem)

			r.Post("/checkout/init", checkoutHandler.InitCheckout)
			r.Post("/coupon", couponHandler.ApplyCoupon)
		})
		r.Post("/order/cash-payment", checkoutHandler.CashPayment)
	})
	return r
}
