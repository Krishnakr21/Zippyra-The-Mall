package middleware

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimid "github.com/go-chi/chi/v5/middleware"
)

// ApplyStandard applies the standard baseline middlewares (Recovery, RequestID, Logger, CORS, Timeout, Metrics, Tracing).
// Auth and RateLimit should be applied by the service per-route.
func ApplyStandard(r *chi.Mux, serviceName string) {
	r.Use(chimid.Recoverer)
	r.Use(chimid.RequestID)
	r.Use(MaxBytesReader(1024 * 1024)) // 1MB limit per requirements
	r.Use(Logging)
	r.Use(CORS)
	r.Use(chimid.Timeout(30 * time.Second))
	// Depending on service, Auth goes after Timeout. Let's assume Auth is selective.
	r.Use(Metrics(serviceName))
	r.Use(Tracing)
}

// MaxBytesReader limits the request body size.
func MaxBytesReader(limit int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, limit)
			next.ServeHTTP(w, r)
		})
	}
}

// CORS defines basic Cross-Origin Resource Sharing logic.
func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, PATCH")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With, Trace-Id")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
