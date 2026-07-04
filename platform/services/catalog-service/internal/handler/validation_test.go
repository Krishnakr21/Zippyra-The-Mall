package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/zippyra/platform/services/catalog-service/internal/model"
	sharedErrors "github.com/zippyra/platform/shared/errors"
)

func TestBarcodeHandler_Validation(t *testing.T) {
	mockSvc := &mockBarcodeService{}
	handler := NewBarcodeHandler(mockSvc)

	tests := []struct {
		name           string
		barcode        string
		expectedStatus int
	}{
		{
			name:           "empty barcode",
			barcode:        "",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "valid barcode",
			barcode:        "123456789",
			expectedStatus: http.StatusInternalServerError, // Will panic with nil service
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/v1/catalog/barcode/"+tt.barcode, nil)
			w := httptest.NewRecorder()

			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("barcode", tt.barcode)
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
			if tt.barcode != "" {
				q := req.URL.Query()
				q.Add("store_id", "550e8400-e29b-41d4-a716-446655440000")
				req.URL.RawQuery = q.Encode()
			}

			if tt.barcode == "" {
				handler.Lookup(w, req)
				assert.Equal(t, tt.expectedStatus, w.Code)
			} else {
				mockSvc.lookup = func(ctx context.Context, storeID uuid.UUID, barcode string) (*model.ProductResponse, error) {
					return &model.ProductResponse{
						Product: model.Product{
							ID:    uuid.New(),
							Name:  "Test Product",
							Brand: "Test Brand",
						},
					}, nil
				}
				handler.Lookup(w, req)
				assert.Equal(t, http.StatusOK, w.Code)
			}
		})
	}
}

func TestBarcodeHandler_UpsertValidation(t *testing.T) {
	mockSvc := &mockBarcodeService{}
	handler := NewBarcodeHandler(mockSvc)

	tests := []struct {
		name           string
		requestBody    string
		expectedStatus int
	}{
		{
			name:           "invalid JSON",
			requestBody:    `{"invalid": json}`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "empty request body",
			requestBody:    "",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "valid JSON",
			requestBody:    `{"store_id":"550e8400-e29b-41d4-a716-446655440000","barcode":"123456789","name":"Test Product","brand":"Test Brand","category":"Test Category","mrp":100.0,"selling_price":90.0}`,
			expectedStatus: http.StatusInternalServerError, // Will panic with nil service
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/v1/catalog/products", nil)
			if tt.requestBody != "" {
				req.Body = io.NopCloser(strings.NewReader(tt.requestBody))
			}
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			if tt.name == "invalid JSON" || tt.name == "empty request body" {
				handler.UpsertProduct(w, req)
				assert.Equal(t, tt.expectedStatus, w.Code)
			} else {
				mockSvc.upsert = func(ctx context.Context, req *model.UpsertProductRequest) (*model.Product, error) {
					return &model.Product{
						ID:    uuid.New(),
						Name:  req.Name,
						Brand: req.Brand,
					}, nil
				}
				handler.UpsertProduct(w, req)
				assert.Equal(t, http.StatusOK, w.Code)
			}
		})
	}
}

func TestSearchHandler_Validation(t *testing.T) {
	mockSvc := &mockSearchService{}
	handler := NewSearchHandler(mockSvc)

	tests := []struct {
		name           string
		queryParams    string
		expectedStatus int
	}{
		{
			name:           "missing store_id",
			queryParams:    "?q=test&page=1&limit=20",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "missing query",
			queryParams:    "?store_id=550e8400-e29b-41d4-a716-446655440000&page=1&limit=20",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "missing page",
			queryParams:    "?store_id=550e8400-e29b-41d4-a716-446655440000&q=test&limit=20",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "missing limit",
			queryParams:    "?store_id=550e8400-e29b-41d4-a716-446655440000&q=test&page=1",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "valid request",
			queryParams:    "?store_id=550e8400-e29b-41d4-a716-446655440000&q=test&page=1&limit=20",
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/v1/catalog/search"+tt.queryParams, nil)
			w := httptest.NewRecorder()

			if tt.expectedStatus == http.StatusBadRequest {
				handler.Search(w, req)
				assert.Equal(t, tt.expectedStatus, w.Code)
			} else {
				mockSvc.search = func(ctx context.Context, req *model.ProductSearchRequest) (*model.ProductListResponse, error) {
					return &model.ProductListResponse{}, nil
				}
				handler.Search(w, req)
				assert.Equal(t, tt.expectedStatus, w.Code)
			}
		})
	}
}

func TestSearchHandler_ByCategoryValidation(t *testing.T) {
	mockSvc := &mockSearchService{}
	handler := NewSearchHandler(mockSvc)

	tests := []struct {
		name           string
		category       string
		queryParams    string
		expectedStatus int
	}{
		{
			name:           "missing store_id",
			category:       "electronics",
			queryParams:    "?page=1&limit=20",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "missing page",
			category:       "electronics",
			queryParams:    "?store_id=550e8400-e29b-41d4-a716-446655440000&limit=20",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "missing limit",
			category:       "electronics",
			queryParams:    "?store_id=550e8400-e29b-41d4-a716-446655440000&page=1",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "invalid page string",
			category:       "electronics",
			queryParams:    "?store_id=550e8400-e29b-41d4-a716-446655440000&page=abc",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "non-positive page",
			category:       "electronics",
			queryParams:    "?store_id=550e8400-e29b-41d4-a716-446655440000&page=0",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "invalid limit string",
			category:       "electronics",
			queryParams:    "?store_id=550e8400-e29b-41d4-a716-446655440000&limit=abc",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "non-positive limit",
			category:       "electronics",
			queryParams:    "?store_id=550e8400-e29b-41d4-a716-446655440000&limit=0",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "limit too high",
			category:       "electronics",
			queryParams:    "?store_id=550e8400-e29b-41d4-a716-446655440000&limit=101",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "valid request",
			category:       "electronics",
			queryParams:    "?store_id=550e8400-e29b-41d4-a716-446655440000&page=1&limit=20",
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/v1/catalog/category/"+tt.category+tt.queryParams, nil)
			w := httptest.NewRecorder()

			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("category", tt.category)
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			if tt.expectedStatus == http.StatusBadRequest {
				handler.ByCategory(w, req)
				assert.Equal(t, tt.expectedStatus, w.Code)
			} else {
				mockSvc.byCategory = func(ctx context.Context, storeID uuid.UUID, category string, page, limit int) (*model.ProductListResponse, error) {
					return &model.ProductListResponse{}, nil
				}
				handler.ByCategory(w, req)
				assert.Equal(t, tt.expectedStatus, w.Code)
			}
		})
	}
}

func TestSyncHandler_Validation(t *testing.T) {
	mockSvc := &mockSyncService{}
	handler := NewSyncHandler(mockSvc)

	tests := []struct {
		name           string
		requestBody    string
		expectedStatus int
	}{
		{
			name:           "invalid JSON",
			requestBody:    `{"invalid": json}`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "empty request body",
			requestBody:    "",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "valid JSON",
			requestBody:    `{"store_id":"550e8400-e29b-41d4-a716-446655440000","last_sync_seq":0,"limit":100}`,
			expectedStatus: http.StatusInternalServerError, // Will panic with nil service
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/v1/catalog/sync", nil)
			if tt.requestBody != "" {
				req.Body = io.NopCloser(strings.NewReader(tt.requestBody))
			}
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			if tt.name == "invalid JSON" || tt.name == "empty request body" {
				handler.Sync(w, req)
				assert.Equal(t, tt.expectedStatus, w.Code)
			} else {
				mockSvc.sync = func(ctx context.Context, req *model.SyncRequest) (*model.SyncResponse, error) {
					return &model.SyncResponse{}, nil
				}
				handler.Sync(w, req)
				assert.Equal(t, http.StatusOK, w.Code)
			}
		})
	}
}

func TestOfferHandler_Validation(t *testing.T) {
	mockSvc := &mockOfferService{}
	handler := NewOfferHandler(mockSvc)

	tests := []struct {
		name           string
		queryParams    string
		expectedStatus int
	}{
		{
			name:           "missing store_id",
			queryParams:    "",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "invalid store_id",
			queryParams:    "?store_id=invalid-uuid",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "valid request",
			queryParams:    "?store_id=550e8400-e29b-41d4-a716-446655440000",
			expectedStatus: http.StatusInternalServerError, // Will panic with nil service
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/v1/catalog/offers"+tt.queryParams, nil)
			w := httptest.NewRecorder()

			if tt.name != "valid request" {
				handler.GetOffers(w, req)
				assert.Equal(t, tt.expectedStatus, w.Code)
			} else {
				mockSvc.getOffers = func(ctx context.Context, storeID uuid.UUID) (*model.OfferResponse, error) {
					return &model.OfferResponse{}, nil
				}
				handler.GetOffers(w, req)
				assert.Equal(t, http.StatusOK, w.Code)
			}
		})
	}
}

func TestErrorResponse(t *testing.T) {
	w := httptest.NewRecorder()

	err := &sharedErrors.AppError{
		Code:    sharedErrors.ErrValidationFailed,
		Message: "test error",
	}

	// Test error response writing
	respondWithError(w, http.StatusBadRequest, err.Message)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "test error")
	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "test error", resp["message"])
}
