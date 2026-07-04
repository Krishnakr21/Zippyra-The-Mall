package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/log"

	"github.com/zippyra/platform/services/catalog-service/config"
	"github.com/zippyra/platform/services/catalog-service/internal/cache"
	"github.com/zippyra/platform/services/catalog-service/internal/handler"
	"github.com/zippyra/platform/services/catalog-service/internal/repository"
	"github.com/zippyra/platform/services/catalog-service/internal/search"
	"github.com/zippyra/platform/services/catalog-service/internal/service"
	"github.com/zippyra/platform/shared/logger"
	"github.com/zippyra/platform/shared/otel"
)

func main() {
	if err := run(context.Background()); err != nil {
		log.Fatal().Err(err).Msg("failed to start server")
	}
}

func run(ctx context.Context) error {
	cfg, _ := config.Load()

	logger.Init("catalog-service", "info")

	if _, err := otel.Init("catalog-service", "v1.0.0", "production"); err != nil {
		log.Warn().Err(err).Msg("failed to initialize otel")
	}

	db, err := cfg.NewDBPool(ctx)
	if err != nil {
		return fmt.Errorf("failed to create database pool: %w", err)
	}
	defer db.Close()

	r := setupRouter(cfg, db)

	server := &http.Server{
		Addr:         ":" + cfg.Server.Port,
		Handler:      r,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		select {
		case <-stop:
			log.Info().Msg("shutting down server (signal)...")
		case <-ctx.Done():
			log.Info().Msg("shutting down server (context)...")
		}
		
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Error().Err(err).Msg("server shutdown failed")
		}
	}()

	return server.ListenAndServe()
}

func setupRouter(cfg *config.Config, db repository.DB) http.Handler {
	redisClient := cfg.NewRedisClient()
	skuCache := cache.NewSKUCache(redisClient)

	var searchClient search.SearchClient
	var err error
	if cfg.Search.Addr != "" {
		searchClient, err = search.NewOpenSearchClient(cfg.Search.Addr, cfg.Search.Username, cfg.Search.Password)
		if err != nil {
			log.Warn().Err(err).Msg("failed to create opensearch client, search will fallback to database")
		}
	}

	productRepo := repository.NewProductRepo(db)
	offerRepo := repository.NewOfferRepo(db)

	searchService := service.NewSearchService(productRepo, searchClient)
	barcodeService := service.NewBarcodeService(productRepo, skuCache, searchService)
	syncService := service.NewSyncService(productRepo)
	offerService := service.NewOfferService(offerRepo, skuCache)

	barcodeHandler := handler.NewBarcodeHandler(barcodeService)
	searchHandler := handler.NewSearchHandler(searchService)
	syncHandler := handler.NewSyncHandler(syncService)
	offerHandler := handler.NewOfferHandler(offerService)

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))
	r.Use(middleware.AllowContentType("application/json"))

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	r.Handle("/metrics", promhttp.Handler())

	r.Get("/v1/catalog/barcode/{barcode}", barcodeHandler.Lookup)
	r.Get("/v1/catalog/search", searchHandler.Search)
	r.Get("/v1/catalog/category/{category}", searchHandler.ByCategory)
	r.Get("/v1/catalog/offers", offerHandler.GetOffers)

	r.Group(func(r chi.Router) {
		r.Post("/v1/catalog/sync", syncHandler.Sync)
	})

	r.Group(func(r chi.Router) {
		r.Post("/v1/catalog/products", barcodeHandler.UpsertProduct)
	})

	return r
}
