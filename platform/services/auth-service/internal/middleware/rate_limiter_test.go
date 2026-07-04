package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	sharedErrors "github.com/zippyra/platform/shared/errors"
)

type rateErrResp struct {
	Error struct {
		Code string `json:"code"`
	} `json:"error"`
}

func newTestRedisRL(t *testing.T) *redis.Client {
	t.Helper()
	return redis.NewClient(&redis.Options{Addr: "localhost:6379", Password: "zippyra_local", DB: 1})
}

func TestCheckRateLimit_AllPass(t *testing.T) {
	rdb := newTestRedisRL(t)
	defer rdb.FlushDB(context.Background())

	layer, err := CheckRateLimit(rdb, "+919876543210", "1.2.3.4", time.Now().Format("2006-01-02"))
	if err != nil {
		t.Fatal(err)
	}
	if layer != 0 {
		t.Fatalf("expected 0, got %d", layer)
	}
}

func TestCheckRateLimit_Layer1Exceeded(t *testing.T) {
	rdb := newTestRedisRL(t)
	defer rdb.FlushDB(context.Background())

	ctx := context.Background()
	phone := "+919876543210"
	ip := "1.2.3.4"
	date := time.Now().Format("2006-01-02")
	_ = rdb.Set(ctx, "otp_phone_10m:"+phone, "5", 600*time.Second).Err()

	layer, err := CheckRateLimit(rdb, phone, ip, date)
	if err != nil {
		t.Fatal(err)
	}
	if layer != 1 {
		t.Fatalf("expected 1, got %d", layer)
	}
}

func TestCheckRateLimit_RedisError(t *testing.T) {
	rdb := redis.NewClient(&redis.Options{Addr: "localhost:9999"})
	defer rdb.Close()

	_, err := CheckRateLimit(rdb, "+919876543210", "1.2.3.4", time.Now().Format("2006-01-02"))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestWriteRateLimitError(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteRateLimitError(rec, "req-1")
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", rec.Code)
	}
	var er rateErrResp
	_ = json.NewDecoder(rec.Body).Decode(&er)
	if er.Error.Code != sharedErrors.ErrRateLimitExceeded {
		t.Fatalf("expected %s, got %s", sharedErrors.ErrRateLimitExceeded, er.Error.Code)
	}
}
