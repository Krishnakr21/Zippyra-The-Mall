package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/zippyra/platform/shared/jwt"
)

type ClaimsContextKey struct{}

// ContextWithClaims returns a new context with the given claims.
func ContextWithClaims(ctx context.Context, claims *jwt.ZippyraClaims) context.Context {
	return context.WithValue(ctx, ClaimsContextKey{}, claims)
}

// Auth validates EdDSA signed JWTs and attaches claims to the request context.
func Auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			http.Error(w, `{"error": "Unauthorized: missing token"}`, http.StatusUnauthorized)
			return
		}
		
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		claims, err := jwt.ValidateToken(tokenString)
		if err != nil {
			http.Error(w, `{"error": "Unauthorized: invalid token"}`, http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), ClaimsContextKey{}, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetClaimsFromContext returns the parsed JWT claims from the context if present.
func GetClaimsFromContext(ctx context.Context) *jwt.ZippyraClaims {
	claims, ok := ctx.Value(ClaimsContextKey{}).(*jwt.ZippyraClaims)
	if !ok {
		return nil
	}
	return claims
}

func requireUserType(allowed jwt.UserType) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := GetClaimsFromContext(r.Context())
			if claims == nil {
				http.Error(w, `{"error": "Unauthorized: token missing from context"}`, http.StatusUnauthorized)
				return
			}
			if claims.UserType != allowed {
				http.Error(w, `{"error": "Forbidden: insufficient permissions"}`, http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireCustomer enforces that the caller has CUSTOMER user type.
func RequireCustomer() func(http.Handler) http.Handler { return requireUserType(jwt.UserTypeCustomer) }

// RequireStaff enforces that the caller has STAFF user type.
func RequireStaff() func(http.Handler) http.Handler    { return requireUserType(jwt.UserTypeStaff) }

// RequireAdmin enforces that the caller has ADMIN user type.
func RequireAdmin() func(http.Handler) http.Handler    { return requireUserType(jwt.UserTypeAdmin) }

// RequireChainHQ enforces that the caller has CHAIN_HQ user type.
func RequireChainHQ() func(http.Handler) http.Handler  { return requireUserType(jwt.UserTypeChainHQ) }
