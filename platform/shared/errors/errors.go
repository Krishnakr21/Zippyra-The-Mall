package errors

import (
	"errors"
	"fmt"
	"net/http"
)

// AppError represents a custom error with standard properties.
type AppError struct {
	Code       string
	Message    string
	HTTPStatus int
	Err        error
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("code: %s, message: %s, err: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("code: %s, message: %s", e.Code, e.Message)
}

func (e *AppError) Unwrap() error {
	return e.Err
}

// As support for Go error wrapping
func (e *AppError) As(target interface{}) bool {
	if t, ok := target.(**AppError); ok {
		*t = e
		return true
	}
	return false
}

// Is support for Go error wrapping
func (e *AppError) Is(target error) bool {
	t, ok := target.(*AppError)
	if !ok {
		return false
	}
	return e.Code == t.Code
}

// Error definitions
const (
	CodeNotFound      = "NOT_FOUND"
	CodeValidation    = "VALIDATION_ERROR"
	CodeUnauthorized  = "UNAUTHORIZED"
	CodeConflict      = "CONFLICT"
	CodeInternalError = "INTERNAL_ERROR"
)

// NotFoundError creates a new NOT_FOUND error.
func NotFoundError(message string, err error) error {
	return &AppError{Code: CodeNotFound, Message: message, HTTPStatus: http.StatusNotFound, Err: err}
}

// ValidationError creates a new VALIDATION_ERROR.
func ValidationError(message string, err error) error {
	return &AppError{Code: CodeValidation, Message: message, HTTPStatus: http.StatusBadRequest, Err: err}
}

// UnauthorizedError creates a new UNAUTHORIZED error.
func UnauthorizedError(message string, err error) error {
	return &AppError{Code: CodeUnauthorized, Message: message, HTTPStatus: http.StatusUnauthorized, Err: err}
}

// ConflictError creates a new CONFLICT error.
func ConflictError(message string, err error) error {
	return &AppError{Code: CodeConflict, Message: message, HTTPStatus: http.StatusConflict, Err: err}
}

// InternalError creates a new INTERNAL_ERROR.
func InternalError(message string, err error) error {
	return &AppError{Code: CodeInternalError, Message: message, HTTPStatus: http.StatusInternalServerError, Err: err}
}

// IsNotFound checks if the error is a NotFoundError.
func IsNotFound(err error) bool {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Code == CodeNotFound
	}
	return false
}

// IsValidation checks if the error is a ValidationError.
func IsValidation(err error) bool {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Code == CodeValidation
	}
	return false
}

// IsUnauthorized checks if the error is an UnauthorizedError.
func IsUnauthorized(err error) bool {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Code == CodeUnauthorized
	}
	return false
}

// IsConflict checks if the error is a ConflictError.
func IsConflict(err error) bool {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Code == CodeConflict
	}
	return false
}

// IsInternal checks if the error is an InternalError.
func IsInternal(err error) bool {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Code == CodeInternalError
	}
	return false
}
