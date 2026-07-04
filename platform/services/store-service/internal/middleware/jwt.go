package middleware

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/rs/zerolog/log"

	"github.com/zippyra/platform/services/store-service/internal/service"
	"github.com/zippyra/platform/shared/errors"
)

type contextKey string

const (
	ContextUserID   contextKey = "user_id"
	ContextUserType contextKey = "user_type"
	ContextJTI      contextKey = "jti"
	ContextChainID  contextKey = "chain_id"
	ContextStoreID  contextKey = "store_id"
)

// Claims are the JWT claims used by Zippyra tokens.
type Claims struct {
	UserID   string `json:"user_id"`
	UserType string `json:"user_type"` // CUSTOMER, STAFF, CHAIN_HQ, ADMIN
	ChainID  string `json:"chain_id,omitempty"`
	StoreID  string `json:"store_id,omitempty"`
	jwt.RegisteredClaims
}

// JWTMiddleware validates Ed25519 JWTs and checks the blacklist.
type JWTMiddleware struct {
	publicKeys map[string]ed25519.PublicKey
	currentKID string
	privateKey ed25519.PrivateKey
	redis      service.RedisStore
}

var fatalLogger = log.Fatal

// NewJWTMiddleware creates a JWT middleware with Ed25519 keys.
func NewJWTMiddleware(publicKeyB64, privateKeyB64 string, redis service.RedisStore) *JWTMiddleware {
	m, err := NewJWTMiddlewareE(publicKeyB64, privateKeyB64, redis)
	if err != nil {
		fatalLogger().Err(err).Msg("failed to init JWT middleware")
		return nil
	}
	return m
}

// NewJWTMiddlewareE creates a JWT middleware, returning an error instead of fatal.
func NewJWTMiddlewareE(publicKeyB64, privateKeyB64 string, redis service.RedisStore) (*JWTMiddleware, error) {
	publicKeys, err := parsePublicKeys(publicKeyB64)
	if err != nil {
		return nil, fmt.Errorf("decode public key: %w", err)
	}

	currentKID := "default"
	privKID, privBytes, err := parsePrivateKey(privateKeyB64)
	if err != nil {
		return nil, fmt.Errorf("decode private key: %w", err)
	}
	if privKID != "" {
		currentKID = privKID
	}

	return &JWTMiddleware{
		publicKeys: publicKeys,
		currentKID: currentKID,
		privateKey: ed25519.PrivateKey(privBytes),
		redis:      redis,
	}, nil
}

// ValidateToken parses and validates an Ed25519 JWT.
func (m *JWTMiddleware) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodEd25519); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		kid := ""
		if v, ok := t.Header["kid"]; ok {
			if s, ok := v.(string); ok {
				kid = s
			}
		}
		if kid == "" {
			kid = "default"
		}
		if pk, ok := m.publicKeys[kid]; ok {
			return pk, nil
		}
		if pk, ok := m.publicKeys["default"]; ok {
			return pk, nil
		}
		return nil, fmt.Errorf("unknown kid")
	})
	if err != nil {
		return nil, err
	}
	claims := token.Claims.(*Claims)
	return claims, nil
}

// IsBlacklisted checks if a JTI is in the Redis blacklist.
func (m *JWTMiddleware) IsBlacklisted(ctx context.Context, jti string) (bool, error) {
	rCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()

	return m.redis.Exists(rCtx, "blacklist:"+jti)
}

// RequireAuth returns middleware that requires any valid JWT.
func (m *JWTMiddleware) RequireAuth() func(http.Handler) http.Handler {
	return m.requireAuth("")
}

// RequireCustomer returns middleware that requires a valid CUSTOMER JWT.
func (m *JWTMiddleware) RequireCustomer() func(http.Handler) http.Handler {
	return m.requireAuth("CUSTOMER")
}

// RequireStaff returns middleware that requires a valid STAFF JWT.
func (m *JWTMiddleware) RequireStaff() func(http.Handler) http.Handler {
	return m.requireAuth("STAFF")
}

func (m *JWTMiddleware) requireAuth(requiredType string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := r.Header.Get("X-Request-ID")

			// Extract Bearer token
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				errors.WriteError(w, http.StatusUnauthorized, errors.ErrUnauthorized,
					"Missing Authorization header", requestID)
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || parts[0] != "Bearer" {
				errors.WriteError(w, http.StatusUnauthorized, errors.ErrTokenInvalid,
					"Invalid Authorization header format", requestID)
				return
			}
			tokenString := parts[1]

			// Validate token
			claims, err := m.ValidateToken(tokenString)
			if err != nil {
				if strings.Contains(err.Error(), "expired") {
					errors.WriteError(w, http.StatusUnauthorized, errors.ErrTokenExpired,
						"Token has expired", requestID)
				} else {
					errors.WriteError(w, http.StatusUnauthorized, errors.ErrTokenInvalid,
						"Invalid token", requestID)
				}
				return
			}

			// Check user type if required
			if requiredType != "" && claims.UserType != requiredType {
				errors.WriteError(w, http.StatusForbidden, errors.ErrWrongUserType,
					"Insufficient permissions for this endpoint", requestID)
				return
			}

			// Check blacklist
			blacklisted, err := m.IsBlacklisted(r.Context(), claims.RegisteredClaims.ID)
			if err != nil {
				log.Error().Err(err).Msg("blacklist check failed")
				errors.WriteInternalError(w, requestID)
				return
			}
			if blacklisted {
				errors.WriteError(w, http.StatusUnauthorized, errors.ErrTokenBlacklisted,
					"Token has been revoked", requestID)
				return
			}

			// Set context values
			ctx := context.WithValue(r.Context(), ContextUserID, claims.UserID)
			ctx = context.WithValue(ctx, ContextUserType, claims.UserType)
			ctx = context.WithValue(ctx, ContextJTI, claims.RegisteredClaims.ID)
			ctx = context.WithValue(ctx, ContextChainID, claims.ChainID)
			ctx = context.WithValue(ctx, ContextStoreID, claims.StoreID)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetUserIDFromContext extracts the user_id from a request context.
func GetUserIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(ContextUserID).(string); ok {
		return v
	}
	return ""
}

// GetUserTypeFromContext extracts the user_type from a request context.
func GetUserTypeFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(ContextUserType).(string); ok {
		return v
	}
	return ""
}

// GetChainIDFromContext extracts the chain_id from a request context.
func GetChainIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(ContextChainID).(string); ok {
		return v
	}
	return ""
}

// GetStoreIDFromContext extracts the store_id from a request context.
func GetStoreIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(ContextStoreID).(string); ok {
		return v
	}
	return ""
}

func parsePublicKeys(input string) (map[string]ed25519.PublicKey, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, fmt.Errorf("empty public key")
	}

	keys := map[string]ed25519.PublicKey{}
	parts := strings.Split(input, ",")
	if len(parts) == 1 && !strings.Contains(parts[0], ":") {
		b64 := parts[0]
		raw, err := base64.StdEncoding.DecodeString(b64)
		if err != nil {
			return nil, err
		}

		// Try parsing as PEM first
		if strings.Contains(string(raw), "BEGIN PUBLIC KEY") {
			pk, err := jwt.ParseEdPublicKeyFromPEM(raw)
			if err != nil {
				return nil, err
			}
			keys["default"] = pk.(ed25519.PublicKey)
		} else {
			// Fallback to raw bytes
			if len(raw) != ed25519.PublicKeySize {
				return nil, fmt.Errorf("invalid ed25519 public key size: %d", len(raw))
			}
			keys["default"] = ed25519.PublicKey(raw)
		}
		return keys, nil
	}

	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		kv := strings.SplitN(p, ":", 2)
		if len(kv) != 2 {
			return nil, fmt.Errorf("invalid public key entry")
		}
		kid := strings.TrimSpace(kv[0])
		b64 := strings.TrimSpace(kv[1])
		if kid == "" || b64 == "" {
			return nil, fmt.Errorf("invalid public key entry")
		}
		pubBytes, err := base64.StdEncoding.DecodeString(b64)
		if err != nil {
			return nil, err
		}
		keys[kid] = ed25519.PublicKey(pubBytes)
		if _, ok := keys["default"]; !ok {
			keys["default"] = ed25519.PublicKey(pubBytes)
		}
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("no public keys")
	}
	return keys, nil
}

func parsePrivateKey(input string) (string, []byte, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", nil, fmt.Errorf("empty private key")
	}

	kid := ""
	b64 := input
	if strings.Contains(input, ":") {
		kv := strings.SplitN(input, ":", 2)
		if len(kv) == 2 {
			kid = strings.TrimSpace(kv[0])
			b64 = strings.TrimSpace(kv[1])
		}
	}

	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return "", nil, err
	}

	// Try parsing as PEM first
	if strings.Contains(string(raw), "BEGIN PRIVATE KEY") {
		pk, err := jwt.ParseEdPrivateKeyFromPEM(raw)
		if err != nil {
			return "", nil, err
		}
		return kid, pk.(ed25519.PrivateKey), nil
	}

	// Fallback to raw bytes
	if len(raw) != ed25519.PrivateKeySize {
		return "", nil, fmt.Errorf("invalid ed25519 private key size: %d", len(raw))
	}

	return kid, ed25519.PrivateKey(raw), nil
}
