package middleware

import (
	"net/http"

	"github.com/rs/zerolog/log"
)

func RequireCustomer() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userType := r.Header.Get("X-User-Type")
			if userType != "customer" {
				log.Warn().Str("userType", userType).Msg("unauthorized access attempt - not customer")
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func RequireStaff() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userType := r.Header.Get("X-User-Type")
			if userType != "staff" {
				log.Warn().Str("userType", userType).Msg("unauthorized access attempt - not staff")
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
