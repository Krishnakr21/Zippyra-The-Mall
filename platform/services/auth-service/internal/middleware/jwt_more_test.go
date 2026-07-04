package middleware

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"

	sharedErrors "github.com/zippyra/platform/shared/errors"
)

type errResp2 struct {
	Error struct {
		Code string `json:"code"`
	} `json:"error"`
}

func TestContextGetters_Fallbacks_OnlyExisting(t *testing.T) {
	ctx := context.Background()
	if GetUserIDFromContext(ctx) != "" {
		t.Fatal("expected empty")
	}
	if GetDeviceIDFromContext(ctx) != "" {
		t.Fatal("expected empty")
	}
	if GetJTIFromContext(ctx) != "" {
		t.Fatal("expected empty")
	}
}

func TestRequireStaff_InvalidBearerFormat(t *testing.T) {
	rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379", Password: "zippyra_local", DB: 1})
	defer rdb.FlushDB(context.Background())
	pub, priv, _ := ed25519.GenerateKey(nil)
	mw := NewJWTMiddleware(base64.StdEncoding.EncodeToString(pub), base64.StdEncoding.EncodeToString(priv), rdb)

	r := chi.NewRouter()
	r.Use(mw.RequireStaff())
	r.Get("/x", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Bearer")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	var er errResp2
	_ = json.NewDecoder(rec.Body).Decode(&er)
	if er.Error.Code != sharedErrors.ErrTokenInvalid {
		t.Fatalf("expected %s, got %s", sharedErrors.ErrTokenInvalid, er.Error.Code)
	}
}

func TestValidateToken_InvalidSignatureResultsInTokenInvalid(t *testing.T) {
	rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379", Password: "zippyra_local", DB: 1})
	defer rdb.FlushDB(context.Background())
	pub, priv, _ := ed25519.GenerateKey(nil)
	mw := NewJWTMiddleware(base64.StdEncoding.EncodeToString(pub), base64.StdEncoding.EncodeToString(priv), rdb)

	claims := Claims{UserID: "u1", UserType: "CUSTOMER", Phone: "p", DeviceID: "d1", RegisteredClaims: jwt.RegisteredClaims{ID: "j", Issuer: "zippyra-auth", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))}}
	tok := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	// Sign with a different private key so signature check fails
	_, otherPriv, _ := ed25519.GenerateKey(nil)
	signed, _ := tok.SignedString(ed25519.PrivateKey(otherPriv))

	_, err := mw.ValidateToken(signed)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParsePublicKeys_InvalidEntryLen(t *testing.T) {
	pub, _, _ := ed25519.GenerateKey(nil)
	pubB64 := base64.StdEncoding.EncodeToString(pub)
	// second entry has no ':' -> triggers len(kv)!=2 branch
	_, err := parsePublicKeys("k1:" + pubB64 + ",bad")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateToken_UnknownKID_FallsBackToDefault(t *testing.T) {
	rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379", Password: "zippyra_local", DB: 1})
	defer rdb.FlushDB(context.Background())
	pub, priv, _ := ed25519.GenerateKey(nil)
	pubB64 := base64.StdEncoding.EncodeToString(pub)
	privB64 := base64.StdEncoding.EncodeToString(priv)

	// public keys only as default; token will be issued with kid=other
	verifier, err := NewJWTMiddlewareE(pubB64, privB64, rdb)
	if err != nil {
		t.Fatal(err)
	}
	issuer, err := NewJWTMiddlewareE(pubB64, "other:"+privB64, rdb)
	if err != nil {
		t.Fatal(err)
	}

	tok, err := issuer.GenerateAccessToken("u1", "CUSTOMER", "p", "d1")
	if err != nil {
		t.Fatal(err)
	}

	_, err = verifier.ValidateToken(tok)
	if err != nil {
		t.Fatal(err)
	}
}

func TestRequireStaff_InvalidToken_NonExpired(t *testing.T) {
	rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379", Password: "zippyra_local", DB: 1})
	defer rdb.FlushDB(context.Background())
	pub, priv, _ := ed25519.GenerateKey(nil)
	mw := NewJWTMiddleware(base64.StdEncoding.EncodeToString(pub), base64.StdEncoding.EncodeToString(priv), rdb)

	r := chi.NewRouter()
	r.Use(mw.RequireStaff())
	r.Get("/x", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Bearer not-a-jwt")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	var er errResp2
	_ = json.NewDecoder(rec.Body).Decode(&er)
	if er.Error.Code != sharedErrors.ErrTokenInvalid {
		t.Fatalf("expected %s, got %s", sharedErrors.ErrTokenInvalid, er.Error.Code)
	}
}

func TestRequireStaff_BlacklistCheckError_Returns500(t *testing.T) {
	failing := redis.NewClient(&redis.Options{Addr: "localhost:9999"})
	defer failing.Close()
	pub, priv, _ := ed25519.GenerateKey(nil)
	mw := NewJWTMiddleware(base64.StdEncoding.EncodeToString(pub), base64.StdEncoding.EncodeToString(priv), failing)

	tok, _ := mw.GenerateAccessToken("u1", "STAFF", "p", "d1")

	r := chi.NewRouter()
	r.Use(mw.RequireStaff())
	r.Get("/x", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}
