// Usage in every service main.go:
//
//   srv := sharedhttp.New(sharedhttp.ServerConfig{
//       Addr:        ":8080",
//       Handler:     router,
//       ServiceName: "auth-service",
//   })
//   if err := srv.Run(context.Background()); err != nil {
//       log.Fatal().Err(err).Msg("server error")
//   }
//   // Cleanup AFTER server drains — ORDER MATTERS:
//   kafkaProducer.Close()  // flush buffered messages
//   dbPool.Close()         // release connections
//   redisClient.Close()    // release connections

package http

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type ServerConfig struct {
	Addr            string        // ":8080"
	Handler         http.Handler
	ReadTimeout     time.Duration // default 30s
	WriteTimeout    time.Duration // default 30s
	IdleTimeout     time.Duration // default 120s
	ShutdownTimeout time.Duration // default 25s — must be < K8s terminationGracePeriodSeconds (30s)
	ServiceName     string
}

type Server struct {
	config ServerConfig
	srv    *http.Server
}

func New(cfg ServerConfig) *Server {
	if cfg.Addr == "" {
		cfg.Addr = ":8080"
	}
	if cfg.ReadTimeout == 0 {
		cfg.ReadTimeout = 30 * time.Second
	}
	if cfg.WriteTimeout == 0 {
		cfg.WriteTimeout = 30 * time.Second
	}
	if cfg.IdleTimeout == 0 {
		cfg.IdleTimeout = 120 * time.Second
	}
	if cfg.ShutdownTimeout == 0 {
		cfg.ShutdownTimeout = 25 * time.Second
	}

	return &Server{
		config: cfg,
		srv: &http.Server{
			Addr:         cfg.Addr,
			Handler:      cfg.Handler,
			ReadTimeout:  cfg.ReadTimeout,
			WriteTimeout: cfg.WriteTimeout,
			IdleTimeout:  cfg.IdleTimeout,
		},
	}
}

// Run starts the server and blocks until SIGTERM or SIGINT.
// On signal: stops accepting new connections, waits for in-flight
// requests up to ShutdownTimeout, then returns nil.
// Caller must close Kafka producers and DB pools AFTER Run() returns.
func (s *Server) Run(ctx context.Context) error {
	// Listen for syscall.SIGTERM and syscall.SIGINT on buffered channel size 1
	shutdownSig := make(chan os.Signal, 1)
	signal.Notify(shutdownSig, syscall.SIGTERM, syscall.SIGINT)

	serverErr := make(chan error, 1)

	// Start srv.ListenAndServe() in a goroutine
	go func() {
		log.Printf("starting HTTP server for %s on %s", s.config.ServiceName, s.config.Addr)
		if err := s.srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	// Wait for shutdown signal or server error
	select {
	case sig := <-shutdownSig:
		log.Printf("shutdown signal received (%s), draining in-flight requests...", sig.String())

		// Call srv.Shutdown(shutdownCtx) with ShutdownTimeout context
		shutdownCtx, cancel := context.WithTimeout(context.Background(), s.config.ShutdownTimeout)
		defer cancel()

		if err := s.srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("graceful shutdown failed for %s: %v", s.config.ServiceName, err)
			return err
		}

		log.Printf("shutdown complete, service=%s", s.config.ServiceName)
		return nil

	case err := <-serverErr:
		log.Printf("HTTP server error for %s: %v", s.config.ServiceName, err)
		return err

	case <-ctx.Done():
		log.Printf("context cancelled, shutting down %s...", s.config.ServiceName)
		shutdownCtx, cancel := context.WithTimeout(context.Background(), s.config.ShutdownTimeout)
		defer cancel()
		return s.srv.Shutdown(shutdownCtx)
	}
}
