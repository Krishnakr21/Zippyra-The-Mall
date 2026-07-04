package service

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/zippyra/platform/services/catalog-service/internal/model"
	sharedErrors "github.com/zippyra/platform/shared/errors"
)

func TestBarcodeService_Validation(t *testing.T) {
	service := &barcodeService{
		productRepo: nil,
		cache:       nil,
		search:      nil,
	}

	tests := []struct {
		name        string
		barcode     string
		expectedErr error
	}{
		{
			name:        "empty barcode",
			barcode:     "",
			expectedErr: &sharedErrors.AppError{Code: sharedErrors.ErrValidationFailed, Message: "barcode cannot be empty"},
		},
		{
			name:        "valid barcode",
			barcode:     "123456789",
			expectedErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storeID := uuid.New()

			if tt.barcode == "" {
				_, err := service.Lookup(context.Background(), storeID, tt.barcode)
				if tt.expectedErr != nil {
					assert.Error(t, err)
					if appErr, ok := err.(*sharedErrors.AppError); ok {
						assert.Equal(t, sharedErrors.ErrValidationFailed, appErr.Code)
						assert.Equal(t, "barcode cannot be empty", appErr.Message)
					}
				}
			} else {
				// Valid barcode will panic with nil dependencies
				assert.Panics(t, func() {
					service.Lookup(context.Background(), storeID, tt.barcode)
				})
			}
		})
	}
}

func TestBarcodeService_UpsertValidation(t *testing.T) {
	service := &barcodeService{
		productRepo: nil,
		cache:       nil,
		search:      nil,
	}

	tests := []struct {
		name        string
		req         *model.UpsertProductRequest
		expectedErr error
	}{
		{
			name: "validation failed - empty store_id",
			req: &model.UpsertProductRequest{
				StoreID: uuid.Nil,
				Barcode: "123456789",
				Name:    "Test Product",
			},
			expectedErr: &sharedErrors.AppError{Code: sharedErrors.ErrValidationFailed, Message: "store_id is required"},
		},
		{
			name: "validation failed - empty barcode",
			req: &model.UpsertProductRequest{
				StoreID: uuid.New(),
				Barcode: "",
				Name:    "Test Product",
			},
			expectedErr: &sharedErrors.AppError{Code: sharedErrors.ErrValidationFailed, Message: "barcode is required"},
		},
		{
			name: "validation failed - empty name",
			req: &model.UpsertProductRequest{
				StoreID: uuid.New(),
				Barcode: "123456789",
				Name:    "",
			},
			expectedErr: &sharedErrors.AppError{Code: sharedErrors.ErrValidationFailed, Message: "name is required"},
		},
		{
			name: "validation failed - negative mrp",
			req: &model.UpsertProductRequest{
				StoreID:  uuid.New(),
				Barcode:  "123456789",
				Name:     "Test Product",
				Brand:    "Test Brand",
				Category: "Test Category",
				HSNCode:  "123456",
				Unit:     "pcs",
				MRP:      -1,
			},
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
			expectedErr: &sharedErrors.AppError{Code: sharedErrors.ErrValidationFailed, Message: "selling_price must be greater than 0"},
		},
		{
			name: "validation failed - negative gst rate",
			req: &model.UpsertProductRequest{
				StoreID:  uuid.New(),
				Barcode:  "123456789",
				Name:     "Test Product",
				Brand:    "Test Brand",
				Category: "Test Category",
				HSNCode:  "123456",
				Unit:         "pcs",
				MRP:          100,
				SellingPrice: 90.0,
				GSTRate:      -1,
			},
			expectedErr: &sharedErrors.AppError{Code: sharedErrors.ErrValidationFailed, Message: "gst_rate must be between 0 and 100"},
		},
		{
			name: "validation failed - negative stock quantity",
			req: &model.UpsertProductRequest{
				StoreID:       uuid.New(),
				Barcode:       "123456789",
				Name:          "Test Product",
				Brand:         "Test Brand",
				Category:      "Test Category",
				HSNCode:       "123456",
				Unit:          "pcs",
				MRP:           100,
				SellingPrice:  90.0,
				StockQuantity: -1,
			},
			expectedErr: &sharedErrors.AppError{Code: sharedErrors.ErrValidationFailed, Message: "stock_quantity must be greater than or equal to 0"},
		},
		{
			name: "validation failed - negative reorder point",
			req: &model.UpsertProductRequest{
				StoreID:      uuid.New(),
				Barcode:      "123456789",
				Name:         "Test Product",
				Brand:        "Test Brand",
				Category:     "Test Category",
				HSNCode:      "123456",
				Unit:         "pcs",
				MRP:          100,
				SellingPrice: 90.0,
				ReorderPoint: func() *int { v := -1; return &v }(),
			},
			expectedErr: &sharedErrors.AppError{Code: sharedErrors.ErrValidationFailed, Message: "reorder_point must be greater than or equal to 0"},
		},
		{
			name: "validation failed - negative reorder quantity",
			req: &model.UpsertProductRequest{
				StoreID:      uuid.New(),
				Barcode:      "123456789",
				Name:         "Test Product",
				Brand:        "Test Brand",
				Category:     "Test Category",
				HSNCode:      "123456",
				Unit:         "pcs",
				MRP:          100,
				SellingPrice: 90.0,
				ReorderQuantity: func() *int { v := -1; return &v }(),
			},
			expectedErr: &sharedErrors.AppError{Code: sharedErrors.ErrValidationFailed, Message: "reorder_quantity must be greater than or equal to 0"},
		},
		{
			name: "valid request",
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
			expectedErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.expectedErr != nil {
				result, err := service.UpsertProduct(context.Background(), tt.req)
				assert.Error(t, err)
				if appErr, ok := err.(*sharedErrors.AppError); ok {
					assert.Equal(t, tt.expectedErr.(*sharedErrors.AppError).Code, appErr.Code)
					assert.Equal(t, tt.expectedErr.(*sharedErrors.AppError).Message, appErr.Message)
				}
				assert.Nil(t, result)
			} else {
				// Provide mocks to avoid panics during success testing
				service.productRepo = &mockProductRepo{
					upsert: func(ctx context.Context, p *model.Product) error { return nil },
				}
				service.cache = &mockCache{
					invalidate: func(ctx context.Context, storeID, barcode string) error { return nil },
				}
				service.search = &mockSearchIndexService{
					indexProduct: func(ctx context.Context, p model.Product) error { return nil },
				}
				result, err := service.UpsertProduct(context.Background(), tt.req)
				assert.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}

func TestSearchService_Validation(t *testing.T) {
	service := &searchService{
		productRepo: nil,
	}

	tests := []struct {
		name        string
		req         *model.ProductSearchRequest
		expectedErr error
	}{
		{
			name: "validation failed - empty store_id",
			req: &model.ProductSearchRequest{
				StoreID: uuid.Nil,
				Query:   "test",
				Page:    1,
				Limit:   20,
			},
			expectedErr: &sharedErrors.AppError{Code: sharedErrors.ErrValidationFailed, Message: "store_id is required"},
		},
		{
			name: "validation failed - empty query",
			req: &model.ProductSearchRequest{
				StoreID: uuid.New(),
				Query:   "",
				Page:    1,
				Limit:   20,
			},
			expectedErr: &sharedErrors.AppError{Code: sharedErrors.ErrValidationFailed, Message: "query is required"},
		},
		{
			name: "validation failed - invalid page",
			req: &model.ProductSearchRequest{
				StoreID: uuid.New(),
				Query:   "test",
				Page:    0,
				Limit:   20,
			},
			expectedErr: &sharedErrors.AppError{Code: sharedErrors.ErrValidationFailed, Message: "page must be greater than 0"},
		},
		{
			name: "validation failed - invalid limit",
			req: &model.ProductSearchRequest{
				StoreID: uuid.New(),
				Query:   "test",
				Page:    1,
				Limit:   0,
			},
			expectedErr: &sharedErrors.AppError{Code: sharedErrors.ErrValidationFailed, Message: "limit must be greater than 0"},
		},
		{
			name: "validation failed - limit too high",
			req: &model.ProductSearchRequest{
				StoreID: uuid.New(),
				Query:   "test",
				Page:    1,
				Limit:   101,
			},
			expectedErr: &sharedErrors.AppError{Code: sharedErrors.ErrValidationFailed, Message: "limit must be less than or equal to 100"},
		},
		{
			name: "valid request",
			req: &model.ProductSearchRequest{
				StoreID: uuid.New(),
				Query:   "test",
				Page:    1,
				Limit:   20,
			},
			expectedErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.expectedErr != nil {
				result, err := service.Search(context.Background(), tt.req)
				assert.Error(t, err)
				if appErr, ok := err.(*sharedErrors.AppError); ok {
					assert.Equal(t, tt.expectedErr.(*sharedErrors.AppError).Code, appErr.Code)
					assert.Equal(t, tt.expectedErr.(*sharedErrors.AppError).Message, appErr.Message)
				}
				assert.Nil(t, result)
			} else {
				// Provide mocks to avoid panics
				service.search = &mockSearchClient{
					searchFunc: func(ctx context.Context, storeID, query, category string, page, limit int) ([]model.Product, int, error) {
						return []model.Product{}, 0, nil
					},
				}
				result, err := service.Search(context.Background(), tt.req)
				assert.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}

func TestSearchService_ByCategoryValidation(t *testing.T) {
	service := &searchService{
		productRepo: nil,
	}

	tests := []struct {
		name        string
		storeID     uuid.UUID
		category    string
		page        int
		limit       int
		expectedErr error
	}{
		{
			name:        "empty store_id",
			storeID:     uuid.Nil,
			category:    "electronics",
			page:        1,
			limit:       20,
			expectedErr: &sharedErrors.AppError{Code: sharedErrors.ErrValidationFailed, Message: "store_id is required"},
		},
		{
			name:        "empty category",
			storeID:     uuid.New(),
			category:    "",
			page:        1,
			limit:       20,
			expectedErr: &sharedErrors.AppError{Code: sharedErrors.ErrValidationFailed, Message: "category is required"},
		},
		{
			name:        "invalid page",
			storeID:     uuid.New(),
			category:    "electronics",
			page:        0,
			limit:       20,
			expectedErr: &sharedErrors.AppError{Code: sharedErrors.ErrValidationFailed, Message: "page must be greater than 0"},
		},
		{
			name:        "invalid limit",
			storeID:     uuid.New(),
			category:    "electronics",
			page:        1,
			limit:       0,
			expectedErr: &sharedErrors.AppError{Code: sharedErrors.ErrValidationFailed, Message: "limit must be greater than 0"},
		},
		{
			name:        "valid request",
			storeID:     uuid.New(),
			category:    "electronics",
			page:        1,
			limit:       20,
			expectedErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.expectedErr != nil {
				result, err := service.ByCategory(context.Background(), tt.storeID, tt.category, tt.page, tt.limit)
				assert.Error(t, err)
				if appErr, ok := err.(*sharedErrors.AppError); ok {
					assert.Equal(t, tt.expectedErr.(*sharedErrors.AppError).Code, appErr.Code)
					assert.Equal(t, tt.expectedErr.(*sharedErrors.AppError).Message, appErr.Message)
				}
				assert.Nil(t, result)
			} else {
				// Provide mocks to avoid panics
				service.productRepo = &mockProductRepo{
					getByCategory: func(ctx context.Context, storeID uuid.UUID, category string, page, limit int) ([]model.Product, int, error) {
						return []model.Product{}, 0, nil
					},
				}
				result, err := service.ByCategory(context.Background(), tt.storeID, tt.category, tt.page, tt.limit)
				assert.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}

func TestSyncService_Validation(t *testing.T) {
	service := &syncService{
		productRepo: nil,
	}

	tests := []struct {
		name        string
		req         *model.SyncRequest
		expectedErr error
	}{
		{
			name: "validation failed - empty store_id",
			req: &model.SyncRequest{
				StoreID:     uuid.Nil,
				LastSyncSeq: 0,
				Limit:       100,
			},
			expectedErr: &sharedErrors.AppError{Code: sharedErrors.ErrValidationFailed, Message: "store_id is required"},
		},
		{
			name: "validation failed - negative last_sync_seq",
			req: &model.SyncRequest{
				StoreID:     uuid.New(),
				LastSyncSeq: -1,
				Limit:       100,
			},
			expectedErr: &sharedErrors.AppError{Code: sharedErrors.ErrValidationFailed, Message: "last_sync_seq must be greater than or equal to 0"},
		},
		{
			name: "validation failed - invalid limit",
			req: &model.SyncRequest{
				StoreID:     uuid.New(),
				LastSyncSeq: 0,
				Limit:       0,
			},
			expectedErr: &sharedErrors.AppError{Code: sharedErrors.ErrValidationFailed, Message: "limit must be greater than 0"},
		},
		{
			name: "validation failed - limit too high",
			req: &model.SyncRequest{
				StoreID:     uuid.New(),
				LastSyncSeq: 0,
				Limit:       1001,
			},
			expectedErr: &sharedErrors.AppError{Code: sharedErrors.ErrValidationFailed, Message: "limit must be less than or equal to 1000"},
		},
		{
			name: "valid request",
			req: &model.SyncRequest{
				StoreID:     uuid.New(),
				LastSyncSeq: 0,
				Limit:       100,
			},
			expectedErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.expectedErr != nil {
				result, err := service.Sync(context.Background(), tt.req)
				assert.Error(t, err)
				if appErr, ok := err.(*sharedErrors.AppError); ok {
					assert.Equal(t, tt.expectedErr.(*sharedErrors.AppError).Code, appErr.Code)
					assert.Equal(t, tt.expectedErr.(*sharedErrors.AppError).Message, appErr.Message)
				}
				assert.Nil(t, result)
			} else {
				// Provide mocks to avoid panics
				service.productRepo = &mockProductRepo{
					sync: func(ctx context.Context, storeID uuid.UUID, lastSyncSeq int64, limit int) ([]model.Product, error) {
						return []model.Product{}, nil
					},
				}
				result, err := service.Sync(context.Background(), tt.req)
				assert.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}

func TestOfferService_Validation(t *testing.T) {
	service := &offerService{
		offerRepo: nil,
		cache:     nil,
	}

	tests := []struct {
		name        string
		storeID     uuid.UUID
		expectedErr error
	}{
		{
			name:        "empty store_id",
			storeID:     uuid.Nil,
			expectedErr: &sharedErrors.AppError{Code: sharedErrors.ErrValidationFailed, Message: "store_id is required"},
		},
		{
			name:        "valid request",
			storeID:     uuid.New(),
			expectedErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.expectedErr != nil {
				result, err := service.GetOffers(context.Background(), tt.storeID)
				assert.Error(t, err)
				if appErr, ok := err.(*sharedErrors.AppError); ok {
					assert.Equal(t, tt.expectedErr.(*sharedErrors.AppError).Code, appErr.Code)
					assert.Equal(t, tt.expectedErr.(*sharedErrors.AppError).Message, appErr.Message)
				}
				assert.Nil(t, result)
			} else {
				// Provide mocks to avoid panics
				service.offerRepo = &mockOfferRepo{
					getActiveByStore: func(ctx context.Context, storeID uuid.UUID) ([]model.OfferRule, error) {
						return []model.OfferRule{}, nil
					},
				}
				service.cache = &mockOfferCache{
					getOffers: func(ctx context.Context, key string) (*model.OfferResponse, error) {
						return nil, nil
					},
					setOffers: func(ctx context.Context, key string, response *model.OfferResponse) error {
						return nil
					},
				}
				_, err := service.GetOffers(context.Background(), tt.storeID)
				assert.NoError(t, err)
			}
		})
	}
}
