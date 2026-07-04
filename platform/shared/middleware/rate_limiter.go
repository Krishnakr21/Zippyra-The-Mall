package middleware

import (
	"fmt"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
	zippyredis "github.com/zippyra/platform/shared/redis"
)

// RateLimit creates a middleware that uses a Redis-backed sliding window.
func RateLimit(client redis.Cmdable, limit int, window time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var key string
			claims := GetClaimsFromContext(r.Context())
			if claims != nil {
				key = fmt.Sprintf("rate_limit:user:%s", claims.UserID)
			} else {
				// Fallback to IP for unauthenticated requests
				ip := r.RemoteAddr
				key = fmt.Sprintf("rate_limit:ip:%s", ip)
			}

			allowed, err := zippyredis.CheckRateLimit(r.Context(), client, key, limit, window)
			if err != nil {
				// Log the error but fail open so we don't block legitimate traffic if redis flakes
				// In a full implementation, you'd log it to the request logger
			}

			if err == nil && !allowed {
				w.Header().Set("Retry-After", fmt.Sprintf("%d", int(window.Seconds())))
				http.Error(w, `{"error": "Too Many Requests"}`, http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
