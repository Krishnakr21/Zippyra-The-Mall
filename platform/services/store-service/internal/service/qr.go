package service

import (
	"context"
	"time"

	"github.com/zippyra/platform/services/store-service/internal/model"
	"github.com/zippyra/platform/shared/errors"
)

// QRService handles QR token validation.
type QRService struct {
	qrRepo  QRTokenRepo
	nowFunc func() time.Time
}

// NewQRService creates a new QRService.
func NewQRService(qrRepo QRTokenRepo) *QRService {
	return &QRService{
		qrRepo:  qrRepo,
		nowFunc: time.Now,
	}
}

// ValidateQRToken validates a QR token for store entry.
func (s *QRService) ValidateQRToken(ctx context.Context, token string) (*model.StoreQRToken, error) {
	now := s.nowFunc()

	if token == "" {
		return nil, &errors.AppError{
			Code: errors.ErrQRTokenInvalid, Message: "QR token is required",
			HTTPStatus: 400,
		}
	}

	qrToken, err := s.qrRepo.GetActiveToken(ctx, token)
	if err != nil {
		return nil, &errors.AppError{
			Code: errors.ErrInternal, Message: "Failed to validate QR token",
			HTTPStatus: 500, Err: err,
		}
	}
	if qrToken == nil {
		return nil, &errors.AppError{
			Code: errors.ErrQRTokenInvalid, Message: "QR token is invalid or inactive",
			HTTPStatus: 400,
		}
	}

	// Check expiry
	if qrToken.ExpiresAt.Before(now) {
		return nil, &errors.AppError{
			Code: errors.ErrQRTokenExpired, Message: "QR token has expired",
			HTTPStatus: 400,
		}
	}

	// Check token type
	if qrToken.TokenType != "ENTRANCE" {
		return nil, &errors.AppError{
			Code: errors.ErrQRTokenInvalid, Message: "QR token is not an entrance token",
			HTTPStatus: 400,
		}
	}

	return qrToken, nil
}
