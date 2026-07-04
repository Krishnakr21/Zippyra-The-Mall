package middleware

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/zippyra/platform/shared/logger"
)

// Logging is a chi middleware that logs the start and end of each request,
// along with relevant metadata like request_id, dur, method, path, and claims if present.
func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip health and metric paths
		if r.URL.Path == "/health" || r.URL.Path == "/metrics" {
			next.ServeHTTP(w, r)
			return
		}

		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

		// Serve the request
		next.ServeHTTP(ww, r)

		reqContext := r.Context()
		reqID := middleware.GetReqID(reqContext)
		
		l := logger.Ctx(reqContext).Info().
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Int("status", ww.Status()).
			Dur("duration", time.Since(start)).
			Str("request_id", reqID)

		if claims := GetClaimsFromContext(reqContext); claims != nil {
			l = l.Str("user_id", claims.UserID)
			if claims.StoreID != "" {
				l = l.Str("store_id", claims.StoreID)
			}
		}

		l.Msg("request completed")
	})
}
