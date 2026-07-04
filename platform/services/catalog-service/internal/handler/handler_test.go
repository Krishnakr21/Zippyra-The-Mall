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

func TestBarcodeHandler_Lookup(t *testing.T) {
	service := &mockBarcodeService{
		lookup: func(ctx context.Context, storeID uuid.UUID, barcode string) (*model.ProductResponse, error) {
			if barcode == "123456789" {
				return &model.ProductResponse{
					Product: model.Product{
						ID:       uuid.New(),
						StoreID:  storeID,
						Barcode:  barcode,
						Name:     "Test Product",
						IsActive: true,
					},
					CacheHit: true,
				}, nil
			}
			return nil, &sharedErrors.AppError{Code: sharedErrors.ErrNotFound, Message: "product not found"}
		},
	}

	handler := NewBarcodeHandler(service)

	tests := []struct {
		name           string
		barcode        string
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "successful lookup",
			barcode:        "123456789",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "product not found",
			barcode:        "999999999",
			expectedStatus: http.StatusNotFound,
			expectedError:  "product not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create request
			req := httptest.NewRequest("GET", "/v1/catalog/barcode/"+tt.barcode+"?store_id=550e8400-e29b-41d4-a716-446655440000", nil)
			w := httptest.NewRecorder()

			// Set up chi router context
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("barcode", tt.barcode)
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			// Call handler
			handler.Lookup(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusOK {
				var response model.ProductResponse
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.Equal(t, tt.barcode, response.Product.Barcode)
				assert.Equal(t, "Test Product", response.Product.Name)
			} else if tt.expectedError != "" {
				var errorResponse map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &errorResponse)
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedError, errorResponse["message"])
			}
		})
	}
}

func TestBarcodeHandler_UpsertProduct(t *testing.T) {
	service := &mockBarcodeService{
		upsert: func(ctx context.Context, req *model.UpsertProductRequest) (*model.Product, error) {
			if req.Barcode == "123456789" {
				return &model.Product{
					ID:       uuid.New(),
					StoreID:  req.StoreID,
					Barcode:  req.Barcode,
					Name:     req.Name,
					IsActive: true,
				}, nil
			}
			return nil, &sharedErrors.AppError{Code: sharedErrors.ErrValidationFailed, Message: "invalid request"}
		},
	}

	handler := NewBarcodeHandler(service)

	tests := []struct {
		name           string
		requestBody    string
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "successful upsert",
			requestBody:    `{"store_id":"550e8400-e29b-41d4-a716-446655440000","barcode":"123456789","name":"Test Product","brand":"Test Brand","category":"Test Category","mrp":100.0,"selling_price":90.0}`,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "invalid JSON",
			requestBody:    `{"invalid": json}`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "validation error",
			requestBody:    `{"store_id":"550e8400-e29b-41d4-a716-446655440000","barcode":"999999999","name":"Test Product"}`,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "invalid request",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create request
			req := httptest.NewRequest("POST", "/v1/catalog/products", nil)
			req.Body = io.NopCloser(strings.NewReader(tt.requestBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			// Call handler
			handler.UpsertProduct(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusOK {
				var response model.Product
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.Equal(t, "123456789", response.Barcode)
				assert.Equal(t, "Test Product", response.Name)
			} else if tt.expectedError != "" {
				var errorResponse map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &errorResponse)
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedError, errorResponse["message"])
			}
		})
	}
}
