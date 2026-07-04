package service

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/zippyra/platform/services/auth-service/internal/middleware"
)

const testSalt = "test-salt-that-is-at-least-32-characters-long"

func newTestRedis() *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "zippyra_local",
		DB:       1,
	})
}

// ─── GenerateOTP ────────────────────────────────────────────────────

func TestGenerateOTP_Fail(t *testing.T) {
	// Mock rand.Reader to fail
	oldReader := rand.Reader
	rand.Reader = bytes.NewReader(nil)
	defer func() { rand.Reader = oldReader }()

	_, err := GenerateOTP()
	if err == nil {
		t.Error("expected GenerateOTP to fail")
	}
}

func TestSendOTP_GenerateOTPFail(t *testing.T) {
	rdb := newTestRedis()
	defer rdb.FlushDB(context.Background())

	oldReader := rand.Reader
	rand.Reader = bytes.NewReader(nil)
	defer func() { rand.Reader = oldReader }()

	svc := NewOTPService(rdb, &mockLoginAttemptRepo{}, testSalt, "local")
	errCode, status, err := svc.SendOTP(context.Background(), "+919876543210", "1.2.3.4", "ua")
	if err == nil {
		t.Fatal("expected error")
	}
	if errCode != "INTERNAL_SERVER_ERROR" {
		t.Errorf("expected INTERNAL_SERVER_ERROR, got %q", errCode)
	}
	if status != 500 {
		t.Errorf("expected 500, got %d", status)
	}
}

func TestSendOTP_PipelineExecFails_ContextCancelled(t *testing.T) {
	rdb := newTestRedis()
	defer rdb.FlushDB(context.Background())

	svc := NewOTPService(rdb, &mockLoginAttemptRepo{}, testSalt, "local")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	errCode, status, err := svc.SendOTP(ctx, "+919876543210", "1.2.3.4", "ua")
	if err == nil {
		t.Fatal("expected error")
	}
	if errCode != "INTERNAL_SERVER_ERROR" {
		t.Errorf("expected INTERNAL_SERVER_ERROR, got %q", errCode)
	}
	if status != 500 {
		t.Errorf("expected 500, got %d", status)
	}
}

func TestGenerateOTP_Is6Digits(t *testing.T) {

	otp, err := GenerateOTP()
	if err != nil {
		t.Fatalf("GenerateOTP failed: %v", err)
	}
	if len(otp) != 6 {
		t.Errorf("expected 6-digit OTP, got %q (len=%d)", otp, len(otp))
	}
	for _, c := range otp {
		if c < '0' || c > '9' {
			t.Errorf("OTP contains non-digit character: %c", c)
		}
	}
}

func TestGenerateOTP_IsRandom(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		otp, err := GenerateOTP()
		if err != nil {
			t.Fatalf("GenerateOTP failed: %v", err)
		}
		seen[otp] = true
	}
	if len(seen) < 50 {
		t.Errorf("OTPs not random enough: only %d unique out of 100", len(seen))
	}
}

// ─── ValidatePhone ──────────────────────────────────────────────────

func TestValidatePhone_ValidIndian(t *testing.T) {
	valid := []string{"+919876543210", "+916000000000", "+917999999999", "+918123456789"}
	for _, phone := range valid {
		if !ValidatePhone(phone) {
			t.Errorf("expected %q to be valid", phone)
		}
	}
}

func TestValidatePhone_Invalid(t *testing.T) {
	invalid := []string{
		"9876543210", "+911234567890", "+91987654321", "+9198765432100",
		"+12025551234", "", "+91ABCDEFGHIJ",
	}
	for _, phone := range invalid {
		if ValidatePhone(phone) {
			t.Errorf("expected %q to be invalid", phone)
		}
	}
}

// ─── HashOTP ────────────────────────────────────────────────────────

func TestHashOTP_Deterministic(t *testing.T) {
	svc := &OTPService{salt: testSalt}
	hash1 := svc.HashOTP("123456", "+919876543210")
	hash2 := svc.HashOTP("123456", "+919876543210")
	if hash1 != hash2 {
		t.Error("same input should produce same hash")
	}
}

func TestHashOTP_DifferentInputs(t *testing.T) {
	svc := &OTPService{salt: testSalt}
	hash1 := svc.HashOTP("123456", "+919876543210")
	hash2 := svc.HashOTP("654321", "+919876543210")
	hash3 := svc.HashOTP("123456", "+919876543211")

	if hash1 == hash2 {
		t.Error("different OTPs should produce different hashes")
	}
	if hash1 == hash3 {
		t.Error("different phones should produce different hashes")
	}
}

// ─── NewOTPService ──────────────────────────────────────────────────

func TestNewOTPService(t *testing.T) {
	rdb := newTestRedis()
	defer rdb.FlushDB(context.Background())

	mock := &mockLoginAttemptRepo{}
	svc := NewOTPService(rdb, mock, testSalt, "local")
	if svc == nil {
		t.Fatal("NewOTPService returned nil")
	}
	if svc.appEnv != "local" {
		t.Errorf("expected appEnv 'local', got %q", svc.appEnv)
	}
}

// ─── SendOTP ────────────────────────────────────────────────────────

func TestSendOTP_InvalidPhone(t *testing.T) {
	rdb := newTestRedis()
	defer rdb.FlushDB(context.Background())

	svc := NewOTPService(rdb, &mockLoginAttemptRepo{}, testSalt, "local")
	errCode, status, err := svc.SendOTP(context.Background(), "invalid", "1.2.3.4", "test-agent")
	if errCode != "INVALID_PHONE_FORMAT" {
		t.Errorf("expected INVALID_PHONE_FORMAT, got %q", errCode)
	}
	if status != 400 {
		t.Errorf("expected 400, got %d", status)
	}
	if err == nil {
		t.Error("expected error")
	}
}

func TestSendOTP_RateLimited(t *testing.T) {
	rdb := newTestRedis()
	defer rdb.FlushDB(context.Background())

	ctx := context.Background()
	phone := "+919876543210"

	// Pre-fill rate limit bucket
	rdb.Set(ctx, "otp_phone_10m:"+phone, "5", 600*time.Second)

	svc := NewOTPService(rdb, &mockLoginAttemptRepo{}, testSalt, "local")
	errCode, status, _ := svc.SendOTP(ctx, phone, "1.2.3.4", "test-agent")
	if errCode != "RATE_LIMIT_EXCEEDED" {
		t.Errorf("expected RATE_LIMIT_EXCEEDED, got %q", errCode)
	}
	if status != 429 {
		t.Errorf("expected 429, got %d", status)
	}
}

func TestSendOTP_Success_LocalEnv(t *testing.T) {
	rdb := newTestRedis()
	defer rdb.FlushDB(context.Background())

	mock := &mockLoginAttemptRepo{}
	svc := NewOTPService(rdb, mock, testSalt, "local")
	errCode, status, err := svc.SendOTP(context.Background(), "+919876543210", "1.2.3.4", "test-agent")
	if errCode != "" {
		t.Errorf("expected no error code, got %q", errCode)
	}
	if status != 0 {
		t.Errorf("expected status 0, got %d", status)
	}
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// OTP hash should be stored in Redis
	val := rdb.Get(context.Background(), "otp:+919876543210").Val()
	if val == "" {
		t.Error("OTP hash should be stored in Redis")
	}

	// Wait for goroutine to complete
	time.Sleep(50 * time.Millisecond)
}

func TestSendOTP_Success_PilotEnv(t *testing.T) {
	rdb := newTestRedis()
	defer rdb.FlushDB(context.Background())

	svc := NewOTPService(rdb, &mockLoginAttemptRepo{}, testSalt, "pilot")
	errCode, _, err := svc.SendOTP(context.Background(), "+916000000000", "1.2.3.4", "test")
	if errCode != "" || err != nil {
		t.Errorf("pilot env should succeed: errCode=%q, err=%v", errCode, err)
	}
}

func TestSendOTP_Success_ProductionEnv(t *testing.T) {
	rdb := newTestRedis()
	defer rdb.FlushDB(context.Background())

	svc := NewOTPService(rdb, &mockLoginAttemptRepo{}, testSalt, "production")
	errCode, _, err := svc.SendOTP(context.Background(), "+917000000000", "1.2.3.4", "test")
	if errCode != "" || err != nil {
		t.Errorf("production env should succeed: errCode=%q, err=%v", errCode, err)
	}
}

// ─── VerifyOTP ──────────────────────────────────────────────────────

func TestVerifyOTP_InvalidPhone(t *testing.T) {
	rdb := newTestRedis()
	defer rdb.FlushDB(context.Background())

	svc := &OTPService{rdb: rdb, salt: testSalt}
	errCode, status, _ := svc.VerifyOTP(context.Background(), "bad", "123456")
	if errCode != "INVALID_PHONE_FORMAT" {
		t.Errorf("expected INVALID_PHONE_FORMAT, got %q", errCode)
	}
	if status != 400 {
		t.Errorf("expected 400, got %d", status)
	}
}

func TestVerifyOTP_CorrectOTP(t *testing.T) {
	rdb := newTestRedis()
	defer rdb.FlushDB(context.Background())

	svc := &OTPService{rdb: rdb, salt: testSalt, appEnv: "local"}
	phone := "+919876543210"
	otp := "123456"
	hash := svc.HashOTP(otp, phone)

	ctx := context.Background()
	rdb.Set(ctx, "otp:"+phone, hash, 600*time.Second)
	rdb.Set(ctx, "otp_attempts:"+phone, "0", 600*time.Second)

	errCode, _, err := svc.VerifyOTP(ctx, phone, otp)
	if err != nil {
		t.Fatalf("VerifyOTP failed: %v", err)
	}
	if errCode != "" {
		t.Errorf("expected no error code, got %q", errCode)
	}

	// OTP should be deleted (one-time use)
	exists := rdb.Exists(ctx, "otp:"+phone).Val()
	if exists != 0 {
		t.Error("OTP should be deleted after successful verification")
	}
}

func TestVerifyOTP_ExpiredOTP(t *testing.T) {
	rdb := newTestRedis()
	defer rdb.FlushDB(context.Background())

	svc := &OTPService{rdb: rdb, salt: testSalt, appEnv: "local"}
	ctx := context.Background()
	errCode, status, _ := svc.VerifyOTP(ctx, "+919876543210", "123456")
	if errCode != "OTP_EXPIRED" {
		t.Errorf("expected OTP_EXPIRED, got %q", errCode)
	}
	if status != 400 {
		t.Errorf("expected status 400, got %d", status)
	}
}

func TestVerifyOTP_WrongOTP_UnderMaxAttempts(t *testing.T) {
	rdb := newTestRedis()
	defer rdb.FlushDB(context.Background())

	svc := &OTPService{rdb: rdb, salt: testSalt, appEnv: "local"}
	phone := "+919876543210"
	hash := svc.HashOTP("123456", phone)

	ctx := context.Background()
	rdb.Set(ctx, "otp:"+phone, hash, 600*time.Second)
	rdb.Set(ctx, "otp_attempts:"+phone, "0", 600*time.Second)

	errCode, status, _ := svc.VerifyOTP(ctx, phone, "000001")
	if errCode != "OTP_INVALID" {
		t.Errorf("expected OTP_INVALID, got %q", errCode)
	}
	if status != 400 {
		t.Errorf("expected 400, got %d", status)
	}
}

func TestVerifyOTP_WrongOTP3Times_MaxAttempts(t *testing.T) {
	rdb := newTestRedis()
	defer rdb.FlushDB(context.Background())

	svc := &OTPService{rdb: rdb, salt: testSalt, appEnv: "local"}
	phone := "+919876543210"
	hash := svc.HashOTP("123456", phone)

	ctx := context.Background()
	rdb.Set(ctx, "otp:"+phone, hash, 600*time.Second)
	rdb.Set(ctx, "otp_attempts:"+phone, "0", 600*time.Second)

	// Attempts 1 and 2
	svc.VerifyOTP(ctx, phone, "000001")
	rdb.Set(ctx, "otp:"+phone, hash, 600*time.Second)
	svc.VerifyOTP(ctx, phone, "000002")
	rdb.Set(ctx, "otp:"+phone, hash, 600*time.Second)

	// Attempt 3 — should exceed max
	errCode, statusCode, _ := svc.VerifyOTP(ctx, phone, "000003")
	if errCode != "OTP_MAX_ATTEMPTS_EXCEEDED" {
		t.Errorf("expected OTP_MAX_ATTEMPTS_EXCEEDED, got %q", errCode)
	}
	if statusCode != 429 {
		t.Errorf("expected status 429, got %d", statusCode)
	}

	exists := rdb.Exists(ctx, "otp:"+phone).Val()
	if exists != 0 {
		t.Error("OTP should be deleted after max attempts")
	}
}

func TestVerifyOTP_TimingAttack(t *testing.T) {
	rdb := newTestRedis()
	defer rdb.FlushDB(context.Background())

	svc := &OTPService{rdb: rdb, salt: testSalt, appEnv: "local"}
	phone := "+919876543210"
	hash := svc.HashOTP("123456", phone)
	ctx := context.Background()

	rdb.Set(ctx, "otp:"+phone, hash, 600*time.Second)
	rdb.Set(ctx, "otp_attempts:"+phone, "0", 600*time.Second)
	errCode, status, err := svc.VerifyOTP(ctx, phone, "123456")
	if err != nil || errCode != "" || status != 0 {
		t.Fatalf("expected correct OTP to succeed, got errCode=%q status=%d err=%v", errCode, status, err)
	}

	rdb.Set(ctx, "otp:"+phone, hash, 600*time.Second)
	rdb.Set(ctx, "otp_attempts:"+phone, "0", 600*time.Second)
	errCode, status, err = svc.VerifyOTP(ctx, phone, "000000")
	if err == nil || errCode != "OTP_INVALID" || status != 400 {
		t.Fatalf("expected wrong OTP to fail, got errCode=%q status=%d err=%v", errCode, status, err)
	}
}

// ─── Rate Limit ─────────────────────────────────────────────────────

func TestRateLimit_6thOTPSend(t *testing.T) {
	rdb := newTestRedis()
	defer rdb.FlushDB(context.Background())

	ctx := context.Background()
	phone := "+919876543210"
	ip := "192.168.1.1"
	date := time.Now().Format("2006-01-02")

	rdb.Set(ctx, "otp_phone_10m:"+phone, "5", 600*time.Second)
	layer, err := middleware.CheckRateLimit(rdb, phone, ip, date)
	if err != nil {
		t.Fatalf("CheckRateLimit failed: %v", err)
	}
	if layer != 1 {
		t.Errorf("expected rate limit at layer 1, got layer %d", layer)
	}
}

func TestRateLimit_AllLayersPass(t *testing.T) {
	rdb := newTestRedis()
	defer rdb.FlushDB(context.Background())

	phone := "+919876543210"
	ip := "10.0.0.1"
	date := time.Now().Format("2006-01-02")

	layer, err := middleware.CheckRateLimit(rdb, phone, ip, date)
	if err != nil {
		t.Fatalf("CheckRateLimit failed: %v", err)
	}
	if layer != 0 {
		t.Errorf("expected layer 0 (all pass), got %d", layer)
	}
}

func TestRateLimit_Layer2_PhonePer1Min(t *testing.T) {
	rdb := newTestRedis()
	defer rdb.FlushDB(context.Background())

	ctx := context.Background()
	phone := "+919876543210"
	ip := "10.0.0.1"
	date := time.Now().Format("2006-01-02")

	rdb.Set(ctx, "otp_phone_1m:"+phone, "3", 60*time.Second)
	layer, _ := middleware.CheckRateLimit(rdb, phone, ip, date)
	if layer != 2 {
		t.Errorf("expected layer 2, got %d", layer)
	}
}

func TestRateLimit_Layer3_IPPer10Min(t *testing.T) {
	rdb := newTestRedis()
	defer rdb.FlushDB(context.Background())

	ctx := context.Background()
	phone := "+919876543210"
	ip := "10.0.0.1"
	date := time.Now().Format("2006-01-02")

	rdb.Set(ctx, "otp_ip_10m:"+ip, "10", 600*time.Second)
	layer, _ := middleware.CheckRateLimit(rdb, phone, ip, date)
	if layer != 3 {
		t.Errorf("expected layer 3, got %d", layer)
	}
}

func TestRateLimit_Layer4_IPPer1Hour(t *testing.T) {
	rdb := newTestRedis()
	defer rdb.FlushDB(context.Background())

	ctx := context.Background()
	phone := "+919876543210"
	ip := "10.0.0.1"
	date := time.Now().Format("2006-01-02")

	rdb.Set(ctx, "otp_ip_1h:"+ip, "20", 3600*time.Second)
	layer, _ := middleware.CheckRateLimit(rdb, phone, ip, date)
	if layer != 4 {
		t.Errorf("expected layer 4, got %d", layer)
	}
}

func TestRateLimit_Layer5_GlobalDaily(t *testing.T) {
	rdb := newTestRedis()
	defer rdb.FlushDB(context.Background())

	ctx := context.Background()
	phone := "+919876543210"
	ip := "10.0.0.1"
	date := time.Now().Format("2006-01-02")

	rdb.Set(ctx, "otp_global:"+date, "100", 86400*time.Second)
	layer, _ := middleware.CheckRateLimit(rdb, phone, ip, date)
	if layer != 5 {
		t.Errorf("expected layer 5, got %d", layer)
	}
}

func TestSendOTP_RateLimitCheckFails(t *testing.T) {
	failingRdb := redis.NewClient(&redis.Options{Addr: "localhost:9999"})
	defer failingRdb.Close()

	svc := NewOTPService(failingRdb, &mockLoginAttemptRepo{}, testSalt, "local")
	errCode, status, err := svc.SendOTP(context.Background(), "+919876543210", "1.2.3.4", "ua")
	if err == nil {
		t.Fatal("expected rate limit check to fail")
	}
	if errCode != "INTERNAL_SERVER_ERROR" {
		t.Errorf("expected INTERNAL_SERVER_ERROR, got %q", errCode)
	}
	if status != 500 {
		t.Errorf("expected 500, got %d", status)
	}
}

func TestSendOTP_RedisStoreFail(t *testing.T) {
	failingRdb := redis.NewClient(&redis.Options{Addr: "localhost:9999"})
	defer failingRdb.Close()

	svc := NewOTPService(failingRdb, &mockLoginAttemptRepo{}, testSalt, "local")
	errCode, _, err := svc.SendOTP(context.Background(), "+919876543210", "1.1.1.1", "ua")
	if err == nil {
		t.Error("expected Redis failure")
	}
	if errCode != "INTERNAL_SERVER_ERROR" {
		t.Errorf("expected INTERNAL_SERVER_ERROR, got %q", errCode)
	}
}

func TestSendOTP_LoginAttemptLogFail(t *testing.T) {
	rdb := newTestRedis()
	defer rdb.FlushDB(context.Background())

	done := make(chan struct{})
	mock := &mockLoginAttemptRepo{insertErr: fmt.Errorf("db error"), insertDone: done}
	svc := NewOTPService(rdb, mock, testSalt, "local")

	errCode, _, err := svc.SendOTP(context.Background(), "+919876543210", "1.1.1.1", "ua")
	if err != nil || errCode != "" {
		t.Errorf("expected success even if logging fails, got errCode=%q, err=%v", errCode, err)
	}

	select {
	case <-done:
		// ok
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for login attempt insert")
	}
}

func TestVerifyOTP_RedisGetError(t *testing.T) {
	rdb := newTestRedis()
	defer rdb.FlushDB(context.Background())
	rdb.Close()

	svc := &OTPService{rdb: rdb, salt: testSalt}
	_, status, err := svc.VerifyOTP(context.Background(), "+919876543210", "123456")
	if err == nil {
		t.Error("expected error from closed Redis")
	}
	if status != 500 {
		t.Errorf("expected 500, got %d", status)
	}
}

func TestVerifyOTP_IncrAttemptsFail(t *testing.T) {
	rdb := newTestRedis()
	defer rdb.FlushDB(context.Background())

	phone := "+919876543210"
	svc := &OTPService{rdb: rdb, salt: testSalt, appEnv: "local"}
	hash := svc.HashOTP("wrong", phone)
	rdb.Set(context.Background(), "otp:"+phone, hash, 600*time.Second)

	rdb.Close()
	_, status, _ := svc.VerifyOTP(context.Background(), phone, "123456")

	// Hits line 151 (rdb.Get fails)
	if status != 500 {
		t.Errorf("expected 500, got %d", status)
	}
}

// ─── sha256Hash ─────────────────────────────────────────────────────

func TestSha256Hash(t *testing.T) {
	h := sha256Hash("test-data")
	if len(h) != 64 {
		t.Errorf("expected 64 char hex hash, got len=%d", len(h))
	}
	h2 := sha256Hash("test-data")
	if h != h2 {
		t.Error("same input should produce same hash")
	}
	h3 := sha256Hash("different-data")
	if h == h3 {
		t.Error("different input should produce different hash")
	}
}
