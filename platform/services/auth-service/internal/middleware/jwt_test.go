package middleware

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"

	sharedErrors "github.com/zippyra/platform/shared/errors"
)

type errResp struct {
	Error struct {
		Code string `json:"code"`
	} `json:"error"`
}

func TestGenerateRefreshToken_Validates(t *testing.T) {
	rdb := newTestRedis(t)
	defer rdb.FlushDB(context.Background())
	pub, priv, _ := ed25519.GenerateKey(nil)
	mw := NewJWTMiddleware(base64.StdEncoding.EncodeToString(pub), base64.StdEncoding.EncodeToString(priv), rdb)

	tok, err := mw.GenerateRefreshToken("u1", "CUSTOMER", "+91XXXXXX0000", "d1")
	if err != nil {
		t.Fatal(err)
	}
	claims, err := mw.ValidateToken(tok)
	if err != nil {
		t.Fatal(err)
	}
	if claims.UserID != "u1" {
		t.Fatal("unexpected claims")
	}
}

func TestValidateToken_InvalidSigningMethod(t *testing.T) {
	rdb := newTestRedis(t)
	defer rdb.FlushDB(context.Background())
	pub, priv, _ := ed25519.GenerateKey(nil)
	mw := NewJWTMiddleware(base64.StdEncoding.EncodeToString(pub), base64.StdEncoding.EncodeToString(priv), rdb)

	claims := Claims{UserID: "u1", UserType: "CUSTOMER", Phone: "p", DeviceID: "d1"}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, _ := tok.SignedString([]byte("secret"))

	_, err := mw.ValidateToken(signed)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateToken_InvalidTokenString(t *testing.T) {
	rdb := newTestRedis(t)
	defer rdb.FlushDB(context.Background())
	pub, priv, _ := ed25519.GenerateKey(nil)
	mw := NewJWTMiddleware(base64.StdEncoding.EncodeToString(pub), base64.StdEncoding.EncodeToString(priv), rdb)

	_, err := mw.ValidateToken("not-a-jwt")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParsePublicKeys_EdgeCases(t *testing.T) {
	if _, err := parsePublicKeys(""); err == nil {
		t.Fatal("expected error")
	}
	if _, err := parsePublicKeys(","); err == nil {
		t.Fatal("expected error")
	}
	if _, err := parsePublicKeys("kidonly:"); err == nil {
		t.Fatal("expected error")
	}
	if _, err := parsePublicKeys(":abcd"); err == nil {
		t.Fatal("expected error")
	}
	if _, err := parsePublicKeys("kid:@@@@"); err == nil {
		t.Fatal("expected error")
	}
	if _, err := parsePublicKeys("a:b:c"); err == nil {
		t.Fatal("expected error")
	}
}

func TestParsePrivateKey_Empty(t *testing.T) {
	if _, _, err := parsePrivateKey(""); err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateToken_UnknownKID_ErrorPath(t *testing.T) {
	rdb := newTestRedis(t)
	defer rdb.FlushDB(context.Background())
	pub, priv, _ := ed25519.GenerateKey(nil)

	// Construct middleware with no default key and token with kid=other.
	mw := &JWTMiddleware{
		publicKeys: map[string]ed25519.PublicKey{"k1": pub},
		currentKID: "k1",
		privateKey: ed25519.PrivateKey(priv),
		rdb:        rdb,
	}

	claims := Claims{UserID: "u1", UserType: "CUSTOMER", Phone: "p", DeviceID: "d1", RegisteredClaims: jwt.RegisteredClaims{ID: "j", Issuer: "zippyra-auth", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))}}
	tok := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	tok.Header["kid"] = "other"
	signed, _ := tok.SignedString(ed25519.PrivateKey(priv))

	_, err := mw.ValidateToken(signed)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRequireCustomer_ExpiredToken(t *testing.T) {
	rdb := newTestRedis(t)
	defer rdb.FlushDB(context.Background())
	pub, priv, _ := ed25519.GenerateKey(nil)
	mw := NewJWTMiddleware(base64.StdEncoding.EncodeToString(pub), base64.StdEncoding.EncodeToString(priv), rdb)

	now := time.Now()
	claims := Claims{
		UserID:   "u1",
		UserType: "CUSTOMER",
		Phone:    "+91XXXXXX0000",
		DeviceID: "d1",
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        "jti-1",
			IssuedAt:  jwt.NewNumericDate(now.Add(-2 * time.Hour)),
			ExpiresAt: jwt.NewNumericDate(now.Add(-1 * time.Hour)),
			Issuer:    "zippyra-auth",
		},
	}

	tok := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	signed, err := tok.SignedString(ed25519.PrivateKey(priv))
	if err != nil {
		t.Fatal(err)
	}

	r := chi.NewRouter()
	r.Use(mw.RequireCustomer())
	r.Get("/x", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Bearer "+signed)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	var er errResp
	_ = json.NewDecoder(rec.Body).Decode(&er)
	if er.Error.Code != sharedErrors.ErrTokenExpired {
		t.Fatalf("expected %s, got %s", sharedErrors.ErrTokenExpired, er.Error.Code)
	}
}

func TestBlacklistToken_TTLExpired_NoOp(t *testing.T) {
	rdb := newTestRedis(t)
	defer rdb.FlushDB(context.Background())
	pub, priv, _ := ed25519.GenerateKey(nil)
	mw := NewJWTMiddleware(base64.StdEncoding.EncodeToString(pub), base64.StdEncoding.EncodeToString(priv), rdb)

	if err := mw.BlacklistToken(context.Background(), "jti", time.Now().Add(-1*time.Minute)); err != nil {
		t.Fatal(err)
	}
}

func TestIsBlacklisted_RedisError(t *testing.T) {
	failing := redis.NewClient(&redis.Options{Addr: "localhost:9999"})
	defer failing.Close()
	pub, priv, _ := ed25519.GenerateKey(nil)
	mw := NewJWTMiddleware(base64.StdEncoding.EncodeToString(pub), base64.StdEncoding.EncodeToString(priv), failing)

	_, err := mw.IsBlacklisted(context.Background(), "jti")
	if err == nil {
		t.Fatal("expected error")
	}
	if errors.Is(err, nil) {
		_ = err
	}
}

func TestNewJWTMiddleware_WrapperErrorPath_ReturnsNilWithoutExit(t *testing.T) {
	old := jwtFatal
	defer func() { jwtFatal = old }()
	jwtFatal = log.Error

	rdb := newTestRedis(t)
	defer rdb.FlushDB(context.Background())

	m := NewJWTMiddleware("bad", "bad", rdb)
	if m != nil {
		t.Fatal("expected nil")
	}
}

func newTestRedis(t *testing.T) *redis.Client {
	t.Helper()
	return redis.NewClient(&redis.Options{Addr: "localhost:6379", Password: "zippyra_local", DB: 1})
}

func TestNewJWTMiddlewareE_InvalidPublicKey(t *testing.T) {
	rdb := newTestRedis(t)
	defer rdb.FlushDB(context.Background())

	_, err := NewJWTMiddlewareE("not-base64", "not-base64", rdb)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNewJWTMiddlewareE_InvalidPrivateKey(t *testing.T) {
	rdb := newTestRedis(t)
	defer rdb.FlushDB(context.Background())

	pub, _, _ := ed25519.GenerateKey(nil)
	_, err := NewJWTMiddlewareE(base64.StdEncoding.EncodeToString(pub), "not-base64", rdb)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestJWT_KID_Rotation_SignsAndValidates(t *testing.T) {
	rdb := newTestRedis(t)
	defer rdb.FlushDB(context.Background())

	pub, priv, _ := ed25519.GenerateKey(nil)
	pubB64 := base64.StdEncoding.EncodeToString(pub)
	privB64 := base64.StdEncoding.EncodeToString(priv)

	mw, err := NewJWTMiddlewareE("k1:"+pubB64, "k1:"+privB64, rdb)
	if err != nil {
		t.Fatal(err)
	}

	tok, err := mw.GenerateAccessToken("u1", "CUSTOMER", "+91XXXXXX0000", "d1")
	if err != nil {
		t.Fatal(err)
	}

	claims, err := mw.ValidateToken(tok)
	if err != nil {
		t.Fatal(err)
	}
	if claims.UserID != "u1" || claims.DeviceID != "d1" {
		t.Fatal("unexpected claims")
	}
}

func TestRequireCustomer_MissingAuthHeader(t *testing.T) {
	rdb := newTestRedis(t)
	defer rdb.FlushDB(context.Background())
	pub, priv, _ := ed25519.GenerateKey(nil)
	mw := NewJWTMiddleware(base64.StdEncoding.EncodeToString(pub), base64.StdEncoding.EncodeToString(priv), rdb)

	r := chi.NewRouter()
	r.Use(mw.RequireCustomer())
	r.Get("/x", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	var er errResp
	_ = json.NewDecoder(rec.Body).Decode(&er)
	if er.Error.Code != sharedErrors.ErrUnauthorized {
		t.Fatalf("expected %s, got %s", sharedErrors.ErrUnauthorized, er.Error.Code)
	}
}

func TestRequireCustomer_WrongUserType(t *testing.T) {
	rdb := newTestRedis(t)
	defer rdb.FlushDB(context.Background())
	pub, priv, _ := ed25519.GenerateKey(nil)
	mw := NewJWTMiddleware(base64.StdEncoding.EncodeToString(pub), base64.StdEncoding.EncodeToString(priv), rdb)

	tok, _ := mw.GenerateAccessToken("u1", "STAFF", "+91XXXXXX0000", "d1")

	r := chi.NewRouter()
	r.Use(mw.RequireCustomer())
	r.Get("/x", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
	var er errResp
	_ = json.NewDecoder(rec.Body).Decode(&er)
	if er.Error.Code != sharedErrors.ErrWrongUserType {
		t.Fatalf("expected %s, got %s", sharedErrors.ErrWrongUserType, er.Error.Code)
	}
}

func TestRequireCustomer_Blacklisted(t *testing.T) {
	rdb := newTestRedis(t)
	defer rdb.FlushDB(context.Background())
	pub, priv, _ := ed25519.GenerateKey(nil)
	mw := NewJWTMiddleware(base64.StdEncoding.EncodeToString(pub), base64.StdEncoding.EncodeToString(priv), rdb)

	tok, _ := mw.GenerateAccessToken("u1", "CUSTOMER", "+91XXXXXX0000", "d1")
	claims, _ := mw.ValidateToken(tok)
	_ = mw.BlacklistToken(context.Background(), claims.ID, time.Now().Add(5*time.Minute))

	r := chi.NewRouter()
	r.Use(mw.RequireCustomer())
	r.Get("/x", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	var er errResp
	_ = json.NewDecoder(rec.Body).Decode(&er)
	if er.Error.Code != sharedErrors.ErrTokenBlacklisted {
		t.Fatalf("expected %s, got %s", sharedErrors.ErrTokenBlacklisted, er.Error.Code)
	}
}

func TestRequireCustomer_Success_SetsContext(t *testing.T) {
	rdb := newTestRedis(t)
	defer rdb.FlushDB(context.Background())
	pub, priv, _ := ed25519.GenerateKey(nil)
	mw := NewJWTMiddleware(base64.StdEncoding.EncodeToString(pub), base64.StdEncoding.EncodeToString(priv), rdb)

	tok, _ := mw.GenerateAccessToken("u1", "CUSTOMER", "+91XXXXXX0000", "d1")

	r := chi.NewRouter()
	r.Use(mw.RequireCustomer())
	r.Get("/x", func(w http.ResponseWriter, r *http.Request) {
		if GetUserIDFromContext(r.Context()) != "u1" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if GetDeviceIDFromContext(r.Context()) != "d1" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if GetJTIFromContext(r.Context()) == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}
