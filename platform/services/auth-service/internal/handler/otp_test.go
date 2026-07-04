package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zippyra/platform/services/auth-service/internal/service"
)

type stubOTPService struct {
	sendFn   func(ctx context.Context, phone, ip, ua string) (string, int, error)
	verifyFn func(ctx context.Context, phone, otp string) (string, int, error)
}

func (s *stubOTPService) SendOTP(ctx context.Context, phone, ip, ua string) (string, int, error) {
	return s.sendFn(ctx, phone, ip, ua)
}
func (s *stubOTPService) VerifyOTP(ctx context.Context, phone, otp string) (string, int, error) {
	return s.verifyFn(ctx, phone, otp)
}

type stubAuthService struct {
	completeFn func(ctx context.Context, phone, deviceID, deviceModel, ip, userAgent string) (*service.AuthResult, error)
}

func (s *stubAuthService) CompleteLogin(ctx context.Context, phone, deviceID, deviceModel, ip, userAgent string) (*service.AuthResult, error) {
	return s.completeFn(ctx, phone, deviceID, deviceModel, ip, userAgent)
}

func TestGetClientIP_XForwardedFor(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("X-Forwarded-For", "1.2.3.4, 5.6.7.8")
	if ip := getClientIP(req); ip != "1.2.3.4" {
		t.Fatalf("expected 1.2.3.4, got %q", ip)
	}
}

func TestGetClientIP_XRealIP(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("X-Real-IP", "9.9.9.9")
	if ip := getClientIP(req); ip != "9.9.9.9" {
		t.Fatalf("expected 9.9.9.9, got %q", ip)
	}
}

func TestSendOTP_BadJSON(t *testing.T) {
	h := NewOTPHandler(&stubOTPService{}, &stubAuthService{})
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/otp/send", bytes.NewBufferString("{"))
	rec := httptest.NewRecorder()
	h.SendOTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestSendOTP_ValidationFail(t *testing.T) {
	h := NewOTPHandler(&stubOTPService{}, &stubAuthService{})
	body, _ := json.Marshal(map[string]any{})
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/otp/send", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.SendOTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestSendOTP_ValidationFail_Generic(t *testing.T) {
	old := validateRequest
	defer func() { validateRequest = old }()
	validateRequest = func(v any) error { return errors.New("boom") }

	h := NewOTPHandler(&stubOTPService{}, &stubAuthService{})
	body, _ := json.Marshal(map[string]any{"phone": "+919876543210"})
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/otp/send", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.SendOTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestSendOTP_ServiceError(t *testing.T) {
	otp := &stubOTPService{sendFn: func(ctx context.Context, phone, ip, ua string) (string, int, error) {
		return "RATE_LIMIT_EXCEEDED", 429, errors.New("rate")
	}}
	h := NewOTPHandler(otp, &stubAuthService{})
	body, _ := json.Marshal(map[string]any{"phone": "+919876543210"})
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/otp/send", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.SendOTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", rec.Code)
	}
}

func TestSendOTP_Success(t *testing.T) {
	otp := &stubOTPService{sendFn: func(ctx context.Context, phone, ip, ua string) (string, int, error) {
		return "", 0, nil
	}}
	h := NewOTPHandler(otp, &stubAuthService{})
	body, _ := json.Marshal(map[string]any{"phone": "+919876543210"})
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/otp/send", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.SendOTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestVerifyOTP_BadJSON(t *testing.T) {
	h := NewOTPHandler(&stubOTPService{}, &stubAuthService{})
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/otp/verify", bytes.NewBufferString("{"))
	rec := httptest.NewRecorder()
	h.VerifyOTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestVerifyOTP_ValidationFail(t *testing.T) {
	h := NewOTPHandler(&stubOTPService{}, &stubAuthService{})
	body, _ := json.Marshal(map[string]any{"phone": "+919876543210"})
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/otp/verify", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.VerifyOTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestVerifyOTP_ValidationFail_Generic(t *testing.T) {
	old := validateRequest
	defer func() { validateRequest = old }()
	validateRequest = func(v any) error { return errors.New("boom") }

	h := NewOTPHandler(&stubOTPService{}, &stubAuthService{})
	body, _ := json.Marshal(map[string]any{"phone": "+919876543210", "otp": "1", "device_id": "d1", "device_model": "m1"})
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/otp/verify", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.VerifyOTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestVerifyOTP_OTPServiceError(t *testing.T) {
	otp := &stubOTPService{verifyFn: func(ctx context.Context, phone, otp string) (string, int, error) {
		return "OTP_INVALID", 400, errors.New("bad")
	}}
	h := NewOTPHandler(otp, &stubAuthService{})
	body, _ := json.Marshal(map[string]any{"phone": "+919876543210", "otp": "123", "device_id": "d1", "device_model": "m1"})
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/otp/verify", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.VerifyOTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestVerifyOTP_CompleteLoginInternalError(t *testing.T) {
	otp := &stubOTPService{verifyFn: func(ctx context.Context, phone, otp string) (string, int, error) {
		return "", 0, nil
	}}
	auth := &stubAuthService{completeFn: func(ctx context.Context, phone, deviceID, deviceModel, ip, ua string) (*service.AuthResult, error) {
		return nil, errors.New("db")
	}}
	h := NewOTPHandler(otp, auth)
	body, _ := json.Marshal(map[string]any{"phone": "+919876543210", "otp": "123456", "device_id": "d1", "device_model": "m1"})
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/otp/verify", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.VerifyOTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

func TestVerifyOTP_Success(t *testing.T) {
	otp := &stubOTPService{verifyFn: func(ctx context.Context, phone, otp string) (string, int, error) {
		return "", 0, nil
	}}
	auth := &stubAuthService{completeFn: func(ctx context.Context, phone, deviceID, deviceModel, ip, ua string) (*service.AuthResult, error) {
		return &service.AuthResult{AccessToken: "a", RefreshToken: "r", UserID: "u", IsNewUser: true, ExpiresIn: 1}, nil
	}}
	h := NewOTPHandler(otp, auth)
	body, _ := json.Marshal(map[string]any{"phone": "+919876543210", "otp": "123456", "device_id": "d1", "device_model": "m1"})
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/otp/verify", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.VerifyOTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}
