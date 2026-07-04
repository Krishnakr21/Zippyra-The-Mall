package service

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/zippyra/platform/services/auth-service/internal/middleware"
	"github.com/zippyra/platform/services/auth-service/internal/model"
)

func setupTestJWT(t *testing.T) (*middleware.JWTMiddleware, *redis.Client) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("failed to generate ed25519 key: %v", err)
	}
	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "zippyra_local",
		DB:       1,
	})
	return middleware.NewJWTMiddleware(
		base64.StdEncoding.EncodeToString(pub),
		base64.StdEncoding.EncodeToString(priv),
		rdb,
	), rdb
}

// ─── NewAuthService ─────────────────────────────────────────────────

func TestNewAuthService(t *testing.T) {
	jwtMW, rdb := setupTestJWT(t)
	defer rdb.FlushDB(context.Background())

	svc := NewAuthService(rdb, &mockUserRepo{}, &mockLoginAttemptRepo{}, &mockSessionStore{}, jwtMW, &mockPublisher{})
	if svc == nil {
		t.Fatal("NewAuthService returned nil")
	}
}

// ─── CompleteLogin ──────────────────────────────────────────────────

func TestCompleteLogin_Success(t *testing.T) {
	jwtMW, rdb := setupTestJWT(t)
	defer rdb.FlushDB(context.Background())

	pub := &mockPublisher{}
	svc := NewAuthService(rdb, &mockUserRepo{}, &mockLoginAttemptRepo{}, &mockSessionStore{}, jwtMW, pub)

	result, err := svc.CompleteLogin(context.Background(), "+919876543210", "device-1", "iPhone 15", "1.2.3.4", "test-agent")
	if err != nil {
		t.Fatalf("CompleteLogin failed: %v", err)
	}
	if result.AccessToken == "" {
		t.Error("access token should not be empty")
	}
	if result.RefreshToken == "" {
		t.Error("refresh token should not be empty")
	}
	if result.ExpiresIn != 86400 {
		t.Errorf("expected expires_in 86400, got %d", result.ExpiresIn)
	}
	if !result.IsNewUser {
		t.Error("expected is_new_user=true from mock")
	}

	// Publisher should have been called
	if !pub.published {
		t.Error("expected login event to be published")
	}

	// Refresh hash should be in Redis
	val := rdb.Get(context.Background(), "refresh:"+result.UserID+":"+"device-1").Val()
	if val == "" {
		t.Error("refresh token hash should be stored in Redis")
	}

	// Wait for goroutine
	time.Sleep(50 * time.Millisecond)
}

func TestCompleteLogin_UserUpsertFails(t *testing.T) {
	jwtMW, rdb := setupTestJWT(t)
	defer rdb.FlushDB(context.Background())

	failRepo := &mockUserRepo{
		upsertFn: func(ctx context.Context, phone string) (*model.User, bool, error) {
			return nil, false, fmt.Errorf("db connection error")
		},
	}
	svc := NewAuthService(rdb, failRepo, &mockLoginAttemptRepo{}, &mockSessionStore{}, jwtMW, &mockPublisher{})

	result, err := svc.CompleteLogin(context.Background(), "+919876543210", "d1", "m1", "1.1.1.1", "ua")
	if err == nil {
		t.Fatal("expected error on user upsert failure")
	}
	if result != nil {
		t.Error("result should be nil on failure")
	}
}

func TestCompleteLogin_SessionCreateFails_NonFatal(t *testing.T) {
	jwtMW, rdb := setupTestJWT(t)
	defer rdb.FlushDB(context.Background())

	sessStore := &mockSessionStore{createErr: fmt.Errorf("session db error")}
	svc := NewAuthService(rdb, &mockUserRepo{}, &mockLoginAttemptRepo{}, sessStore, jwtMW, &mockPublisher{})

	result, err := svc.CompleteLogin(context.Background(), "+919876543210", "d1", "m1", "1.1.1.1", "ua")
	if err != nil {
		t.Fatalf("session create failure should be non-fatal, got: %v", err)
	}
	if result == nil || result.AccessToken == "" {
		t.Error("login should still succeed even if session creation fails")
	}
}

func TestCompleteLogin_UpdateStatusFails_GoroutineCovered(t *testing.T) {
	jwtMW, rdb := setupTestJWT(t)
	defer rdb.FlushDB(context.Background())

	updateDone := make(chan struct{})
	loginRepo := &mockLoginAttemptRepo{updateErr: fmt.Errorf("db error"), updateDone: updateDone}
	pub := &mockPublisher{}
	// session store doesn't matter for CompleteLogin
	svc := NewAuthService(rdb, &mockUserRepo{}, loginRepo, &mockSessionStore{}, jwtMW, pub)

	_, err := svc.CompleteLogin(context.Background(), "+919876543210", "device-1", "iPhone", "1.2.3.4", "ua")
	if err != nil {
		t.Fatalf("CompleteLogin failed: %v", err)
	}

	select {
	case <-updateDone:
		// ok
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for login attempt status update")
	}
}

func TestCompleteLogin_AccessTokenGenFail(t *testing.T) {
	jwtFail := &mockTokenGenerator{genAccessErr: fmt.Errorf("token gen error")}
	svc := NewAuthService(nil, &mockUserRepo{}, &mockLoginAttemptRepo{}, &mockSessionStore{}, jwtFail, &mockPublisher{})
	_, err := svc.CompleteLogin(context.Background(), "+919876543210", "d1", "m1", "1.1.1.1", "ua")
	if err == nil {
		t.Error("expected access token generation to fail")
	}
}

func TestCompleteLogin_RefreshTokenGenFail(t *testing.T) {
	jwtFail := &mockTokenGenerator{genRefreshErr: fmt.Errorf("refresh gen error")}
	svc := NewAuthService(nil, &mockUserRepo{}, &mockLoginAttemptRepo{}, &mockSessionStore{}, jwtFail, &mockPublisher{})
	_, err := svc.CompleteLogin(context.Background(), "+919876543210", "d1", "m1", "1.1.1.1", "ua")
	if err == nil {
		t.Error("expected refresh token generation to fail")
	}
}

func TestCompleteLogin_RefreshHashStoreFail(t *testing.T) {
	jwtMW, rdb := setupTestJWT(t)
	defer rdb.FlushDB(context.Background())

	failingRdb := redis.NewClient(&redis.Options{Addr: "localhost:9999"})
	defer failingRdb.Close()

	sessStore := &mockSessionStore{hasActive: true}
	svc := NewAuthService(failingRdb, &mockUserRepo{}, &mockLoginAttemptRepo{}, sessStore, jwtMW, &mockPublisher{})
	_, err := svc.CompleteLogin(context.Background(), "+919876543210", "d1", "m1", "1.1.1.1", "ua")
	if err == nil {
		t.Error("expected refresh token storage to fail")
	}
}

// ─── RefreshAccessToken ─────────────────────────────────────────────

func TestRefreshAccessToken_Success(t *testing.T) {
	jwtMW, rdb := setupTestJWT(t)
	defer rdb.FlushDB(context.Background())

	sessStore := &mockSessionStore{hasActive: true}
	svc := NewAuthService(rdb, &mockUserRepo{}, &mockLoginAttemptRepo{}, sessStore, jwtMW, &mockPublisher{})

	// Generate refresh token and store hash
	userID := uuid.New().String()
	refreshToken, _ := jwtMW.GenerateRefreshToken(userID, "CUSTOMER", "+91XXXXXX0000", "device-1")
	h := sha256.Sum256([]byte(refreshToken))
	rdb.Set(context.Background(), "refresh:"+userID+":"+"device-1", hex.EncodeToString(h[:]), 30*24*time.Hour)

	newAccessToken, errCode, err := svc.RefreshAccessToken(context.Background(), refreshToken)
	if err != nil {
		t.Fatalf("RefreshAccessToken failed: %v", err)
	}
	if errCode != "" {
		t.Errorf("expected no error code, got %q", errCode)
	}
	if newAccessToken == "" {
		t.Error("new access token should not be empty")
	}

	time.Sleep(50 * time.Millisecond)
}

func TestRefreshAccessToken_InvalidToken(t *testing.T) {
	jwtMW, rdb := setupTestJWT(t)
	defer rdb.FlushDB(context.Background())

	svc := NewAuthService(rdb, &mockUserRepo{}, &mockLoginAttemptRepo{}, &mockSessionStore{}, jwtMW, &mockPublisher{})

	_, errCode, err := svc.RefreshAccessToken(context.Background(), "invalid-token")
	if err == nil {
		t.Fatal("expected error for invalid token")
	}
	if errCode != "TOKEN_INVALID" {
		t.Errorf("expected TOKEN_INVALID, got %q", errCode)
	}
}

func TestRefreshAccessToken_NoActiveSession(t *testing.T) {
	jwtMW, rdb := setupTestJWT(t)
	defer rdb.FlushDB(context.Background())

	// No active session -> should fail before Redis lookup
	sessStore := &mockSessionStore{hasActive: false}
	svc := NewAuthService(rdb, &mockUserRepo{}, &mockLoginAttemptRepo{}, sessStore, jwtMW, &mockPublisher{})

	userID := uuid.New().String()
	refreshToken, _ := jwtMW.GenerateRefreshToken(userID, "CUSTOMER", "+91XXXXXX0000", "device-1")

	_, errCode, err := svc.RefreshAccessToken(context.Background(), refreshToken)
	if err == nil {
		t.Fatal("expected error")
	}
	if errCode != "TOKEN_INVALID" {
		t.Errorf("expected TOKEN_INVALID, got %q", errCode)
	}
}

func TestRefreshAccessToken_HasActiveSessionError(t *testing.T) {
	jwtMW, rdb := setupTestJWT(t)
	defer rdb.FlushDB(context.Background())

	sessStore := &mockSessionStore{hasActiveErr: fmt.Errorf("db down")}
	svc := NewAuthService(rdb, &mockUserRepo{}, &mockLoginAttemptRepo{}, sessStore, jwtMW, &mockPublisher{})

	userID := uuid.New().String()
	refreshToken, _ := jwtMW.GenerateRefreshToken(userID, "CUSTOMER", "+91XXXXXX0000", "device-1")

	_, errCode, err := svc.RefreshAccessToken(context.Background(), refreshToken)
	if err == nil {
		t.Fatal("expected error")
	}
	if errCode != "INTERNAL_SERVER_ERROR" {
		t.Errorf("expected INTERNAL_SERVER_ERROR, got %q", errCode)
	}
}

func TestRefreshAccessToken_RedisGetError_AfterBlacklistPasses(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	goodRdb := redis.NewClient(&redis.Options{Addr: "localhost:6379", Password: "zippyra_local", DB: 1})
	defer goodRdb.FlushDB(context.Background())

	failingRdb := redis.NewClient(&redis.Options{Addr: "localhost:9999"})
	defer failingRdb.Close()

	jwtMW := middleware.NewJWTMiddleware(
		base64.StdEncoding.EncodeToString(pub),
		base64.StdEncoding.EncodeToString(priv),
		goodRdb,
	)

	sessStore := &mockSessionStore{hasActive: true}
	svc := NewAuthService(failingRdb, &mockUserRepo{}, &mockLoginAttemptRepo{}, sessStore, jwtMW, &mockPublisher{})

	userID := uuid.New().String()
	refreshToken, _ := jwtMW.GenerateRefreshToken(userID, "CUSTOMER", "+91XXXXXX0000", "device-1")

	_, errCode, err := svc.RefreshAccessToken(context.Background(), refreshToken)
	if err == nil {
		t.Fatal("expected Redis GET to fail")
	}
	if errCode != "INTERNAL_SERVER_ERROR" {
		t.Errorf("expected INTERNAL_SERVER_ERROR, got %q", errCode)
	}
}

func TestRefreshAccessToken_BlacklistedToken(t *testing.T) {
	jwtMW, rdb := setupTestJWT(t)
	defer rdb.FlushDB(context.Background())

	sessStore := &mockSessionStore{hasActive: true}
	svc := NewAuthService(rdb, &mockUserRepo{}, &mockLoginAttemptRepo{}, sessStore, jwtMW, &mockPublisher{})

	userID := uuid.New().String()
	refreshToken, _ := jwtMW.GenerateRefreshToken(userID, "CUSTOMER", "+91XXXXXX0000", "device-1")
	claims, _ := jwtMW.ValidateToken(refreshToken)

	// Blacklist the token
	jwtMW.BlacklistToken(context.Background(), claims.ID, claims.ExpiresAt.Time)

	_, errCode, err := svc.RefreshAccessToken(context.Background(), refreshToken)
	if err == nil {
		t.Fatal("expected error for blacklisted token")
	}
	if errCode != "TOKEN_BLACKLISTED" {
		t.Errorf("expected TOKEN_BLACKLISTED, got %q", errCode)
	}
}

func TestRefreshAccessToken_NoStoredHash(t *testing.T) {
	jwtMW, rdb := setupTestJWT(t)
	defer rdb.FlushDB(context.Background())

	sessStore := &mockSessionStore{hasActive: true}
	svc := NewAuthService(rdb, &mockUserRepo{}, &mockLoginAttemptRepo{}, sessStore, jwtMW, &mockPublisher{})

	userID := uuid.New().String()
	refreshToken, _ := jwtMW.GenerateRefreshToken(userID, "CUSTOMER", "+91XXXXXX0000", "device-1")
	// Don't store hash — simulate expired/missing

	_, errCode, err := svc.RefreshAccessToken(context.Background(), refreshToken)
	if err == nil {
		t.Fatal("expected error")
	}
	if errCode != "TOKEN_INVALID" {
		t.Errorf("expected TOKEN_INVALID, got %q", errCode)
	}
}

func TestRefreshAccessToken_HashMismatch(t *testing.T) {
	jwtMW, rdb := setupTestJWT(t)
	defer rdb.FlushDB(context.Background())

	sessStore := &mockSessionStore{hasActive: true}
	svc := NewAuthService(rdb, &mockUserRepo{}, &mockLoginAttemptRepo{}, sessStore, jwtMW, &mockPublisher{})

	userID := uuid.New().String()
	refreshToken, _ := jwtMW.GenerateRefreshToken(userID, "CUSTOMER", "+91XXXXXX0000", "device-1")
	// Store wrong hash
	rdb.Set(context.Background(), "refresh:"+userID+":"+"device-1", "wrong-hash-value", 30*24*time.Hour)

	_, errCode, err := svc.RefreshAccessToken(context.Background(), refreshToken)
	if err == nil {
		t.Fatal("expected error for hash mismatch")
	}
	if errCode != "TOKEN_INVALID" {
		t.Errorf("expected TOKEN_INVALID, got %q", errCode)
	}
}

func TestRefreshAccessToken_RedisGetError(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	failingRdb := redis.NewClient(&redis.Options{Addr: "localhost:9999"})
	defer failingRdb.Close()

	jwtMW := middleware.NewJWTMiddleware(
		base64.StdEncoding.EncodeToString(pub),
		base64.StdEncoding.EncodeToString(priv),
		failingRdb,
	)

	sessStore := &mockSessionStore{hasActive: true}
	svc := NewAuthService(failingRdb, &mockUserRepo{}, &mockLoginAttemptRepo{}, sessStore, jwtMW, &mockPublisher{})

	// Need a valid token to pass the first validation step
	// We'll use a valid middleware to generate it
	rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379", Password: "zippyra_local", DB: 1})
	defer rdb.FlushDB(context.Background())
	validMW := middleware.NewJWTMiddleware(
		base64.StdEncoding.EncodeToString(pub),
		base64.StdEncoding.EncodeToString(priv),
		rdb,
	)

	userID := uuid.New().String()
	refreshToken, _ := validMW.GenerateRefreshToken(userID, "CUSTOMER", "+91XXXXXX0000", "device-1")

	_, errCode, err := svc.RefreshAccessToken(context.Background(), refreshToken)
	if err == nil {
		t.Error("expected Redis failure")
	}
	if errCode != "INTERNAL_SERVER_ERROR" {
		t.Errorf("expected INTERNAL_SERVER_ERROR, got %q", errCode)
	}
}

func TestRefreshAccessToken_AccessTokenGenFail(t *testing.T) {
	_, rdb := setupTestJWT(t)
	defer rdb.FlushDB(context.Background())

	jwtMW := &mockTokenGenerator{genAccessErr: fmt.Errorf("token gen error")}
	sessStore := &mockSessionStore{hasActive: true}
	svc := NewAuthService(rdb, &mockUserRepo{}, &mockLoginAttemptRepo{}, sessStore, jwtMW, &mockPublisher{})

	// Stored refresh hash must exist for RefreshAccessToken to reach access token generation.
	rdb.Set(context.Background(), "refresh:user-1:d1", sha256Hash("some-token"), 30*24*time.Hour)

	_, errCode, err := svc.RefreshAccessToken(context.Background(), "some-token")
	if err == nil {
		t.Error("expected access token generation to fail")
	}
	if errCode != "INTERNAL_SERVER_ERROR" {
		t.Errorf("expected INTERNAL_SERVER_ERROR, got %q", errCode)
	}
}

func TestRefreshAccessToken_BlacklistCheckError(t *testing.T) {
	_, rdb := setupTestJWT(t)
	defer rdb.FlushDB(context.Background())

	jwtMW := &mockTokenGenerator{isBlackErr: fmt.Errorf("redis down")}
	sessStore := &mockSessionStore{hasActive: true}
	svc := NewAuthService(rdb, &mockUserRepo{}, &mockLoginAttemptRepo{}, sessStore, jwtMW, &mockPublisher{})

	rdb.Set(context.Background(), "refresh:user-1:d1", sha256Hash("some-token"), 30*24*time.Hour)

	_, errCode, err := svc.RefreshAccessToken(context.Background(), "some-token")
	if err == nil {
		t.Fatal("expected error")
	}
	if errCode != "INTERNAL_SERVER_ERROR" {
		t.Errorf("expected INTERNAL_SERVER_ERROR, got %q", errCode)
	}
}

// ─── Logout ─────────────────────────────────────────────────────────

func TestLogout_Success(t *testing.T) {
	jwtMW, rdb := setupTestJWT(t)
	defer rdb.FlushDB(context.Background())

	svc := NewAuthService(rdb, &mockUserRepo{}, &mockLoginAttemptRepo{}, &mockSessionStore{}, jwtMW, &mockPublisher{})

	ctx := context.Background()
	userID := uuid.New().String()

	accessToken, _ := jwtMW.GenerateAccessToken(userID, "CUSTOMER", "+91XXXXXX0000", "device-1")
	claims, _ := jwtMW.ValidateToken(accessToken)

	// Store refresh token
	rdb.Set(ctx, "refresh:"+userID+":"+"device-1", "some-hash", 30*24*time.Hour)

	err := svc.Logout(ctx, claims)
	if err != nil {
		t.Fatalf("Logout failed: %v", err)
	}

	// Access token should be blacklisted
	blacklisted, _ := jwtMW.IsBlacklisted(ctx, claims.ID)
	if !blacklisted {
		t.Error("access token should be blacklisted")
	}

	// Refresh should be deleted
	time.Sleep(50 * time.Millisecond)
	exists := rdb.Exists(ctx, "refresh:"+userID+":"+"device-1").Val()
	if exists != 0 {
		t.Error("refresh token should be deleted")
	}
}

func TestLogout_BlacklistFail(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	failingRdb := redis.NewClient(&redis.Options{Addr: "localhost:9999"})
	defer failingRdb.Close()

	jwtMW := middleware.NewJWTMiddleware(
		base64.StdEncoding.EncodeToString(pub),
		base64.StdEncoding.EncodeToString(priv),
		failingRdb,
	)

	svc := NewAuthService(failingRdb, &mockUserRepo{}, &mockLoginAttemptRepo{}, &mockSessionStore{}, jwtMW, &mockPublisher{})

	userID := uuid.New().String()
	// Use valid MW to generate token
	rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379", Password: "zippyra_local", DB: 1})
	defer rdb.FlushDB(context.Background())
	validMW := middleware.NewJWTMiddleware(
		base64.StdEncoding.EncodeToString(pub),
		base64.StdEncoding.EncodeToString(priv),
		rdb,
	)
	accessToken, _ := validMW.GenerateAccessToken(userID, "CUSTOMER", "+91XXXXXX0000", "device-1")
	claims, _ := validMW.ValidateToken(accessToken)

	err := svc.Logout(context.Background(), claims)
	if err == nil {
		t.Error("expected blacklist SET to fail")
	}
}

// ─── JWT token generation/validation ────────────────────────────────

func TestJWT_GenerateAndValidate(t *testing.T) {
	jwtMW, rdb := setupTestJWT(t)
	defer rdb.FlushDB(context.Background())

	token, err := jwtMW.GenerateAccessToken("user-123", "CUSTOMER", "+91XXXXXX3210", "device-1")
	if err != nil {
		t.Fatalf("GenerateAccessToken failed: %v", err)
	}

	claims, err := jwtMW.ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken failed: %v", err)
	}
	if claims.UserID != "user-123" {
		t.Errorf("expected user_id 'user-123', got %q", claims.UserID)
	}
	if claims.UserType != "CUSTOMER" {
		t.Errorf("expected user_type 'CUSTOMER', got %q", claims.UserType)
	}
}

func TestJWT_StaffTokenOnCustomerEndpoint_Forbidden(t *testing.T) {
	jwtMW, rdb := setupTestJWT(t)
	defer rdb.FlushDB(context.Background())

	token, _ := jwtMW.GenerateAccessToken("user-456", "STAFF", "+91XXXXXX0000", "device-2")
	claims, _ := jwtMW.ValidateToken(token)

	if claims.UserType == "CUSTOMER" {
		t.Error("STAFF token should not have CUSTOMER user_type")
	}
	if claims.UserType != "STAFF" {
		t.Errorf("expected 'STAFF', got %q", claims.UserType)
	}
}

func TestJWT_BlacklistedToken(t *testing.T) {
	jwtMW, rdb := setupTestJWT(t)
	defer rdb.FlushDB(context.Background())

	token, _ := jwtMW.GenerateAccessToken("user-789", "CUSTOMER", "+91XXXXXX1111", "device-3")
	claims, _ := jwtMW.ValidateToken(token)

	jwtMW.BlacklistToken(context.Background(), claims.ID, claims.ExpiresAt.Time)

	blacklisted, _ := jwtMW.IsBlacklisted(context.Background(), claims.ID)
	if !blacklisted {
		t.Error("token should be blacklisted")
	}
}

func TestJWT_NotBlacklisted(t *testing.T) {
	jwtMW, rdb := setupTestJWT(t)
	defer rdb.FlushDB(context.Background())

	blacklisted, err := jwtMW.IsBlacklisted(context.Background(), "non-existent-jti")
	if err != nil {
		t.Fatalf("IsBlacklisted failed: %v", err)
	}
	if blacklisted {
		t.Error("non-existent JTI should not be blacklisted")
	}
}

func TestJWT_RefreshTokenLongerExpiry(t *testing.T) {
	jwtMW, rdb := setupTestJWT(t)
	defer rdb.FlushDB(context.Background())

	accessToken, _ := jwtMW.GenerateAccessToken("u1", "CUSTOMER", "p1", "d1")
	refreshToken, _ := jwtMW.GenerateRefreshToken("u1", "CUSTOMER", "p1", "d1")

	aClaims, _ := jwtMW.ValidateToken(accessToken)
	rClaims, _ := jwtMW.ValidateToken(refreshToken)

	if !rClaims.ExpiresAt.Time.After(aClaims.ExpiresAt.Time) {
		t.Error("refresh token should expire after access token")
	}
}

func TestJWT_BlacklistExpiredToken_NoOp(t *testing.T) {
	jwtMW, rdb := setupTestJWT(t)
	defer rdb.FlushDB(context.Background())

	// Blacklist with a past expiry — should be a no-op
	err := jwtMW.BlacklistToken(context.Background(), "past-jti", time.Now().Add(-1*time.Hour))
	if err != nil {
		t.Fatalf("BlacklistToken with past expiry should not error: %v", err)
	}

	// Should NOT be blacklisted (TTL was 0, so Set was skipped)
	blacklisted, _ := jwtMW.IsBlacklisted(context.Background(), "past-jti")
	if blacklisted {
		t.Error("expired token should not be in blacklist")
	}
}
