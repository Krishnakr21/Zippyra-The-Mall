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

type OfferService interface {
	GetOffers(ctx context.Context, storeID uuid.UUID) (*model.OfferResponse, error)
}

type OfferCache interface {
	GetOffers(ctx context.Context, key string) (*model.OfferResponse, error)
	SetOffers(ctx context.Context, key string, response *model.OfferResponse) error
}

type offerService struct {
	offerRepo repository.OfferRepo
	cache     OfferCache
}

func NewOfferService(offerRepo repository.OfferRepo, cache OfferCache) OfferService {
	return &offerService{
		offerRepo: offerRepo,
		cache:     cache,
	}
}

func (s *offerService) GetOffers(ctx context.Context, storeID uuid.UUID) (*model.OfferResponse, error) {
	if storeID == uuid.Nil {
		return nil, &sharedErrors.AppError{Code: sharedErrors.ErrValidationFailed, Message: "store_id is required"}
	}

	cacheKey := "offer_rules:" + storeID.String()

	cachedOffers, err := s.cache.GetOffers(ctx, cacheKey)
	if err != nil {
		log.Warn().Err(err).Str("storeID", storeID.String()).Msg("cache lookup failed for offers, falling back to database")
	} else if cachedOffers != nil {
		return cachedOffers, nil
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	offers, err := s.offerRepo.GetActiveByStore(ctx, storeID)
	if err != nil {
		return nil, &sharedErrors.AppError{Code: sharedErrors.ErrInternal, Message: "failed to get offers"}
	}

	response := &model.OfferResponse{
		Offers: offers,
	}

	if err := s.cache.SetOffers(ctx, cacheKey, response); err != nil {
		log.Warn().Err(err).Str("storeID", storeID.String()).Msg("failed to cache offers")
	}

	return response, nil
}
