package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"service", "method", "path", "status"},
	)

	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Duration of HTTP requests in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"service", "method", "path", "status"},
	)

	httpRequestsInFlight = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "http_requests_in_flight",
			Help: "Current number of HTTP requests being processed",
		},
		[]string{"service"},
	)
)

// Metrics creates a middleware that records Prometheus metrics for the handler.
func Metrics(serviceName string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/health" || r.URL.Path == "/metrics" {
				next.ServeHTTP(w, r)
				return
			}

			httpRequestsInFlight.WithLabelValues(serviceName).Inc()
			defer httpRequestsInFlight.WithLabelValues(serviceName).Dec()

			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			next.ServeHTTP(ww, r)

			// Get matching route pattern to prevent high cardinality metric labels
			routeCtx := chi.RouteContext(r.Context())
			path := r.URL.Path
			if routeCtx != nil && routeCtx.RoutePattern() != "" {
				path = routeCtx.RoutePattern()
			}

			status := strconv.Itoa(ww.Status())

			httpRequestsTotal.WithLabelValues(serviceName, r.Method, path, status).Inc()
			httpRequestDuration.WithLabelValues(serviceName, r.Method, path, status).Observe(time.Since(start).Seconds())
		})
	}
}

// MetricsHandler returns the prometheus HTTP handler for /metrics endpoint exposure.
func MetricsHandler() http.Handler {
	return promhttp.Handler()
}
