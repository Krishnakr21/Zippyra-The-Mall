package service

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/zippyra/platform/services/catalog-service/internal/model"
	sharedErrors "github.com/zippyra/platform/shared/errors"
)

type mockProductRepoFull struct {
	getByStoreAndBarcode func(ctx context.Context, storeID uuid.UUID, barcode string) (*model.Product, error)
	search               func(ctx context.Context, storeID uuid.UUID, query string, category *string, page, limit int) ([]model.Product, int, error)
	getByCategory        func(ctx context.Context, storeID uuid.UUID, category string, page, limit int) ([]model.Product, int, error)
	sync                 func(ctx context.Context, storeID uuid.UUID, lastSyncSeq int64, limit int) ([]model.Product, error)
	upsert               func(ctx context.Context, p *model.Product) error
	getByID              func(ctx context.Context, id uuid.UUID) (*model.Product, error)
}

func (m *mockProductRepoFull) GetByStoreAndBarcode(ctx context.Context, storeID uuid.UUID, barcode string) (*model.Product, error) {
	return m.getByStoreAndBarcode(ctx, storeID, barcode)
}

func (m *mockProductRepoFull) Search(ctx context.Context, storeID uuid.UUID, query string, category *string, page, limit int) ([]model.Product, int, error) {
	return m.search(ctx, storeID, query, category, page, limit)
}

func (m *mockProductRepoFull) GetByCategory(ctx context.Context, storeID uuid.UUID, category string, page, limit int) ([]model.Product, int, error) {
	return m.getByCategory(ctx, storeID, category, page, limit)
}

func (m *mockProductRepoFull) Sync(ctx context.Context, storeID uuid.UUID, lastSyncSeq int64, limit int) ([]model.Product, error) {
	return m.sync(ctx, storeID, lastSyncSeq, limit)
}

func (m *mockProductRepoFull) Upsert(ctx context.Context, p *model.Product) error {
	return m.upsert(ctx, p)
}

func (m *mockProductRepoFull) GetByID(ctx context.Context, id uuid.UUID) (*model.Product, error) {
	return m.getByID(ctx, id)
}

type mockCacheFull struct {
	get        func(ctx context.Context, storeID, barcode string) (*model.Product, error)
	set        func(ctx context.Context, storeID, barcode string, product *model.Product) error
	invalidate func(ctx context.Context, storeID, barcode string) error
}

func (m *mockCacheFull) Get(ctx context.Context, storeID, barcode string) (*model.Product, error) {
	return m.get(ctx, storeID, barcode)
}

func (m *mockCacheFull) Set(ctx context.Context, storeID, barcode string, product *model.Product) error {
	return m.set(ctx, storeID, barcode, product)
}

func (m *mockCacheFull) Invalidate(ctx context.Context, storeID, barcode string) error {
	return m.invalidate(ctx, storeID, barcode)
}

type mockSearchIndexServiceFull struct {
	indexProduct func(ctx context.Context, p model.Product) error
}

func (m *mockSearchIndexServiceFull) IndexProduct(ctx context.Context, p model.Product) error {
	return m.indexProduct(ctx, p)
}

func TestBarcodeService_FullCoverage(t *testing.T) {
	storeID := uuid.New()
	barcode := "123456789"

	tests := []struct {
		name        string
		repo        *mockProductRepoFull
		cache       *mockCacheFull
		search      *mockSearchIndexServiceFull
		barcode     string
		expected    *model.ProductResponse
		expectedErr error
	}{
		{
			name: "empty barcode validation",
			repo: &mockProductRepoFull{},
			cache: &mockCacheFull{
				get: func(ctx context.Context, storeID, barcode string) (*model.Product, error) {
					return nil, nil
				},
			},
			search:      &mockSearchIndexServiceFull{},
			barcode:     "",
			expected:    nil,
			expectedErr: &sharedErrors.AppError{Code: sharedErrors.ErrValidationFailed, Message: "barcode cannot be empty"},
		},
		{
			name: "cache hit",
			repo: &mockProductRepoFull{},
			cache: &mockCacheFull{
				get: func(ctx context.Context, storeID, barcode string) (*model.Product, error) {
					return &model.Product{
						ID:       uuid.New(),
						StoreID:  uuid.MustParse(storeID),
						Barcode:  barcode,
						Name:     "Test Product",
						IsActive: true,
					}, nil
				},
			},
			search: &mockSearchIndexServiceFull{},
			expected: &model.ProductResponse{
				Product: model.Product{
					Barcode:  barcode,
					Name:     "Test Product",
					IsActive: true,
				},
				CacheHit: true,
			},
			barcode:     "123456789",
			expectedErr: nil,
		},
		{
			name: "cache miss, database hit",
			repo: &mockProductRepoFull{
				getByStoreAndBarcode: func(ctx context.Context, storeID uuid.UUID, barcode string) (*model.Product, error) {
					return &model.Product{
						ID:       uuid.New(),
						StoreID:  storeID,
						Barcode:  barcode,
						Name:     "Test Product",
						IsActive: true,
					}, nil
				},
			},
			cache: &mockCacheFull{
				get: func(ctx context.Context, storeID, barcode string) (*model.Product, error) {
					return nil, nil
				},
				set: func(ctx context.Context, storeID, barcode string, product *model.Product) error {
					return nil
				},
			},
			search: &mockSearchIndexServiceFull{},
			expected: &model.ProductResponse{
				Product: model.Product{
					Barcode:  barcode,
					Name:     "Test Product",
					IsActive: true,
				},
				CacheHit: false,
			},
			barcode:     "123456789",
			expectedErr: nil,
		},
		{
			name: "database error",
			repo: &mockProductRepoFull{
				getByStoreAndBarcode: func(ctx context.Context, storeID uuid.UUID, barcode string) (*model.Product, error) {
					return nil, assert.AnError
				},
			},
			cache: &mockCacheFull{
				get: func(ctx context.Context, storeID, barcode string) (*model.Product, error) {
					return nil, nil
				},
			},
			search:      &mockSearchIndexServiceFull{},
			barcode:     "123456789",
			expected:    nil,
			expectedErr: assert.AnError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewBarcodeService(tt.repo, tt.cache, tt.search)

			result, err := service.Lookup(context.Background(), storeID, tt.barcode)

			if tt.expectedErr != nil {
				assert.Error(t, err)
				if appErr, ok := tt.expectedErr.(*sharedErrors.AppError); ok {
					if actualAppErr, ok := err.(*sharedErrors.AppError); ok {
						assert.Equal(t, appErr.Code, actualAppErr.Code)
						assert.Equal(t, appErr.Message, actualAppErr.Message)
					}
				}
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected.CacheHit, result.CacheHit)
				assert.Equal(t, tt.expected.Product.Barcode, result.Product.Barcode)
				assert.Equal(t, tt.expected.Product.Name, result.Product.Name)
				assert.Equal(t, tt.expected.Product.IsActive, result.Product.IsActive)
			}
		})
	}
}

func TestBarcodeService_UpsertProduct_FullCoverage(t *testing.T) {
	tests := []struct {
		name        string
		req         *model.UpsertProductRequest
		repo        *mockProductRepoFull
		cache       *mockCacheFull
		search      *mockSearchIndexServiceFull
		expectedErr error
	}{
		{
			name: "validation failed - empty store_id",
			req: &model.UpsertProductRequest{
				StoreID: uuid.Nil,
				Barcode: "123456789",
				Name:    "Test Product",
			},
			repo:        &mockProductRepoFull{},
			cache:       &mockCacheFull{},
			search:      &mockSearchIndexServiceFull{},
			expectedErr: &sharedErrors.AppError{Code: sharedErrors.ErrValidationFailed, Message: "store_id is required"},
		},
		{
			name: "validation failed - empty barcode",
			req: &model.UpsertProductRequest{
				StoreID: uuid.New(),
				Barcode: "",
				Name:    "Test Product",
			},
			repo:        &mockProductRepoFull{},
			cache:       &mockCacheFull{},
			search:      &mockSearchIndexServiceFull{},
			expectedErr: &sharedErrors.AppError{Code: sharedErrors.ErrValidationFailed, Message: "barcode is required"},
		},
		{
			name: "validation failed - empty name",
			req: &model.UpsertProductRequest{
				StoreID: uuid.New(),
				Barcode: "123456789",
				Name:    "",
			},
			repo:        &mockProductRepoFull{},
			cache:       &mockCacheFull{},
			search:      &mockSearchIndexServiceFull{},
			expectedErr: &sharedErrors.AppError{Code: sharedErrors.ErrValidationFailed, Message: "name is required"},
		},
		{
			name: "validation failed - negative mrp",
			req: &model.UpsertProductRequest{
				StoreID: uuid.New(),
				Barcode: "123456789",
				Name:    "Test Product",
				Brand:   "Test Brand",
				Category: "Test Category",
				HSNCode:  "123456",
				Unit:     "pcs",
				MRP:     -1,
			},
			repo:        &mockProductRepoFull{},
			cache:       &mockCacheFull{},
			search:      &mockSearchIndexServiceFull{},
			expectedErr: &sharedErrors.AppError{Code: sharedErrors.ErrValidationFailed, Message: "mrp must be greater than 0"},
		},
		{
			name: "validation failed - negative selling price",
			req: &model.UpsertProductRequest{
				StoreID:      uuid.New(),
				Barcode:      "123456789",
				Name:         "Test Product",
				Brand:        "Test Brand",
				Category:     "Test Category",
				HSNCode:      "123456",
				Unit:         "pcs",
				MRP:          100,
				SellingPrice: -1,
			},
			repo:        &mockProductRepoFull{},
			cache:       &mockCacheFull{},
			search:      &mockSearchIndexServiceFull{},
			expectedErr: &sharedErrors.AppError{Code: sharedErrors.ErrValidationFailed, Message: "selling_price must be greater than 0"},
		},
		{
			name: "successful upsert",
			req: &model.UpsertProductRequest{
				StoreID:       uuid.New(),
				Barcode:       "123456789",
				Name:          "Test Product",
				Brand:         "Test Brand",
				Category:      "Test Category",
				HSNCode:       "123456",
				MRP:           100.0,
				SellingPrice:  90.0,
				GSTRate:       18.0,
				Unit:          "pcs",
				StockQuantity: 100,
			},
			repo: &mockProductRepoFull{
				upsert: func(ctx context.Context, p *model.Product) error {
					return nil
				},
			},
			cache: &mockCacheFull{
				invalidate: func(ctx context.Context, storeID, barcode string) error {
					return nil
				},
			},
			search: &mockSearchIndexServiceFull{
				indexProduct: func(ctx context.Context, p model.Product) error {
					return nil
				},
			},
			expectedErr: nil,
		},
		{
			name: "database error",
			req: &model.UpsertProductRequest{
				StoreID:       uuid.New(),
				Barcode:       "123456789",
				Name:          "Test Product",
				Brand:         "Test Brand",
				Category:      "Test Category",
				HSNCode:       "123456",
				MRP:           100.0,
				SellingPrice:  90.0,
				GSTRate:       18.0,
				Unit:          "pcs",
				StockQuantity: 100,
			},
			repo: &mockProductRepoFull{
				upsert: func(ctx context.Context, p *model.Product) error {
					return assert.AnError
				},
			},
			cache:       &mockCacheFull{},
			search:      &mockSearchIndexServiceFull{},
			expectedErr: assert.AnError,
		},
		{
			name: "cache error",
			req: &model.UpsertProductRequest{
				StoreID:       uuid.New(),
				Barcode:       "123456789",
				Name:          "Test Product",
				Brand:         "Test Brand",
				Category:      "Test Category",
				HSNCode:       "123456",
				MRP:           100.0,
				SellingPrice:  90.0,
				GSTRate:       18.0,
				Unit:          "pcs",
				StockQuantity: 100,
			},
			repo: &mockProductRepoFull{
				upsert: func(ctx context.Context, p *model.Product) error {
					return nil
				},
			},
			cache: &mockCacheFull{
				invalidate: func(ctx context.Context, storeID, barcode string) error {
					return assert.AnError
				},
			},
			search: &mockSearchIndexServiceFull{
				indexProduct: func(ctx context.Context, p model.Product) error {
					return nil
				},
			},
			expectedErr: nil,
		},
		{
			name: "search index error",
			req: &model.UpsertProductRequest{
				StoreID:       uuid.New(),
				Barcode:       "123456789",
				Name:          "Test Product",
				Brand:         "Test Brand",
				Category:      "Test Category",
				HSNCode:       "123456",
				MRP:           100.0,
				SellingPrice:  90.0,
				GSTRate:       18.0,
				Unit:          "pcs",
				StockQuantity: 100,
			},
			repo: &mockProductRepoFull{
				upsert: func(ctx context.Context, p *model.Product) error {
					return nil
				},
			},
			cache: &mockCacheFull{
				invalidate: func(ctx context.Context, storeID, barcode string) error {
					return nil
				},
			},
			search: &mockSearchIndexServiceFull{
				indexProduct: func(ctx context.Context, p model.Product) error {
					return assert.AnError
				},
			},
			expectedErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewBarcodeService(tt.repo, tt.cache, tt.search)

			result, err := service.UpsertProduct(context.Background(), tt.req)

			if tt.expectedErr != nil {
				assert.Error(t, err)
				if appErr, ok := tt.expectedErr.(*sharedErrors.AppError); ok {
					if actualAppErr, ok := err.(*sharedErrors.AppError); ok {
						assert.Equal(t, appErr.Code, actualAppErr.Code)
						assert.Equal(t, appErr.Message, actualAppErr.Message)
					}
				}
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tt.req.Barcode, result.Barcode)
				assert.Equal(t, tt.req.Name, result.Name)
			}
		})
	}
}
