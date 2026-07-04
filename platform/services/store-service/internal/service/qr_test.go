package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/zippyra/platform/services/store-service/internal/model"
	"github.com/zippyra/platform/shared/errors"
)

func TestValidateQRToken_Success(t *testing.T) {
	storeID := uuid.New()
	tokenID := uuid.New()

	qrRepo := &mockQRTokenRepo{
		getActiveTokenFn: func(_ context.Context, _ string) (*model.StoreQRToken, error) {
			return &model.StoreQRToken{
				ID: tokenID, StoreID: storeID, Token: "valid-token",
				TokenType: "ENTRANCE", IsActive: true,
				ExpiresAt: time.Date(2024, 12, 31, 23, 59, 59, 0, time.UTC),
			}, nil
		},
	}

	svc := NewQRService(qrRepo)
	svc.nowFunc = func() time.Time {
		return time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	}

	token, err := svc.ValidateQRToken(context.Background(), "valid-token")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if token.ID != tokenID {
		t.Errorf("expected token ID %s, got %s", tokenID, token.ID)
	}
	if token.StoreID != storeID {
		t.Errorf("expected store ID %s, got %s", storeID, token.StoreID)
	}
}

func TestValidateQRToken_Empty(t *testing.T) {
	svc := NewQRService(&mockQRTokenRepo{})
	_, err := svc.ValidateQRToken(context.Background(), "")

	if err == nil {
		t.Fatal("expected error for empty token")
	}
	appErr, ok := err.(*errors.AppError)
	if !ok {
		t.Fatalf("expected AppError, got %T", err)
	}
	if appErr.Code != errors.ErrQRTokenInvalid {
		t.Errorf("expected code %s, got %s", errors.ErrQRTokenInvalid, appErr.Code)
	}
}

func TestValidateQRToken_NotFound(t *testing.T) {
	qrRepo := &mockQRTokenRepo{
		getActiveTokenFn: func(_ context.Context, _ string) (*model.StoreQRToken, error) {
			return nil, nil
		},
	}

	svc := NewQRService(qrRepo)
	_, err := svc.ValidateQRToken(context.Background(), "missing-token")

	if err == nil {
		t.Fatal("expected error for missing token")
	}
	appErr, ok := err.(*errors.AppError)
	if !ok {
		t.Fatalf("expected AppError, got %T", err)
	}
	if appErr.Code != errors.ErrQRTokenInvalid {
		t.Errorf("expected code %s, got %s", errors.ErrQRTokenInvalid, appErr.Code)
	}
}

func TestValidateQRToken_Expired(t *testing.T) {
	qrRepo := &mockQRTokenRepo{
		getActiveTokenFn: func(_ context.Context, _ string) (*model.StoreQRToken, error) {
			return &model.StoreQRToken{
				ID: uuid.New(), StoreID: uuid.New(), Token: "old-token",
				TokenType: "ENTRANCE", IsActive: true,
				ExpiresAt: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			}, nil
		},
	}

	svc := NewQRService(qrRepo)
	svc.nowFunc = func() time.Time {
		return time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	}
	_, err := svc.ValidateQRToken(context.Background(), "old-token")

	if err == nil {
		t.Fatal("expected error for expired token")
	}
	appErr, ok := err.(*errors.AppError)
	if !ok {
		t.Fatalf("expected AppError, got %T", err)
	}
	if appErr.Code != errors.ErrQRTokenExpired {
		t.Errorf("expected code %s, got %s", errors.ErrQRTokenExpired, appErr.Code)
	}
}

func TestValidateQRToken_NonEntrance(t *testing.T) {
	qrRepo := &mockQRTokenRepo{
		getActiveTokenFn: func(_ context.Context, _ string) (*model.StoreQRToken, error) {
			return &model.StoreQRToken{
				ID: uuid.New(), StoreID: uuid.New(), Token: "product-qr",
				TokenType: "PRODUCT", IsActive: true,
				ExpiresAt: time.Date(2024, 12, 31, 23, 59, 59, 0, time.UTC),
			}, nil
		},
	}

	svc := NewQRService(qrRepo)
	svc.nowFunc = func() time.Time {
		return time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	}
	_, err := svc.ValidateQRToken(context.Background(), "product-qr")

	if err == nil {
		t.Fatal("expected error for non-ENTRANCE token")
	}
	appErr, ok := err.(*errors.AppError)
	if !ok {
		t.Fatalf("expected AppError, got %T", err)
	}
	if appErr.Code != errors.ErrQRTokenInvalid {
		t.Errorf("expected code %s, got %s", errors.ErrQRTokenInvalid, appErr.Code)
	}
}

func TestValidateQRToken_DBError(t *testing.T) {
	qrRepo := &mockQRTokenRepo{
		getActiveTokenFn: func(_ context.Context, _ string) (*model.StoreQRToken, error) {
			return nil, context.DeadlineExceeded
		},
	}

	svc := NewQRService(qrRepo)
	_, err := svc.ValidateQRToken(context.Background(), "any-token")

	if err == nil {
		t.Fatal("expected error for DB failure")
	}
	appErr, ok := err.(*errors.AppError)
	if !ok {
		t.Fatalf("expected AppError, got %T", err)
	}
	if appErr.Code != errors.ErrInternal {
		t.Errorf("expected code %s, got %s", errors.ErrInternal, appErr.Code)
	}
}
