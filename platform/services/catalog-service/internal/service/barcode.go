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

type SearchIndexService interface {
	IndexProduct(ctx context.Context, p model.Product) error
}

type BarcodeService interface {
	Lookup(ctx context.Context, storeID uuid.UUID, barcode string) (*model.ProductResponse, error)
	UpsertProduct(ctx context.Context, req *model.UpsertProductRequest) (*model.Product, error)
}

type SKUCache interface {
	Get(ctx context.Context, storeID, barcode string) (*model.Product, error)
	Set(ctx context.Context, storeID, barcode string, product *model.Product) error
	Invalidate(ctx context.Context, storeID, barcode string) error
}

type barcodeService struct {
	productRepo repository.ProductRepo
	cache       SKUCache
	search      SearchIndexService
}

func NewBarcodeService(productRepo repository.ProductRepo, cache SKUCache, searchService SearchIndexService) BarcodeService {
	return &barcodeService{
		productRepo: productRepo,
		cache:       cache,
		search:      searchService,
	}
}

func (s *barcodeService) Lookup(ctx context.Context, storeID uuid.UUID, barcode string) (*model.ProductResponse, error) {
	if barcode == "" {
		return nil, &sharedErrors.AppError{Code: sharedErrors.ErrValidationFailed, Message: "barcode cannot be empty"}
	}

	cacheHit := false

	product, err := s.cache.Get(ctx, storeID.String(), barcode)
	if err != nil {
		log.Warn().Err(err).Str("storeID", storeID.String()).Str("barcode", barcode).Msg("cache lookup failed, falling back to database")
	} else if product != nil {
		cacheHit = true
		log.Debug().Str("storeID", storeID.String()).Str("barcode", barcode).Msg("cache hit")
	}

	if product == nil {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		product, err = s.productRepo.GetByStoreAndBarcode(ctx, storeID, barcode)
		if err != nil {
			if err.Error() == "failed to get product by store and barcode: no rows in result set" {
				return nil, &sharedErrors.AppError{Code: sharedErrors.ErrBarcodeNotFound, Message: "product not found"}
			}
			return nil, &sharedErrors.AppError{Code: sharedErrors.ErrInternal, Message: "database error"}
		}

		if err := s.cache.Set(ctx, storeID.String(), barcode, product); err != nil {
			log.Warn().Err(err).Str("storeID", storeID.String()).Str("barcode", barcode).Msg("failed to cache product")
		}
	}

	return &model.ProductResponse{
		Product:  *product,
		CacheHit: cacheHit,
	}, nil
}

func (s *barcodeService) UpsertProduct(ctx context.Context, req *model.UpsertProductRequest) (*model.Product, error) {
	if err := s.validateUpsertRequest(req); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	product := &model.Product{
		StoreID:       req.StoreID,
		Barcode:       req.Barcode,
		Name:          req.Name,
		Description:   req.Description,
		Brand:         req.Brand,
		Category:      req.Category,
		HSNCode:       req.HSNCode,
		MRP:           req.MRP,
		SellingPrice:  req.SellingPrice,
		GSTRate:       req.GSTRate,
		Unit:          req.Unit,
		ImageURL:      req.ImageURL,
		ThumbnailURL:  req.ThumbnailURL,
		StockQuantity: req.StockQuantity,
		IsActive:      true,
		IsReturnable:  true,
	}

	if req.IsReturnable != nil {
		product.IsReturnable = *req.IsReturnable
	}

	if req.ReorderPoint != nil {
		product.ReorderPoint = *req.ReorderPoint
	}

	if err := s.productRepo.Upsert(ctx, product); err != nil {
		return nil, &sharedErrors.AppError{Code: sharedErrors.ErrInternal, Message: "failed to upsert product"}
	}

	if err := s.cache.Invalidate(ctx, req.StoreID.String(), req.Barcode); err != nil {
		log.Warn().Err(err).Str("storeID", req.StoreID.String()).Str("barcode", req.Barcode).Msg("failed to invalidate cache")
	}

	if err := s.search.IndexProduct(ctx, *product); err != nil {
		log.Warn().Err(err).Str("productID", product.ID.String()).Msg("failed to index product in search")
	}

	return product, nil
}

func (s *barcodeService) validateUpsertRequest(req *model.UpsertProductRequest) error {
	if req.StoreID == uuid.Nil {
		return &sharedErrors.AppError{Code: sharedErrors.ErrValidationFailed, Message: "store_id is required"}
	}
	if req.Barcode == "" {
		return &sharedErrors.AppError{Code: sharedErrors.ErrValidationFailed, Message: "barcode is required"}
	}
	if req.Name == "" {
		return &sharedErrors.AppError{Code: sharedErrors.ErrValidationFailed, Message: "name is required"}
	}
	if req.Brand == "" {
		return &sharedErrors.AppError{Code: sharedErrors.ErrValidationFailed, Message: "brand is required"}
	}
	if req.Category == "" {
		return &sharedErrors.AppError{Code: sharedErrors.ErrValidationFailed, Message: "category is required"}
	}
	if req.HSNCode == "" {
		return &sharedErrors.AppError{Code: sharedErrors.ErrValidationFailed, Message: "hsn_code is required"}
	}
	if req.MRP <= 0 {
		return &sharedErrors.AppError{Code: sharedErrors.ErrValidationFailed, Message: "mrp must be greater than 0"}
	}
	if req.SellingPrice <= 0 {
		return &sharedErrors.AppError{Code: sharedErrors.ErrValidationFailed, Message: "selling_price must be greater than 0"}
	}
	if req.GSTRate < 0 || req.GSTRate > 100 {
		return &sharedErrors.AppError{Code: sharedErrors.ErrValidationFailed, Message: "gst_rate must be between 0 and 100"}
	}
	if req.Unit == "" {
		return &sharedErrors.AppError{Code: sharedErrors.ErrValidationFailed, Message: "unit is required"}
	}
	if req.StockQuantity < 0 {
		return &sharedErrors.AppError{Code: sharedErrors.ErrValidationFailed, Message: "stock_quantity must be greater than or equal to 0"}
	}
	if req.ReorderPoint != nil && *req.ReorderPoint < 0 {
		return &sharedErrors.AppError{Code: sharedErrors.ErrValidationFailed, Message: "reorder_point must be greater than or equal to 0"}
	}
	if req.ReorderQuantity != nil && *req.ReorderQuantity < 0 {
		return &sharedErrors.AppError{Code: sharedErrors.ErrValidationFailed, Message: "reorder_quantity must be greater than or equal to 0"}
	}

	return nil
}
