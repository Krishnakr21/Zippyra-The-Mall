package health

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// HealthConfig provides dependencies for health checks
type HealthConfig struct {
	DB          *pgxpool.Pool
	Redis       *redis.Client
	KafkaBroker string
	ServiceName string
	Version     string
}

type healthHandler struct {
	cfg       HealthConfig
	startedAt time.Time
	mu        sync.RWMutex
	ready     bool
}

// NewHealthHandler returns a single http.Handler that mounts all 3 routes internally.
func NewHealthHandler(cfg HealthConfig) http.Handler {
	h := &healthHandler{
		cfg:       cfg,
		startedAt: time.Now(),
	}

	// In a real app, you'd set this to true after migrations/cache warming.
	// For this shared package, we'll wait a few seconds to simulate startup.
	go func() {
		time.Sleep(5 * time.Second)
		h.mu.Lock()
		h.ready = true
		h.mu.Unlock()
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz/live", h.handleLiveness)
	mux.HandleFunc("/healthz/ready", h.handleReadiness)
	mux.HandleFunc("/healthz/startup", h.handleStartup)

	return mux
}

// handleLiveness: Always returns 200 if process is running. NEVER checks dependencies.
func (h *healthHandler) handleLiveness(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "alive"})
}

// handleReadiness: Returns 200 only if ALL dependencies are healthy.
func (h *healthHandler) handleReadiness(w http.ResponseWriter, r *http.Request) {
	checks := make(map[string]string)
	isHealthy := true

	// Postgres check (2s timeout)
	if h.cfg.DB != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		if err := h.cfg.DB.Ping(ctx); err != nil {
			checks["postgres"] = "error: " + err.Error()
			isHealthy = false
		} else {
			checks["postgres"] = "ok"
		}
		cancel()
	}

	// Redis check (500ms timeout)
	if h.cfg.Redis != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 500*time.Millisecond)
		if err := h.cfg.Redis.Ping(ctx).Err(); err != nil {
			checks["redis"] = "error: " + err.Error()
			isHealthy = false
		} else {
			checks["redis"] = "ok"
		}
		cancel()
	}

	// Kafka check (1s timeout)
	if h.cfg.KafkaBroker != "" {
		conn, err := net.DialTimeout("tcp", h.cfg.KafkaBroker, 1*time.Second)
		if err != nil {
			checks["kafka"] = "error: " + err.Error()
			isHealthy = false
		} else {
			_ = conn.Close()
			checks["kafka"] = "ok"
		}
	}

	w.Header().Set("Content-Type", "application/json")
	resp := map[string]interface{}{
		"status": "ready",
		"checks": checks,
	}

	if !isHealthy {
		resp["status"] = "degraded"
		w.WriteHeader(http.StatusServiceUnavailable)
	} else {
		w.WriteHeader(http.StatusOK)
	}

	_ = json.NewEncoder(w).Encode(resp)
}

// handleStartup: Returns 200 once initial setup is complete.
func (h *healthHandler) handleStartup(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	ready := h.ready
	h.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	if !ready {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "starting"})
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":     "started",
		"started_at": h.startedAt.Format(time.RFC3339),
	})
}
