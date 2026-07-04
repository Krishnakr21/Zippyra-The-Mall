package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"

	mw "github.com/zippyra/platform/services/auth-service/internal/middleware"
	"github.com/zippyra/platform/services/auth-service/internal/service"
	"github.com/zippyra/platform/shared/errors"
)

// SessionHandler handles session management endpoints.
type SessionHandler struct {
	authService    sessionAuthService
	sessionService sessionService
	jwtMiddleware  tokenValidator
}

type sessionAuthService interface {
	RefreshAccessToken(ctx context.Context, refreshTokenStr string) (string, string, error)
	Logout(ctx context.Context, claims *mw.Claims) error
}

type sessionService interface {
	ListSessions(ctx context.Context, userID, currentDeviceID string) ([]service.SessionInfo, error)
	RevokeSession(ctx context.Context, userID, sessionID string) error
	RevokeAllSessions(ctx context.Context, userID, currentDeviceID string) (int, error)
}

type tokenValidator interface {
	ValidateToken(tokenString string) (*mw.Claims, error)
}

// NewSessionHandler creates a new SessionHandler.
func NewSessionHandler(authService sessionAuthService, sessionService sessionService, jwtMiddleware tokenValidator) *SessionHandler {
	return &SessionHandler{
		authService:    authService,
		sessionService: sessionService,
		jwtMiddleware:  jwtMiddleware,
	}
}

// Refresh handles POST /v1/auth/refresh
func (h *SessionHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	requestID := r.Header.Get("X-Request-ID")

	// Extract refresh token from Authorization header
	authHeader := r.Header.Get("Authorization")
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || parts[0] != "Bearer" {
		errors.WriteError(w, http.StatusUnauthorized, errors.ErrTokenInvalid,
			"Invalid Authorization header", requestID)
		return
	}
	refreshToken := parts[1]

	newAccessToken, errCode, err := h.authService.RefreshAccessToken(r.Context(), refreshToken)
	if err != nil {
		statusCode := http.StatusUnauthorized
		if errCode == errors.ErrInternal {
			statusCode = http.StatusInternalServerError
		}
		errors.WriteError(w, statusCode, errCode, err.Error(), requestID)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"access_token": newAccessToken,
		"expires_in":   86400,
	})
}

// Logout handles POST /v1/auth/logout
func (h *SessionHandler) Logout(w http.ResponseWriter, r *http.Request) {
	requestID := r.Header.Get("X-Request-ID")

	// Extract token to get claims for blacklisting
	authHeader := r.Header.Get("Authorization")
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 {
		errors.WriteError(w, http.StatusUnauthorized, errors.ErrTokenInvalid,
			"Invalid Authorization header", requestID)
		return
	}

	claims, err := h.jwtMiddleware.ValidateToken(parts[1])
	if err != nil {
		errors.WriteError(w, http.StatusUnauthorized, errors.ErrTokenInvalid,
			"Invalid token", requestID)
		return
	}

	if err := h.authService.Logout(r.Context(), claims); err != nil {
		log.Error().Err(err).Msg("logout failed")
		errors.WriteInternalError(w, requestID)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Logged out successfully",
	})
}

// ListSessions handles GET /v1/auth/sessions
func (h *SessionHandler) ListSessions(w http.ResponseWriter, r *http.Request) {
	requestID := r.Header.Get("X-Request-ID")
	userID := mw.GetUserIDFromContext(r.Context())
	deviceID := mw.GetDeviceIDFromContext(r.Context())

	sessions, err := h.sessionService.ListSessions(r.Context(), userID, deviceID)
	if err != nil {
		log.Error().Err(err).Msg("list sessions failed")
		errors.WriteInternalError(w, requestID)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"sessions": sessions,
	})
}

// RevokeSession handles DELETE /v1/auth/sessions/{session_id}
func (h *SessionHandler) RevokeSession(w http.ResponseWriter, r *http.Request) {
	requestID := r.Header.Get("X-Request-ID")
	userID := mw.GetUserIDFromContext(r.Context())
	sessionID := chi.URLParam(r, "session_id")

	if err := h.sessionService.RevokeSession(r.Context(), userID, sessionID); err != nil {
		if err.Error() == "session not found" {
			errors.WriteError(w, http.StatusNotFound, errors.ErrNotFound,
				"Session not found", requestID)
		} else {
			log.Error().Err(err).Msg("revoke session failed")
			errors.WriteInternalError(w, requestID)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Session revoked",
	})
}

// RevokeAllSessions handles DELETE /v1/auth/sessions
func (h *SessionHandler) RevokeAllSessions(w http.ResponseWriter, r *http.Request) {
	requestID := r.Header.Get("X-Request-ID")
	userID := mw.GetUserIDFromContext(r.Context())
	deviceID := mw.GetDeviceIDFromContext(r.Context())

	count, err := h.sessionService.RevokeAllSessions(r.Context(), userID, deviceID)
	if err != nil {
		log.Error().Err(err).Msg("revoke all sessions failed")
		errors.WriteInternalError(w, requestID)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":       "All other sessions revoked",
		"revoked_count": count,
	})
}
