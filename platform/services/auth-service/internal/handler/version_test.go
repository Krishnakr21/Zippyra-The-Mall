package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/zippyra/platform/services/auth-service/internal/model"
)

type stubAppVersionRepo struct {
	getFn func(ctx context.Context, platform string) (*model.AppVersion, error)
}

func (s *stubAppVersionRepo) GetLatest(ctx context.Context, platform string) (*model.AppVersion, error) {
	return s.getFn(ctx, platform)
}

func TestCompareSemver(t *testing.T) {
	if compareSemver("1.0.0", "1.0.0") != 0 {
		t.Fatal("expected equal")
	}
	if compareSemver("1.0.0", "1.0.1") != -1 {
		t.Fatal("expected -1")
	}
	if compareSemver("2.0.0", "1.9.9") != 1 {
		t.Fatal("expected 1")
	}
}

func TestParseVersionPart(t *testing.T) {
	if parseVersionPart([]string{"1", "2", "3"}, 0) != 1 {
		t.Fatal("expected 1")
	}
	if parseVersionPart([]string{"1", "2", "3"}, 5) != 0 {
		t.Fatal("expected 0")
	}
	if parseVersionPart([]string{"x9"}, 0) != 9 {
		t.Fatal("expected 9")
	}
}

func TestVersionHandler_InvalidPlatform(t *testing.T) {
	repo := &stubAppVersionRepo{getFn: func(ctx context.Context, platform string) (*model.AppVersion, error) {
		return nil, nil
	}}
	h := NewVersionHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/auth/version-check?platform=web&version=1.0.0", nil)
	rec := httptest.NewRecorder()
	h.VersionCheck(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestVersionHandler_InvalidVersion(t *testing.T) {
	repo := &stubAppVersionRepo{getFn: func(ctx context.Context, platform string) (*model.AppVersion, error) {
		return nil, nil
	}}
	h := NewVersionHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/auth/version-check?platform=android&version=bad", nil)
	rec := httptest.NewRecorder()
	h.VersionCheck(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestVersionHandler_InternalError(t *testing.T) {
	repo := &stubAppVersionRepo{getFn: func(ctx context.Context, platform string) (*model.AppVersion, error) {
		return nil, context.Canceled
	}}
	h := NewVersionHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/auth/version-check?platform=android&version=1.0.0", nil)
	rec := httptest.NewRecorder()
	h.VersionCheck(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

func TestVersionHandler_Success_ForceUpdate(t *testing.T) {
	now := time.Now()
	repo := &stubAppVersionRepo{getFn: func(ctx context.Context, platform string) (*model.AppVersion, error) {
		notes := "hi"
		return &model.AppVersion{ID: uuid.New(), Platform: platform, Version: "2.0.0", MinSupportedVersion: "2.0.0", IsForceUpdate: false, ReleaseNotes: &notes, CreatedAt: now, UpdatedAt: now}, nil
	}}
	h := NewVersionHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/auth/version-check?platform=android&version=1.0.0", nil)
	rec := httptest.NewRecorder()
	h.VersionCheck(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}
