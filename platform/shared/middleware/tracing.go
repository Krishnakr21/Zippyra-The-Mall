package middleware

import (
	"net/http"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

// Tracing manages OpenTelemetry trace propagation and spans for HTTP requests.
func Tracing(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		propagator := otel.GetTextMapPropagator()
		ctx := propagator.Extract(r.Context(), propagation.HeaderCarrier(r.Header))

		tracer := otel.Tracer("http-server")
		ctx, span := tracer.Start(ctx, r.URL.Path)
		defer span.End()

		spanContext := span.SpanContext()
		if spanContext.IsValid() {
			w.Header().Set("Trace-Id", spanContext.TraceID().String())
			w.Header().Set("Span-Id", spanContext.SpanID().String())
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
