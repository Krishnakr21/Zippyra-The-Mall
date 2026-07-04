package handler

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	mw "github.com/zippyra/platform/services/auth-service/internal/middleware"
	"github.com/zippyra/platform/services/auth-service/internal/service"
	sharedErrors "github.com/zippyra/platform/shared/errors"
)

type stubSessionAuth struct {
	refreshFn func(ctx context.Context, refreshTokenStr string) (string, string, error)
	logoutFn  func(ctx context.Context, claims *mw.Claims) error
}

func (s *stubSessionAuth) RefreshAccessToken(ctx context.Context, refreshTokenStr string) (string, string, error) {
	return s.refreshFn(ctx, refreshTokenStr)
}
func (s *stubSessionAuth) Logout(ctx context.Context, claims *mw.Claims) error {
	return s.logoutFn(ctx, claims)
}

type stubSessionSvc struct {
	listFn      func(ctx context.Context, userID, currentDeviceID string) ([]service.SessionInfo, error)
	revokeFn    func(ctx context.Context, userID, sessionID string) error
	revokeAllFn func(ctx context.Context, userID, currentDeviceID string) (int, error)
}

func (s *stubSessionSvc) ListSessions(ctx context.Context, userID, currentDeviceID string) ([]service.SessionInfo, error) {
	return s.listFn(ctx, userID, currentDeviceID)
}
func (s *stubSessionSvc) RevokeSession(ctx context.Context, userID, sessionID string) error {
	return s.revokeFn(ctx, userID, sessionID)
}
func (s *stubSessionSvc) RevokeAllSessions(ctx context.Context, userID, currentDeviceID string) (int, error) {
	return s.revokeAllFn(ctx, userID, currentDeviceID)
}

type stubTokenValidator struct {
	claims *mw.Claims
	err    error
}

func (s *stubTokenValidator) ValidateToken(tokenString string) (*mw.Claims, error) {
	return s.claims, s.err
}

func TestSessionHandler_Refresh_BadAuthHeader(t *testing.T) {
	h := NewSessionHandler(&stubSessionAuth{}, &stubSessionSvc{}, &stubTokenValidator{})
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/refresh", nil)
	req.Header.Set("Authorization", "bad")
	rec := httptest.NewRecorder()
	h.Refresh(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestSessionHandler_Refresh_InternalError(t *testing.T) {
	auth := &stubSessionAuth{refreshFn: func(ctx context.Context, refreshTokenStr string) (string, string, error) {
		return "", sharedErrors.ErrInternal, errors.New("boom")
	}}
	h := NewSessionHandler(auth, &stubSessionSvc{}, &stubTokenValidator{})
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/refresh", nil)
	req.Header.Set("Authorization", "Bearer r")
	rec := httptest.NewRecorder()
	h.Refresh(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

func TestSessionHandler_Refresh_Success(t *testing.T) {
	auth := &stubSessionAuth{refreshFn: func(ctx context.Context, refreshTokenStr string) (string, string, error) {
		return "new-access", "", nil
	}}
	h := NewSessionHandler(auth, &stubSessionSvc{}, &stubTokenValidator{})
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/refresh", nil)
	req.Header.Set("Authorization", "Bearer r")
	rec := httptest.NewRecorder()
	h.Refresh(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestSessionHandler_Logout_BadAuthHeader(t *testing.T) {
	h := NewSessionHandler(&stubSessionAuth{}, &stubSessionSvc{}, &stubTokenValidator{})
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/logout", nil)
	req.Header.Set("Authorization", "bad")
	rec := httptest.NewRecorder()
	h.Logout(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestSessionHandler_Logout_InvalidToken(t *testing.T) {
	validator := &stubTokenValidator{err: errors.New("bad")}
	h := NewSessionHandler(&stubSessionAuth{}, &stubSessionSvc{}, validator)
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/logout", nil)
	req.Header.Set("Authorization", "Bearer t")
	rec := httptest.NewRecorder()
	h.Logout(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestSessionHandler_Logout_InternalError(t *testing.T) {
	claims := &mw.Claims{RegisteredClaims: jwt.RegisteredClaims{ID: "j", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))}, UserID: "u", DeviceID: "d", UserType: "CUSTOMER"}
	validator := &stubTokenValidator{claims: claims}
	auth := &stubSessionAuth{logoutFn: func(ctx context.Context, c *mw.Claims) error { return errors.New("x") }}
	h := NewSessionHandler(auth, &stubSessionSvc{}, validator)
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/logout", nil)
	req.Header.Set("Authorization", "Bearer t")
	rec := httptest.NewRecorder()
	h.Logout(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

func TestSessionHandler_Logout_Success(t *testing.T) {
	claims := &mw.Claims{RegisteredClaims: jwt.RegisteredClaims{ID: "j", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))}, UserID: "u", DeviceID: "d", UserType: "CUSTOMER"}
	validator := &stubTokenValidator{claims: claims}
	auth := &stubSessionAuth{logoutFn: func(ctx context.Context, c *mw.Claims) error { return nil }}
	h := NewSessionHandler(auth, &stubSessionSvc{}, validator)
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/logout", nil)
	req.Header.Set("Authorization", "Bearer t")
	rec := httptest.NewRecorder()
	h.Logout(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestSessionHandler_ListSessions_InternalError(t *testing.T) {
	svc := &stubSessionSvc{listFn: func(ctx context.Context, userID, currentDeviceID string) ([]service.SessionInfo, error) {
		return nil, errors.New("db")
	}}
	h := NewSessionHandler(&stubSessionAuth{}, svc, &stubTokenValidator{})

	ctx := context.WithValue(context.Background(), mw.ContextUserID, "u1")
	ctx = context.WithValue(ctx, mw.ContextDeviceID, "d1")
	req := httptest.NewRequest(http.MethodGet, "/v1/auth/sessions", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	h.ListSessions(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

func TestSessionHandler_ListSessions_Success(t *testing.T) {
	svc := &stubSessionSvc{listFn: func(ctx context.Context, userID, currentDeviceID string) ([]service.SessionInfo, error) {
		return []service.SessionInfo{{ID: "1"}}, nil
	}}
	h := NewSessionHandler(&stubSessionAuth{}, svc, &stubTokenValidator{})

	ctx := context.WithValue(context.Background(), mw.ContextUserID, "u1")
	ctx = context.WithValue(ctx, mw.ContextDeviceID, "d1")
	req := httptest.NewRequest(http.MethodGet, "/v1/auth/sessions", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	h.ListSessions(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestSessionHandler_RevokeSession_NotFound(t *testing.T) {
	svc := &stubSessionSvc{revokeFn: func(ctx context.Context, userID, sessionID string) error {
		return errors.New("session not found")
	}}
	h := NewSessionHandler(&stubSessionAuth{}, svc, &stubTokenValidator{})

	ctx := context.WithValue(context.Background(), mw.ContextUserID, "u1")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("session_id", uuid.New().String())
	ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)

	req := httptest.NewRequest(http.MethodDelete, "/v1/auth/sessions/1", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	h.RevokeSession(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestSessionHandler_RevokeSession_InternalError(t *testing.T) {
	svc := &stubSessionSvc{revokeFn: func(ctx context.Context, userID, sessionID string) error {
		return errors.New("db")
	}}
	h := NewSessionHandler(&stubSessionAuth{}, svc, &stubTokenValidator{})

	ctx := context.WithValue(context.Background(), mw.ContextUserID, "u1")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("session_id", uuid.New().String())
	ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)

	req := httptest.NewRequest(http.MethodDelete, "/v1/auth/sessions/1", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	h.RevokeSession(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

func TestSessionHandler_RevokeSession_Success(t *testing.T) {
	svc := &stubSessionSvc{revokeFn: func(ctx context.Context, userID, sessionID string) error { return nil }}
	h := NewSessionHandler(&stubSessionAuth{}, svc, &stubTokenValidator{})

	ctx := context.WithValue(context.Background(), mw.ContextUserID, "u1")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("session_id", uuid.New().String())
	ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)

	req := httptest.NewRequest(http.MethodDelete, "/v1/auth/sessions/1", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	h.RevokeSession(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestSessionHandler_RevokeAllSessions_InternalError(t *testing.T) {
	svc := &stubSessionSvc{revokeAllFn: func(ctx context.Context, userID, currentDeviceID string) (int, error) {
		return 0, errors.New("db")
	}}
	h := NewSessionHandler(&stubSessionAuth{}, svc, &stubTokenValidator{})

	ctx := context.WithValue(context.Background(), mw.ContextUserID, "u1")
	ctx = context.WithValue(ctx, mw.ContextDeviceID, "d1")
	req := httptest.NewRequest(http.MethodDelete, "/v1/auth/sessions", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	h.RevokeAllSessions(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

func TestSessionHandler_RevokeAllSessions_Success(t *testing.T) {
	svc := &stubSessionSvc{revokeAllFn: func(ctx context.Context, userID, currentDeviceID string) (int, error) {
		return 2, nil
	}}
	h := NewSessionHandler(&stubSessionAuth{}, svc, &stubTokenValidator{})

	ctx := context.WithValue(context.Background(), mw.ContextUserID, "u1")
	ctx = context.WithValue(ctx, mw.ContextDeviceID, "d1")
	req := httptest.NewRequest(http.MethodDelete, "/v1/auth/sessions", bytes.NewReader([]byte{})).WithContext(ctx)
	rec := httptest.NewRecorder()
	h.RevokeAllSessions(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}
