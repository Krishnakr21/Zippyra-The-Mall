package jwt

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
	"github.com/zippyra/platform/shared/config"
)

type UserType string

const (
	UserTypeCustomer UserType = "CUSTOMER"
	UserTypeStaff    UserType = "STAFF"
	UserTypeChainHQ  UserType = "CHAIN_HQ"
	UserTypeAdmin    UserType = "ADMIN"
)

type ZippyraClaims struct {
	UserID   string   `json:"user_id"`
	UserType UserType `json:"user_type"`
	StoreID  string   `json:"store_id,omitempty"`
	jwt.RegisteredClaims
}

var (
	privateKey ed25519.PrivateKey
	publicKey  ed25519.PublicKey
)

// Init initialized keys from config inside an init function.
// For testing or manual override, use environment variables JWT_PRIVATE_KEY and JWT_PUBLIC_KEY.
func init() {
	privBase64 := config.Get("JWT_PRIVATE_KEY", "")
	pubBase64 := config.Get("JWT_PUBLIC_KEY", "")

	if privBase64 != "" {
		privBytes, err := base64.StdEncoding.DecodeString(privBase64)
		if err != nil {
			panic(fmt.Sprintf("jwt: failed to decode private key: %v", err))
		}
		privateKey = ed25519.PrivateKey(privBytes)
	}

	if pubBase64 != "" {
		pubBytes, err := base64.StdEncoding.DecodeString(pubBase64)
		if err != nil {
			panic(fmt.Sprintf("jwt: failed to decode public key: %v", err))
		}
		publicKey = ed25519.PublicKey(pubBytes)
	}
}

func generateJTI() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// GenerateAccessToken returns an EdDSA signed JWT token for 24h.
func GenerateAccessToken(claims ZippyraClaims) (string, error) {
	now := time.Now()
	claims.RegisteredClaims = jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(now.Add(24 * time.Hour)),
		IssuedAt:  jwt.NewNumericDate(now),
		ID:        generateJTI(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	signed, err := token.SignedString(privateKey)
	if err != nil {
		return "", fmt.Errorf("context: failed to sign token: %w", err)
	}
	return signed, nil
}

// GenerateRefreshToken returns an EdDSA signed refresh JWT for 30d.
func GenerateRefreshToken(userID string) (string, error) {
	now := time.Now()
	claims := ZippyraClaims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(30 * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        generateJTI(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	signed, err := token.SignedString(privateKey)
	if err != nil {
		return "", fmt.Errorf("context: failed to sign refresh token: %w", err)
	}
	return signed, nil
}

// ValidateToken decodes and verifies the signature using Ed25519.
func ValidateToken(tokenString string) (*ZippyraClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &ZippyraClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodEd25519); !ok {
			return nil, fmt.Errorf("context: unexpected signing method: %v", t.Header["alg"])
		}
		return publicKey, nil
	})

	if err != nil {
		return nil, fmt.Errorf("context: failed to parse token: %w", err)
	}

	if claims, ok := token.Claims.(*ZippyraClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("context: invalid token claims")
}

// BlacklistToken saves the given JTI into Redis to invalidate it.
func BlacklistToken(ctx context.Context, client redis.Cmdable, jti string, ttl time.Duration) error {
	key := "blacklist:" + jti
	err := client.SetNX(ctx, key, "1", ttl).Err()
	if err != nil {
		return fmt.Errorf("context: failed to blacklist token: %w", err)
	}
	return nil
}

// IsBlacklisted returns whether a JTI string is in the redis blacklist.
func IsBlacklisted(ctx context.Context, client redis.Cmdable, jti string) (bool, error) {
	key := "blacklist:" + jti
	res, err := client.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("context: failed to check token blacklist: %w", err)
	}
	return res == 1, nil
}

type ExitTokenClaims struct {
	OrderID string `json:"order_id"`
	UserID  string `json:"user_id"`
	StoreID string `json:"store_id"`
	jwt.RegisteredClaims
}

func GenerateExitToken(claims ExitTokenClaims) (string, error) {
	now := time.Now()
	claims.RegisteredClaims = jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(now.Add(10 * time.Minute)),
		IssuedAt:  jwt.NewNumericDate(now),
		ID:        generateJTI(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	signed, err := token.SignedString(privateKey)
	if err != nil {
		return "", fmt.Errorf("jwt: failed to sign exit token: %w", err)
	}
	return signed, nil
}

