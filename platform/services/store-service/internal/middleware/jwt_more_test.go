package middleware

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// Additional JWT middleware tests for coverage

func TestNewJWTMiddlewareE_InvalidPublicKey(t *testing.T) {
	_, err := NewJWTMiddlewareE("invalid-base64", "", nil)
	if err == nil {
		t.Error("expected error for invalid public key")
	}
	// Ensure the non-error-returning constructor path is covered elsewhere.
}

func TestNewJWTMiddleware_FatalPath(t *testing.T) {
	origFatal := fatalLogger
	defer func() { fatalLogger = origFatal }()

	called := false
	fatalLogger = func() *zerolog.Event {
		called = true
		l := zerolog.New(io.Discard)
		return l.Error()
	}

	// invalid public key triggers fatal path
	m := NewJWTMiddleware("not-base64", "", &mockRedisJWTMiddleware{})
	if m != nil {
		t.Fatalf("expected nil middleware")
	}
	if !called {
		t.Fatalf("expected fatalLogger to be called")
	}
}

func TestValidateToken_UnknownKID_NoDefaultFallback(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	pubB64 := base64.StdEncoding.EncodeToString(pub)
	privB64 := base64.StdEncoding.EncodeToString(priv)

	mw, err := NewJWTMiddlewareE(pubB64, privB64, &mockRedisJWTMiddleware{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// create token with a KID that doesn't exist and no default fallback
	delete(mw.publicKeys, "default")
	claims := &Claims{UserID: "u", UserType: "CUSTOMER"}
	tok := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	tok.Header["kid"] = "missing"
	s, err := tok.SignedString(priv)
	if err != nil {
		t.Fatalf("sign failed: %v", err)
	}

	_, err = mw.ValidateToken(s)
	if err == nil || !strings.Contains(err.Error(), "unknown kid") {
		t.Fatalf("expected unknown kid error, got %v", err)
	}
}

func TestParsePublicKeys_InvalidEntry(t *testing.T) {
	// has colon but not key:value
	_, err := parsePublicKeys("kidonly:")
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestParsePublicKeys_InvalidEntry_NoColon(t *testing.T) {
	// comma-separated entries, one without ':' triggers len(kv) != 2 branch
	_, err := parsePublicKeys("kid:ZGVhZGJlZWY=,invalid")
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestNewJWTMiddlewareE_EmptyPublicKey(t *testing.T) {
	_, err := NewJWTMiddlewareE("", "", nil)
	if err == nil {
		t.Error("expected error for empty public key")
	}
}

func TestNewJWTMiddlewareE_InvalidPrivateKey(t *testing.T) {
	pub, _, _ := ed25519.GenerateKey(nil)
	pubB64 := base64.StdEncoding.EncodeToString(pub)

	_, err := NewJWTMiddlewareE(pubB64, "invalid-base64", nil)
	if err == nil {
		t.Error("expected error for invalid private key")
	}
}

func TestNewJWTMiddlewareE_WithKID(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	pubB64 := "v1:" + base64.StdEncoding.EncodeToString(pub)
	privB64 := "v1:" + base64.StdEncoding.EncodeToString(priv)

	m, err := NewJWTMiddlewareE(pubB64, privB64, &mockRedisJWTMiddleware{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.currentKID != "v1" {
		t.Errorf("expected currentKID 'v1', got '%s'", m.currentKID)
	}
}

func TestValidateToken_NonEd25519Algorithm(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	pubB64 := base64.StdEncoding.EncodeToString(pub)
	privB64 := base64.StdEncoding.EncodeToString(priv)

	m := NewJWTMiddleware(pubB64, privB64, &mockRedisJWTMiddleware{})

	// Create a token signed with a different method that we can parse
	claims := &Claims{
		UserID: "user-1",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, _ := token.SignedString([]byte("secret"))

	_, err := m.ValidateToken(tokenString)
	if err == nil {
		t.Error("expected error for non-Ed25519 algorithm")
	}
}

func TestValidateToken_UnknownKID(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	pubB64 := base64.StdEncoding.EncodeToString(pub)
	privB64 := base64.StdEncoding.EncodeToString(priv)

	m := NewJWTMiddleware(pubB64, privB64, &mockRedisJWTMiddleware{})

	// Create token with unknown kid in header
	claims := &Claims{
		UserID: "user-1",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	token.Header["kid"] = "unknown-kid"
	tokenString, _ := token.SignedString(priv)

	// Should fallback to default key and succeed
	validated, err := m.ValidateToken(tokenString)
	if err != nil {
		t.Fatalf("expected success with default key fallback, got: %v", err)
	}
	if validated.UserID != "user-1" {
		t.Errorf("expected user_id 'user-1', got '%s'", validated.UserID)
	}
}

func TestValidateToken_NoMatchingKID_FallsBackToDefault(t *testing.T) {
	// Create middleware with a key - parsePublicKeys always sets a default key
	pub, priv, _ := ed25519.GenerateKey(nil)
	pubB64 := "v1:" + base64.StdEncoding.EncodeToString(pub)
	privB64 := "v1:" + base64.StdEncoding.EncodeToString(priv)

	m, err := NewJWTMiddlewareE(pubB64, privB64, &mockRedisJWTMiddleware{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Create token with kid that doesn't exist in publicKeys
	claims := &Claims{
		UserID: "user-1",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	token.Header["kid"] = "nonexistent"
	tokenString, _ := token.SignedString(priv)

	// Should succeed by falling back to default key (which is set to v1 key)
	validated, err := m.ValidateToken(tokenString)
	if err != nil {
		t.Errorf("expected success with default key fallback, got: %v", err)
	}
	if validated.UserID != "user-1" {
		t.Errorf("expected user_id 'user-1', got '%s'", validated.UserID)
	}
}

func TestRequireAuth_BlacklistError(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	pubB64 := base64.StdEncoding.EncodeToString(pub)
	privB64 := base64.StdEncoding.EncodeToString(priv)

	// Redis that returns error on Exists check
	redis := &mockRedisJWTMiddleware{err: errors.New("redis error")}
	m := NewJWTMiddleware(pubB64, privB64, redis)

	claims := &Claims{
		UserID:   uuid.New().String(),
		UserType: "CUSTOMER",
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.New().String(),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	tokenString, _ := token.SignedString(priv)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokenString)
	w := httptest.NewRecorder()

	m.RequireAuth()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called when blacklist check fails")
	})).ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 for blacklist error, got %d", w.Code)
	}
}

func TestRequireAuth_InvalidTokenFormat(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	pubB64 := base64.StdEncoding.EncodeToString(pub)
	privB64 := base64.StdEncoding.EncodeToString(priv)

	m := NewJWTMiddleware(pubB64, privB64, &mockRedisJWTMiddleware{})

	// Test missing "Bearer " prefix
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "invalid-token-format")
	w := httptest.NewRecorder()

	m.RequireAuth()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})).ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for invalid token format, got %d", w.Code)
	}
}

func TestRequireAuth_ExpiredToken(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	pubB64 := base64.StdEncoding.EncodeToString(pub)
	privB64 := base64.StdEncoding.EncodeToString(priv)

	m := NewJWTMiddleware(pubB64, privB64, &mockRedisJWTMiddleware{})

	claims := &Claims{
		UserID:   uuid.New().String(),
		UserType: "CUSTOMER",
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.New().String(),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)), // Expired
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	tokenString, _ := token.SignedString(priv)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokenString)
	w := httptest.NewRecorder()

	m.RequireAuth()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})).ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for expired token, got %d", w.Code)
	}
}

func TestContextHelpers_NonStringValues(t *testing.T) {
	// Test that non-string values return empty string
	ctx := context.WithValue(context.Background(), ContextUserID, 123)
	if GetUserIDFromContext(ctx) != "" {
		t.Error("expected empty string for non-string user_id")
	}

	ctx = context.WithValue(context.Background(), ContextUserType, 123)
	if GetUserTypeFromContext(ctx) != "" {
		t.Error("expected empty string for non-string user_type")
	}

	ctx = context.WithValue(context.Background(), ContextChainID, 123)
	if GetChainIDFromContext(ctx) != "" {
		t.Error("expected empty string for non-string chain_id")
	}

	ctx = context.WithValue(context.Background(), ContextStoreID, 123)
	if GetStoreIDFromContext(ctx) != "" {
		t.Error("expected empty string for non-string store_id")
	}
}

func TestParsePublicKeys_InvalidBase64(t *testing.T) {
	_, err := parsePublicKeys("not-valid-base64!!!")
	if err == nil {
		t.Error("expected error for invalid base64")
	}
}

func TestParsePublicKeys_InvalidEntryFormat(t *testing.T) {
	// Missing colon separator
	_, err := parsePublicKeys("kidvalue-without-colon")
	if err == nil {
		t.Error("expected error for invalid entry format")
	}
}

func TestParsePublicKeys_InvalidKIDValue(t *testing.T) {
	// Empty kid or value
	_, err := parsePublicKeys(":value")
	if err == nil {
		t.Error("expected error for empty kid")
	}

	_, err = parsePublicKeys("kid:")
	if err == nil {
		t.Error("expected error for empty value")
	}
}

func TestParsePublicKeys_InvalidBase64InKIDValue(t *testing.T) {
	// Invalid base64 in kid:value pair
	_, err := parsePublicKeys("kid:not-valid-base64!!!")
	if err == nil {
		t.Error("expected error for invalid base64 in kid:value")
	}
}

func TestParsePublicKeys_NoKeysAfterFiltering(t *testing.T) {
	// Only empty entries after splitting
	_, err := parsePublicKeys("  ,  ,  ")
	if err == nil {
		t.Error("expected error when no valid keys after filtering")
	}
}

func TestParsePublicKeys_MultipleKeys(t *testing.T) {
	pub1, _, _ := ed25519.GenerateKey(nil)
	pub2, _, _ := ed25519.GenerateKey(nil)
	b64_1 := base64.StdEncoding.EncodeToString(pub1)
	b64_2 := base64.StdEncoding.EncodeToString(pub2)

	input := "v1:" + b64_1 + ", v2:" + b64_2
	keys, err := parsePublicKeys(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(keys) != 3 { // v1, v2, and default (set to first key)
		t.Errorf("expected 3 keys, got %d", len(keys))
	}

	if _, ok := keys["v1"]; !ok {
		t.Error("expected key 'v1' to exist")
	}
	if _, ok := keys["v2"]; !ok {
		t.Error("expected key 'v2' to exist")
	}
	if _, ok := keys["default"]; !ok {
		t.Error("expected key 'default' to exist")
	}
}

func TestParsePrivateKey_Empty(t *testing.T) {
	_, _, err := parsePrivateKey("")
	if err == nil {
		t.Error("expected error for empty private key")
	}
}

func TestParsePrivateKey_OnlyWhitespace(t *testing.T) {
	_, _, err := parsePrivateKey("   ")
	if err == nil {
		t.Error("expected error for whitespace-only private key")
	}
}

func TestParsePrivateKey_InvalidBase64(t *testing.T) {
	_, _, err := parsePrivateKey("not-valid-base64!!!")
	if err == nil {
		t.Error("expected error for invalid base64")
	}
}

func TestParsePrivateKey_InvalidKIDFormat(t *testing.T) {
	// Empty kid with colon
	_, _, err := parsePrivateKey(":value")
	if err == nil {
		t.Error("expected error for empty kid")
	}

	// Empty value with colon
	_, _, err = parsePrivateKey("key:")
	if err == nil {
		t.Error("expected error for empty value after colon")
	}
}

func TestParsePrivateKey_ValidWithKID(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(nil)
	b64 := base64.StdEncoding.EncodeToString(priv)

	kid, bytes, err := parsePrivateKey("v1:" + b64)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if kid != "v1" {
		t.Errorf("expected kid 'v1', got '%s'", kid)
	}
	if len(bytes) == 0 {
		t.Error("expected non-empty bytes")
	}
}

func TestParsePrivateKey_ValidWithoutKID(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(nil)
	b64 := base64.StdEncoding.EncodeToString(priv)

	kid, bytes, err := parsePrivateKey(b64)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if kid != "" {
		t.Errorf("expected empty kid, got '%s'", kid)
	}
	if len(bytes) == 0 {
		t.Error("expected non-empty bytes")
	}
}

func TestParsePrivateKey_InvalidBase64WithKID(t *testing.T) {
	_, _, err := parsePrivateKey("kid:not-valid-base64!!!")
	if err == nil {
		t.Error("expected error for invalid base64 with kid")
	}
}
