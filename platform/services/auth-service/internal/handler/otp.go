package handler

import (
	"context"
	"encoding/json"
	stdErrors "errors"
	"net"
	"net/http"
	"strings"

	v10 "github.com/go-playground/validator/v10"
	"github.com/rs/zerolog/log"

	"github.com/zippyra/platform/services/auth-service/internal/service"
	"github.com/zippyra/platform/shared/errors"
	sharedvalidator "github.com/zippyra/platform/shared/validator"
)

var validateRequest = sharedvalidator.Validate

// getClientIP extracts the client IP from the request.
func getClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	return host
}

// OTPHandler handles OTP send and verify endpoints.
type OTPHandler struct {
	otpService  otpService
	authService authService
}

type otpService interface {
	SendOTP(ctx context.Context, phone, ip, userAgent string) (string, int, error)
	VerifyOTP(ctx context.Context, phone, otp string) (string, int, error)
}

type authService interface {
	CompleteLogin(ctx context.Context, phone, deviceID, deviceModel, ip, userAgent string) (*service.AuthResult, error)
}

// NewOTPHandler creates a new OTPHandler.
func NewOTPHandler(otpService otpService, authService authService) *OTPHandler {
	return &OTPHandler{otpService: otpService, authService: authService}
}

type sendOTPRequest struct {
	Phone string `json:"phone" validate:"required"`
}

type sendOTPResponse struct {
	Message   string `json:"message"`
	ExpiresIn int    `json:"expires_in"`
}

// SendOTP handles POST /v1/auth/otp/send
func (h *OTPHandler) SendOTP(w http.ResponseWriter, r *http.Request) {
	requestID := r.Header.Get("X-Request-ID")

	var req sendOTPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errors.WriteError(w, http.StatusBadRequest, errors.ErrValidationFailed,
			"Invalid request body", requestID)
		return
	}
	if err := validateRequest(&req); err != nil {
		var ve v10.ValidationErrors
		if stdErrors.As(err, &ve) && len(ve) > 0 {
			errors.WriteValidationError(w, strings.ToLower(ve[0].Field()), "invalid value", requestID)
			return
		}
		errors.WriteError(w, http.StatusBadRequest, errors.ErrValidationFailed, "Validation failed", requestID)
		return
	}

	ip := getClientIP(r)
	userAgent := r.UserAgent()

	errCode, statusCode, err := h.otpService.SendOTP(r.Context(), req.Phone, ip, userAgent)
	if err != nil {
		errors.WriteError(w, statusCode, errCode, err.Error(), requestID)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(sendOTPResponse{
		Message:   "OTP sent successfully",
		ExpiresIn: 600,
	})
}

type verifyOTPRequest struct {
	Phone       string `json:"phone" validate:"required"`
	OTP         string `json:"otp" validate:"required"`
	DeviceID    string `json:"device_id" validate:"required"`
	DeviceModel string `json:"device_model" validate:"required"`
}

// VerifyOTP handles POST /v1/auth/otp/verify
func (h *OTPHandler) VerifyOTP(w http.ResponseWriter, r *http.Request) {
	requestID := r.Header.Get("X-Request-ID")

	var req verifyOTPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errors.WriteError(w, http.StatusBadRequest, errors.ErrValidationFailed,
			"Invalid request body", requestID)
		return
	}
	if err := validateRequest(&req); err != nil {
		var ve v10.ValidationErrors
		if stdErrors.As(err, &ve) && len(ve) > 0 {
			errors.WriteValidationError(w, strings.ToLower(ve[0].Field()), "invalid value", requestID)
			return
		}
		errors.WriteError(w, http.StatusBadRequest, errors.ErrValidationFailed, "Validation failed", requestID)
		return
	}

	// 1. Verify OTP
	errCode, statusCode, err := h.otpService.VerifyOTP(r.Context(), req.Phone, req.OTP)
	if err != nil {
		errors.WriteError(w, statusCode, errCode, err.Error(), requestID)
		return
	}

	// 2. Complete login — upsert user, generate tokens, publish event
	ip := getClientIP(r)
	userAgent := r.UserAgent()

	result, err := h.authService.CompleteLogin(r.Context(), req.Phone, req.DeviceID, req.DeviceModel, ip, userAgent)
	if err != nil {
		log.Error().Err(err).Msg("login completion failed")
		errors.WriteInternalError(w, requestID)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}
