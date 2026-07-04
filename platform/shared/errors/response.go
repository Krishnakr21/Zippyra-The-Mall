package errors

import (
	"encoding/json"
	"net/http"
)

// ErrorResponse is the standard error response envelope ALL 18 services use.
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail contains the specifics of the error.
type ErrorDetail struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
	Field     string `json:"field,omitempty"` // for validation errors only
}

// WriteError writes a JSON-encoded ErrorResponse body with the given status code.
func WriteError(w http.ResponseWriter, statusCode int, code, message, requestID string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	resp := ErrorResponse{
		Error: ErrorDetail{
			Code:      code,
			Message:   message,
			RequestID: requestID,
		},
	}

	_ = json.NewEncoder(w).Encode(resp)
}

// WriteValidationError writes a 400 Bad Request JSON error response including the field name.
func WriteValidationError(w http.ResponseWriter, field, message, requestID string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)

	resp := ErrorResponse{
		Error: ErrorDetail{
			Code:      ErrValidationFailed,
			Message:   message,
			RequestID: requestID,
			Field:     field,
		},
	}

	_ = json.NewEncoder(w).Encode(resp)
}

// WriteInternalError writes a 500 Internal Server Error response.
// It never exposes internal error details to the client.
func WriteInternalError(w http.ResponseWriter, requestID string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)

	resp := ErrorResponse{
		Error: ErrorDetail{
			Code:      ErrInternal,
			Message:   "An internal error occurred. Please try again.",
			RequestID: requestID,
		},
	}

	_ = json.NewEncoder(w).Encode(resp)
}

// CodeToHTTPStatus maps error code strings to their corresponding HTTP status codes.
func CodeToHTTPStatus(code string) int {
	switch code {
	case ErrUnauthorized, ErrTokenInvalid, ErrTokenExpired, ErrTokenBlacklisted,
		ErrOTPInvalid, ErrOTPExpired, ErrOTPNotFound:
		return http.StatusUnauthorized

	case ErrForbidden, ErrWrongUserType:
		return http.StatusForbidden

	case ErrProductNotFound, ErrStoreNotFound, ErrOrderNotFound, ErrUserNotFound,
		ErrCartNotFound, ErrBarcodeNotFound, ErrNotFound, ErrGRNNotFound,
		ErrDeviceNotFound, ErrCategoryNotFound:
		return http.StatusNotFound

	case ErrStoreAlreadyBound, ErrOrderAlreadyExists, ErrIdempotencyConflict,
		ErrConflict, ErrQRAlreadyUsed, ErrReturnAlreadyExists:
		return http.StatusConflict

	case ErrRateLimitExceeded:
		return http.StatusTooManyRequests

	case ErrValidationFailed, ErrInvalidPhone, ErrBarcodeInvalid,
		ErrInvalidQuantity, ErrGSTINInvalid, ErrRequestTooLarge:
		return http.StatusBadRequest

	case ErrInternal, ErrTimeout:
		return http.StatusInternalServerError

	case ErrServiceUnavailable, ErrPaymentGatewayDown:
		return http.StatusServiceUnavailable

	default:
		return http.StatusBadRequest
	}
}
