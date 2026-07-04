package service

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/zippyra/platform/services/auth-service/internal/middleware"
)

func setupSessionService(t *testing.T) (*SessionService, *redis.Client, *mockSessionStore) {
	t.Helper()
	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "zippyra_local",
		DB:       1,
	})
	store := &mockSessionStore{}
	pub, priv, _ := ed25519.GenerateKey(nil)
	jwtMW := middleware.NewJWTMiddleware(
		base64.StdEncoding.EncodeToString(pub),
		base64.StdEncoding.EncodeToString(priv),
		rdb,
	)
	return NewSessionService(rdb, store, jwtMW), rdb, store
}

// ─── NewSessionService ──────────────────────────────────────────────

func TestNewSessionService(t *testing.T) {
	svc, rdb, _ := setupSessionService(t)
	defer rdb.FlushDB(context.Background())
	if svc == nil {
		t.Fatal("NewSessionService returned nil")
	}
}

// ─── ListSessions ───────────────────────────────────────────────────

func TestListSessions_Success(t *testing.T) {
	svc, rdb, store := setupSessionService(t)
	defer rdb.FlushDB(context.Background())

	sid1 := uuid.New()
	sid2 := uuid.New()
	now := time.Now()

	store.listResult = []SessionRow{
		{ID: sid1, DeviceID: "device-1", DeviceModel: "iPhone 15", LastActiveAt: now, IPAddress: "10.0.0.1"},
		{ID: sid2, DeviceID: "device-2", DeviceModel: "Pixel 8", LastActiveAt: now.Add(-time.Hour), IPAddress: "10.0.0.2"},
	}

	sessions, err := svc.ListSessions(context.Background(), uuid.New().String(), "device-1")
	if err != nil {
		t.Fatalf("ListSessions failed: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(sessions))
	}
	if !sessions[0].IsCurrent {
		t.Error("first session should be current (device-1)")
	}
	if sessions[1].IsCurrent {
		t.Error("second session should not be current")
	}
	if sessions[0].DeviceModel != "iPhone 15" {
		t.Errorf("expected 'iPhone 15', got %q", sessions[0].DeviceModel)
	}
}

func TestListSessions_InvalidUserID(t *testing.T) {
	svc, rdb, _ := setupSessionService(t)
	defer rdb.FlushDB(context.Background())

	_, err := svc.ListSessions(context.Background(), "not-a-uuid", "device-1")
	if err == nil {
		t.Fatal("expected error for invalid user_id")
	}
}

func TestListSessions_DBError(t *testing.T) {
	svc, rdb, store := setupSessionService(t)
	defer rdb.FlushDB(context.Background())

	store.listErr = fmt.Errorf("connection refused")
	_, err := svc.ListSessions(context.Background(), uuid.New().String(), "d1")
	if err == nil {
		t.Fatal("expected error from DB")
	}
}

func TestListSessions_Empty(t *testing.T) {
	svc, rdb, store := setupSessionService(t)
	defer rdb.FlushDB(context.Background())

	store.listResult = nil
	sessions, err := svc.ListSessions(context.Background(), uuid.New().String(), "d1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sessions != nil {
		t.Errorf("expected nil sessions, got %d", len(sessions))
	}
}

// ─── RevokeSession ──────────────────────────────────────────────────

func TestRevokeSession_Success(t *testing.T) {
	svc, rdb, _ := setupSessionService(t)
	defer rdb.FlushDB(context.Background())

	err := svc.RevokeSession(context.Background(), uuid.New().String(), uuid.New().String())
	if err != nil {
		t.Fatalf("RevokeSession failed: %v", err)
	}
}

func TestRevokeSession_InvalidSessionID(t *testing.T) {
	svc, rdb, _ := setupSessionService(t)
	defer rdb.FlushDB(context.Background())

	err := svc.RevokeSession(context.Background(), uuid.New().String(), "not-a-uuid")
	if err == nil {
		t.Fatal("expected error for invalid session_id")
	}
}

func TestRevokeSession_NotFound(t *testing.T) {
	svc, rdb, store := setupSessionService(t)
	defer rdb.FlushDB(context.Background())

	store.revokeErr = fmt.Errorf("session not found")
	err := svc.RevokeSession(context.Background(), uuid.New().String(), uuid.New().String())
	if err == nil {
		t.Fatal("expected error for not found session")
	}
}

// ─── RevokeAllSessions ──────────────────────────────────────────────

func TestRevokeAllSessions_Success(t *testing.T) {
	svc, rdb, store := setupSessionService(t)
	defer rdb.FlushDB(context.Background())

	store.revokeAllCnt = 3
	userID := uuid.New().String()

	rdb.Set(context.Background(), "refresh:"+userID+":device-1", "hash1", 30*24*time.Hour)
	rdb.Set(context.Background(), "refresh:"+userID+":device-2", "hash2", 30*24*time.Hour)

	count, err := svc.RevokeAllSessions(context.Background(), userID, "device-1")
	if err != nil {
		t.Fatalf("RevokeAllSessions failed: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3 revoked, got %d", count)
	}

	deleted := rdb.Exists(context.Background(), "refresh:"+userID+":device-2").Val()
	if deleted != 0 {
		t.Error("other device refresh token should be deleted")
	}
	kept := rdb.Exists(context.Background(), "refresh:"+userID+":device-1").Val()
	if kept == 0 {
		t.Error("current device refresh token should be kept")
	}
}

func TestRevokeAllSessions_DBError(t *testing.T) {
	svc, rdb, store := setupSessionService(t)
	defer rdb.FlushDB(context.Background())

	store.revokeAllErr = fmt.Errorf("db failure")
	_, err := svc.RevokeAllSessions(context.Background(), uuid.New().String(), "d1")
	if err == nil {
		t.Fatal("expected error")
	}
}
