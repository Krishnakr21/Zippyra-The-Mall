package middleware

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// ── Mock Redis ──────────────────────────────────────────────────────

type mockRedisJWTMiddleware struct {
	exists bool
	err    error
}

func (m *mockRedisJWTMiddleware) Get(ctx context.Context, key string) (string, error) { return "", nil }
func (m *mockRedisJWTMiddleware) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	return nil
}
func (m *mockRedisJWTMiddleware) Incr(ctx context.Context, key string) (int64, error) { return 0, nil }
func (m *mockRedisJWTMiddleware) Decr(ctx context.Context, key string) (int64, error) { return 0, nil }
func (m *mockRedisJWTMiddleware) Del(ctx context.Context, key string) error           { return nil }
func (m *mockRedisJWTMiddleware) Exists(ctx context.Context, key string) (bool, error) {
	return m.exists, m.err
}

// ── Tests ───────────────────────────────────────────────────────────

func TestJWTMiddleware_RequireAuth(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	pubB64 := base64.StdEncoding.EncodeToString(pub)
	privB64 := base64.StdEncoding.EncodeToString(priv)

	m := NewJWTMiddleware(pubB64, privB64, &mockRedisJWTMiddleware{})

	// 1. Success case
	userID := uuid.New().String()
	claims := &Claims{
		UserID:   userID,
		UserType: "CUSTOMER",
		ChainID:  uuid.New().String(),
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

	success := false
	m.RequireAuth()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if GetUserIDFromContext(r.Context()) == userID {
			success = true
		}
	})).ServeHTTP(w, req)

	if !success {
		t.Error("expected context to have user_id")
	}

	// 2. Missing header
	req = httptest.NewRequest("GET", "/", nil)
	w = httptest.NewRecorder()
	m.RequireAuth()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})).ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for missing header, got %d", w.Code)
	}

	// 3. Invalid token
	req = httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	w = httptest.NewRecorder()
	m.RequireAuth()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})).ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for invalid token, got %d", w.Code)
	}
}

func TestJWTMiddleware_UserType(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	pubB64 := base64.StdEncoding.EncodeToString(pub)
	privB64 := base64.StdEncoding.EncodeToString(priv)

	m := NewJWTMiddleware(pubB64, privB64, &mockRedisJWTMiddleware{})

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

	// 1. RequireCustomer (Success)
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokenString)
	w := httptest.NewRecorder()
	m.RequireCustomer()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	// 2. RequireStaff (Failure)
	req = httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokenString)
	w = httptest.NewRecorder()
	m.RequireStaff()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})).ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for user type mismatch, got %d", w.Code)
	}
}

func TestJWTMiddleware_Blacklist(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	pubB64 := base64.StdEncoding.EncodeToString(pub)
	privB64 := base64.StdEncoding.EncodeToString(priv)

	m := NewJWTMiddleware(pubB64, privB64, &mockRedisJWTMiddleware{exists: true})

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

	m.RequireAuth()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})).ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for blacklisted token, got %d", w.Code)
	}
}

func TestJWTMiddleware_ContextHelpers(t *testing.T) {
	ctx := context.Background()
	if GetUserIDFromContext(ctx) != "" {
		t.Error("expected empty user_id for empty context")
	}

	userID := "user-123"
	ctx = context.WithValue(ctx, ContextUserID, userID)
	if GetUserIDFromContext(ctx) != userID {
		t.Errorf("expected %s, got %s", userID, GetUserIDFromContext(ctx))
	}

	userType := "STAFF"
	ctx = context.WithValue(ctx, ContextUserType, userType)
	if GetUserTypeFromContext(ctx) != userType {
		t.Errorf("expected %s, got %s", userType, GetUserTypeFromContext(ctx))
	}

	chainID := "chain-456"
	ctx = context.WithValue(ctx, ContextChainID, chainID)
	if GetChainIDFromContext(ctx) != chainID {
		t.Errorf("expected %s, got %s", chainID, GetChainIDFromContext(ctx))
	}

	storeID := "store-789"
	ctx = context.WithValue(ctx, ContextStoreID, storeID)
	if GetStoreIDFromContext(ctx) != storeID {
		t.Errorf("expected %s, got %s", storeID, GetStoreIDFromContext(ctx))
	}
}
func TestJWTMiddleware_InitializationFail(t *testing.T) {
	// Mock fatalLogger
	oldFatal := fatalLogger
	defer func() { fatalLogger = oldFatal }()
	
	fatalCalled := false
	fatalLogger = func() *zerolog.Event {
		fatalCalled = true
		return log.Logger.Error() // Use Error instead of Fatal to avoid os.Exit
	}

	// Invalid base64
	m := NewJWTMiddleware("invalid-b64", "invalid-b64", &mockRedisJWTMiddleware{})
	if m != nil {
		t.Error("expected nil on init failure")
	}
	if !fatalCalled {
		t.Error("expected fatalLogger to be called")
	}
}

func TestJWTMiddleware_BlacklistError(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	pubB64 := base64.StdEncoding.EncodeToString(pub)
	privB64 := base64.StdEncoding.EncodeToString(priv)

	m := NewJWTMiddleware(pubB64, privB64, &mockRedisJWTMiddleware{err: context.DeadlineExceeded})

	claims := &Claims{
		UserID: uuid.New().String(),
		RegisteredClaims: jwt.RegisteredClaims{ID: "jti-123", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	tokenString, _ := token.SignedString(priv)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokenString)
	w := httptest.NewRecorder()

	m.RequireAuth()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})).ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 on blacklist lookup error, got %d", w.Code)
	}
}

func TestJWTMiddleware_InvalidSigningMethod(t *testing.T) {
	pub, _, _ := ed25519.GenerateKey(nil)
	pubB64 := base64.StdEncoding.EncodeToString(pub)
	
	m := NewJWTMiddleware(pubB64, "priv", &mockRedisJWTMiddleware{})
	
	// Create token with HS256
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, &Claims{})
	tokenString, _ := token.SignedString([]byte("secret"))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokenString)
	w := httptest.NewRecorder()

	m.RequireAuth()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})).ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 on invalid signing method, got %d", w.Code)
	}
}

func TestJWTMiddleware_MalformedHeader(t *testing.T) {
	m := &JWTMiddleware{}
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "InvalidFormat")
	w := httptest.NewRecorder()
	
	m.RequireAuth()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})).ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for malformed header, got %d", w.Code)
	}
}
