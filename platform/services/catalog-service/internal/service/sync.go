package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/zippyra/platform/services/catalog-service/internal/model"
	"github.com/zippyra/platform/services/catalog-service/internal/repository"
	sharedErrors "github.com/zippyra/platform/shared/errors"
)

type SyncService interface {
	Sync(ctx context.Context, req *model.SyncRequest) (*model.SyncResponse, error)
}

type syncService struct {
	productRepo repository.ProductRepo
}

func NewSyncService(productRepo repository.ProductRepo) SyncService {
	return &syncService{
		productRepo: productRepo,
	}
}

func (s *syncService) Sync(ctx context.Context, req *model.SyncRequest) (*model.SyncResponse, error) {
	if err := s.validateSyncRequest(req); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	products, err := s.productRepo.Sync(ctx, req.StoreID, req.LastSyncSeq, req.Limit)
	if err != nil {
		return nil, &sharedErrors.AppError{Code: sharedErrors.ErrInternal, Message: "failed to sync products"}
	}

	var nextSyncSeq int64 = req.LastSyncSeq
	var hasMore bool
	var totalChanges int

	if len(products) > 0 {
		lastProduct := products[len(products)-1]
		nextSyncSeq = lastProduct.SyncSeq

		var nextBatch []model.Product
		nextBatch, err = s.productRepo.Sync(ctx, req.StoreID, nextSyncSeq, 1)
		if err != nil {
			log.Warn().Err(err).Str("storeID", req.StoreID.String()).Msg("failed to check for more products")
			hasMore = false
		} else {
			hasMore = len(nextBatch) > 0
		}

		totalChanges = len(products)
	}

	return &model.SyncResponse{
		Products:     products,
		NextSyncSeq:  nextSyncSeq,
		HasMore:      hasMore,
		TotalChanges: totalChanges,
	}, nil
}

func (s *syncService) validateSyncRequest(req *model.SyncRequest) error {
	if req.StoreID == uuid.Nil {
		return &sharedErrors.AppError{Code: sharedErrors.ErrValidationFailed, Message: "store_id is required"}
	}
	if req.LastSyncSeq < 0 {
		return &sharedErrors.AppError{Code: sharedErrors.ErrValidationFailed, Message: "last_sync_seq must be greater than or equal to 0"}
	}
	if req.Limit < 1 {
		return &sharedErrors.AppError{Code: sharedErrors.ErrValidationFailed, Message: "limit must be greater than 0"}
	}
	if req.Limit > 1000 {
		return &sharedErrors.AppError{Code: sharedErrors.ErrValidationFailed, Message: "limit must be less than or equal to 1000"}
	}
	return nil
}
