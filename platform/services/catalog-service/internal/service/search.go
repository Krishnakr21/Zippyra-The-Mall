package service

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/zippyra/platform/services/catalog-service/internal/model"
	"github.com/zippyra/platform/services/catalog-service/internal/repository"
	catalogSearch "github.com/zippyra/platform/services/catalog-service/internal/search"
	sharedErrors "github.com/zippyra/platform/shared/errors"
)

type SearchService interface {
	Search(ctx context.Context, req *model.ProductSearchRequest) (*model.ProductListResponse, error)
	ByCategory(ctx context.Context, storeID uuid.UUID, category string, page, limit int) (*model.ProductListResponse, error)
	IndexProduct(ctx context.Context, p model.Product) error
}

type SearchClient interface {
	Search(ctx context.Context, storeID, query, category string, page, limit int) ([]model.Product, int, error)
	IndexProduct(ctx context.Context, p model.Product) error
}

type searchService struct {
	productRepo repository.ProductRepo
	search      SearchClient
}

func NewSearchService(productRepo repository.ProductRepo, search SearchClient) SearchService {
	return &searchService{
		productRepo: productRepo,
		search:      search,
	}
}

func (s *searchService) Search(ctx context.Context, req *model.ProductSearchRequest) (*model.ProductListResponse, error) {
	if err := s.validateSearchRequest(req); err != nil {
		return nil, err
	}

	sanitizedQuery := catalogSearch.SanitizeQuery(req.Query)

	categoryStr := ""
	if req.Category != nil {
		categoryStr = *req.Category
	}

	products, total, err := s.search.Search(ctx, req.StoreID.String(), sanitizedQuery, categoryStr, req.Page, req.Limit)
	if err != nil {
		log.Warn().Err(err).Str("storeID", req.StoreID.String()).Str("query", req.Query).Msg("opensearch search failed, falling back to database")

		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		products, total, err = s.productRepo.Search(ctx, req.StoreID, sanitizedQuery, req.Category, req.Page, req.Limit)
		if err != nil {
			return nil, &sharedErrors.AppError{Code: sharedErrors.ErrInternal, Message: "search failed"}
		}
	}

	return &model.ProductListResponse{
		Products: products,
		Total:    total,
		Page:     req.Page,
		Limit:    req.Limit,
	}, nil
}

func (s *searchService) ByCategory(ctx context.Context, storeID uuid.UUID, category string, page, limit int) (*model.ProductListResponse, error) {
	if storeID == uuid.Nil {
		return nil, &sharedErrors.AppError{Code: sharedErrors.ErrValidationFailed, Message: "store_id is required"}
	}
	if category == "" {
		return nil, &sharedErrors.AppError{Code: sharedErrors.ErrValidationFailed, Message: "category is required"}
	}
	if page < 1 {
		return nil, &sharedErrors.AppError{Code: sharedErrors.ErrValidationFailed, Message: "page must be greater than 0"}
	}
	if limit < 1 {
		return nil, &sharedErrors.AppError{Code: sharedErrors.ErrValidationFailed, Message: "limit must be greater than 0"}
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	products, total, err := s.productRepo.GetByCategory(ctx, storeID, category, page, limit)
	if err != nil {
		return nil, &sharedErrors.AppError{Code: sharedErrors.ErrInternal, Message: "failed to get products by category"}
	}

	return &model.ProductListResponse{
		Products: products,
		Total:    total,
		Page:     page,
		Limit:    limit,
	}, nil
}

func (s *searchService) IndexProduct(ctx context.Context, p model.Product) error {
	if s.search != nil {
		if err := s.search.IndexProduct(ctx, p); err != nil {
			log.Warn().Err(err).Str("productID", p.ID.String()).Msg("failed to index product in opensearch")
			return err
		}
	}
	return nil
}

func (s *searchService) validateSearchRequest(req *model.ProductSearchRequest) error {
	if req.StoreID == uuid.Nil {
		return &sharedErrors.AppError{Code: sharedErrors.ErrValidationFailed, Message: "store_id is required"}
	}
	if strings.TrimSpace(req.Query) == "" {
		return &sharedErrors.AppError{Code: sharedErrors.ErrValidationFailed, Message: "query is required"}
	}
	if req.Page < 1 {
		return &sharedErrors.AppError{Code: sharedErrors.ErrValidationFailed, Message: "page must be greater than 0"}
	}
	if req.Limit < 1 {
		return &sharedErrors.AppError{Code: sharedErrors.ErrValidationFailed, Message: "limit must be greater than 0"}
	}
	if req.Limit > 100 {
		return &sharedErrors.AppError{Code: sharedErrors.ErrValidationFailed, Message: "limit must be less than or equal to 100"}
	}

	return nil
}
