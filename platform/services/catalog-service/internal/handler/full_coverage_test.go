package handler

import (
	"context"
	"errors"
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

type mockBarcodeServiceFull struct {
	lookup func(ctx context.Context, storeID uuid.UUID, barcode string) (*model.ProductResponse, error)
	upsert func(ctx context.Context, req *model.UpsertProductRequest) (*model.Product, error)
}

func (m *mockBarcodeServiceFull) Lookup(ctx context.Context, storeID uuid.UUID, barcode string) (*model.ProductResponse, error) {
	return m.lookup(ctx, storeID, barcode)
}

func (m *mockBarcodeServiceFull) UpsertProduct(ctx context.Context, req *model.UpsertProductRequest) (*model.Product, error) {
	return m.upsert(ctx, req)
}

func TestBarcodeHandler_FullCoverage(t *testing.T) {
	service := &mockBarcodeServiceFull{}
	handler := NewBarcodeHandler(service)

	t.Run("Lookup - Success", func(t *testing.T) {
		service.lookup = func(ctx context.Context, storeID uuid.UUID, barcode string) (*model.ProductResponse, error) {
			return &model.ProductResponse{Product: model.Product{Barcode: barcode}}, nil
		}
		req := httptest.NewRequest("GET", "/v1/catalog/barcode/123?store_id="+uuid.New().String(), nil)
		w := httptest.NewRecorder()
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("barcode", "123")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		handler.Lookup(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("Lookup - Missing StoreID", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/v1/catalog/barcode/123", nil)
		w := httptest.NewRecorder()
		handler.Lookup(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("Lookup - Invalid StoreID", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/v1/catalog/barcode/123?store_id=invalid", nil)
		w := httptest.NewRecorder()
		handler.Lookup(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("Lookup - Service Errors", func(t *testing.T) {
		errs := []error{
			&sharedErrors.AppError{Code: sharedErrors.ErrValidationFailed, Message: "V"},
			&sharedErrors.AppError{Code: sharedErrors.ErrBarcodeNotFound},
			&sharedErrors.AppError{Code: sharedErrors.ErrInternal}, // Default branch
			errors.New("not app error"),
		}
		expectedStatus := []int{http.StatusBadRequest, http.StatusNotFound, http.StatusInternalServerError, http.StatusInternalServerError}

		for i, e := range errs {
			service.lookup = func(ctx context.Context, storeID uuid.UUID, barcode string) (*model.ProductResponse, error) {
				return nil, e
			}
			req := httptest.NewRequest("GET", "/v1/catalog/barcode/123?store_id="+uuid.New().String(), nil)
			w := httptest.NewRecorder()
			handler.Lookup(w, req)
			assert.Equal(t, expectedStatus[i], w.Code)
		}
	})

	t.Run("Upsert - Success", func(t *testing.T) {
		service.upsert = func(ctx context.Context, req *model.UpsertProductRequest) (*model.Product, error) {
			return &model.Product{Barcode: req.Barcode}, nil
		}
		req := httptest.NewRequest("POST", "/v1/catalog/products", strings.NewReader(`{"barcode":"123"}`))
		w := httptest.NewRecorder()
		handler.UpsertProduct(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("Upsert - Invalid Body", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/v1/catalog/products", strings.NewReader(`{invalid`))
		w := httptest.NewRecorder()
		handler.UpsertProduct(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("Upsert - Service Errors", func(t *testing.T) {
		errs := []error{
			&sharedErrors.AppError{Code: sharedErrors.ErrValidationFailed, Message: "V"},
			&sharedErrors.AppError{Code: sharedErrors.ErrInternal}, // Default branch
			errors.New("not app error"),
		}
		expectedStatus := []int{http.StatusBadRequest, http.StatusInternalServerError, http.StatusInternalServerError}

		for i, e := range errs {
			service.upsert = func(ctx context.Context, req *model.UpsertProductRequest) (*model.Product, error) {
				return nil, e
			}
			req := httptest.NewRequest("POST", "/v1/catalog/products", strings.NewReader(`{"barcode":"123"}`))
			w := httptest.NewRecorder()
			handler.UpsertProduct(w, req)
			assert.Equal(t, expectedStatus[i], w.Code)
		}
	})
}

type mockSearchServiceFull struct {
	search       func(ctx context.Context, req *model.ProductSearchRequest) (*model.ProductListResponse, error)
	byCategory   func(ctx context.Context, storeID uuid.UUID, category string, page, limit int) (*model.ProductListResponse, error)
	indexProduct func(ctx context.Context, p model.Product) error
}

func (m *mockSearchServiceFull) Search(ctx context.Context, req *model.ProductSearchRequest) (*model.ProductListResponse, error) {
	return m.search(ctx, req)
}

func (m *mockSearchServiceFull) ByCategory(ctx context.Context, storeID uuid.UUID, category string, page, limit int) (*model.ProductListResponse, error) {
	return m.byCategory(ctx, storeID, category, page, limit)
}

func (m *mockSearchServiceFull) IndexProduct(ctx context.Context, p model.Product) error {
	return m.indexProduct(ctx, p)
}

func TestSearchHandler_FullCoverage(t *testing.T) {
	service := &mockSearchServiceFull{}
	handler := NewSearchHandler(service)

	t.Run("Search - Success", func(t *testing.T) {
		service.search = func(ctx context.Context, req *model.ProductSearchRequest) (*model.ProductListResponse, error) {
			return &model.ProductListResponse{}, nil
		}
		req := httptest.NewRequest("GET", "/v1/catalog/search?store_id="+uuid.New().String()+"&q=test&page=2&limit=50&category=C", nil)
		w := httptest.NewRecorder()
		handler.Search(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("Search - Missing Params", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/v1/catalog/search", nil)
		w := httptest.NewRecorder()
		handler.Search(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)

		req = httptest.NewRequest("GET", "/v1/catalog/search?store_id="+uuid.New().String(), nil)
		w = httptest.NewRecorder()
		handler.Search(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("Search - Invalid StoreID", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/v1/catalog/search?store_id=invalid&q=a", nil)
		w := httptest.NewRecorder()
		handler.Search(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("Search - Service Errors", func(t *testing.T) {
		errs := []error{
			&sharedErrors.AppError{Code: sharedErrors.ErrValidationFailed, Message: "V"},
			&sharedErrors.AppError{Code: sharedErrors.ErrInternal},
			errors.New("not app error"),
		}
		for _, e := range errs {
			service.search = func(ctx context.Context, req *model.ProductSearchRequest) (*model.ProductListResponse, error) {
				return nil, e
			}
			req := httptest.NewRequest("GET", "/v1/catalog/search?store_id="+uuid.New().String()+"&q=a", nil)
			w := httptest.NewRecorder()
			handler.Search(w, req)
			assert.True(t, w.Code == http.StatusBadRequest || w.Code == http.StatusInternalServerError)
		}
	})

	t.Run("ByCategory - Success", func(t *testing.T) {
		service.byCategory = func(ctx context.Context, storeID uuid.UUID, category string, page, limit int) (*model.ProductListResponse, error) {
			return &model.ProductListResponse{}, nil
		}
		req := httptest.NewRequest("GET", "/v1/catalog/category/C?store_id="+uuid.New().String()+"&page=2&limit=50", nil)
		w := httptest.NewRecorder()
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("category", "C")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		handler.ByCategory(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("ByCategory - Invalid Page/Limit Strings", func(t *testing.T) {
		service.byCategory = func(ctx context.Context, storeID uuid.UUID, category string, page, limit int) (*model.ProductListResponse, error) {
			return &model.ProductListResponse{}, nil
		}
		// abc fails Atoi, 999 fails <= 100
		req := httptest.NewRequest("GET", "/v1/catalog/category/C?store_id="+uuid.New().String()+"&page=abc&limit=999", nil)
		w := httptest.NewRecorder()
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("category", "C")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		handler.ByCategory(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		// 0 fails > 0
		req = httptest.NewRequest("GET", "/v1/catalog/category/C?store_id="+uuid.New().String()+"&page=0&limit=0", nil)
		w = httptest.NewRecorder()
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		handler.ByCategory(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("ByCategory - Missing StoreID", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/v1/catalog/category/C", nil)
		w := httptest.NewRecorder()
		handler.ByCategory(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("ByCategory - Missing Category", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/v1/catalog/category/?store_id="+uuid.New().String(), nil)
		w := httptest.NewRecorder()
		// No chi param
		handler.ByCategory(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("ByCategory - Invalid StoreID", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/v1/catalog/category/C?store_id=invalid", nil)
		w := httptest.NewRecorder()
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("category", "C")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		handler.ByCategory(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("ByCategory - Service Errors", func(t *testing.T) {
		errs := []error{
			&sharedErrors.AppError{Code: sharedErrors.ErrValidationFailed, Message: "V"},
			&sharedErrors.AppError{Code: sharedErrors.ErrInternal},
			errors.New("not app error"),
		}
		for _, e := range errs {
			service.byCategory = func(ctx context.Context, storeID uuid.UUID, category string, page, limit int) (*model.ProductListResponse, error) {
				return nil, e
			}
			req := httptest.NewRequest("GET", "/v1/catalog/category/C?store_id="+uuid.New().String(), nil)
			w := httptest.NewRecorder()
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("category", "C")
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
			handler.ByCategory(w, req)
			assert.True(t, w.Code == http.StatusBadRequest || w.Code == http.StatusInternalServerError)
		}
	})
}

type mockSyncServiceFull struct {
	sync func(ctx context.Context, req *model.SyncRequest) (*model.SyncResponse, error)
}

func (m *mockSyncServiceFull) Sync(ctx context.Context, req *model.SyncRequest) (*model.SyncResponse, error) {
	return m.sync(ctx, req)
}

func TestSyncHandler_FullCoverage(t *testing.T) {
	service := &mockSyncServiceFull{}
	handler := NewSyncHandler(service)

	t.Run("Sync - Success", func(t *testing.T) {
		service.sync = func(ctx context.Context, req *model.SyncRequest) (*model.SyncResponse, error) {
			return &model.SyncResponse{}, nil
		}
		req := httptest.NewRequest("POST", "/v1/catalog/sync", strings.NewReader(`{"store_id":"550e8400-e29b-41d4-a716-446655440000"}`))
		w := httptest.NewRecorder()
		handler.Sync(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("Sync - Invalid Body", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/v1/catalog/sync", strings.NewReader(`{invalid`))
		w := httptest.NewRecorder()
		handler.Sync(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("Sync - Service Errors", func(t *testing.T) {
		errs := []error{
			&sharedErrors.AppError{Code: sharedErrors.ErrValidationFailed, Message: "V"},
			&sharedErrors.AppError{Code: sharedErrors.ErrInternal},
			errors.New("not app error"),
		}
		for _, e := range errs {
			service.sync = func(ctx context.Context, req *model.SyncRequest) (*model.SyncResponse, error) {
				return nil, e
			}
			req := httptest.NewRequest("POST", "/v1/catalog/sync", strings.NewReader(`{"store_id":"550e8400-e29b-41d4-a716-446655440000"}`))
			w := httptest.NewRecorder()
			handler.Sync(w, req)
			assert.True(t, w.Code == http.StatusBadRequest || w.Code == http.StatusInternalServerError)
		}
	})
}

type mockOfferServiceFull struct {
	getOffers func(ctx context.Context, storeID uuid.UUID) (*model.OfferResponse, error)
}

func (m *mockOfferServiceFull) GetOffers(ctx context.Context, storeID uuid.UUID) (*model.OfferResponse, error) {
	return m.getOffers(ctx, storeID)
}

func TestOfferHandler_FullCoverage(t *testing.T) {
	service := &mockOfferServiceFull{}
	handler := NewOfferHandler(service)

	t.Run("GetOffers - Success", func(t *testing.T) {
		service.getOffers = func(ctx context.Context, storeID uuid.UUID) (*model.OfferResponse, error) {
			return &model.OfferResponse{}, nil
		}
		req := httptest.NewRequest("GET", "/v1/catalog/offers?store_id="+uuid.New().String(), nil)
		w := httptest.NewRecorder()
		handler.GetOffers(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("GetOffers - Missing StoreID", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/v1/catalog/offers", nil)
		w := httptest.NewRecorder()
		handler.GetOffers(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("GetOffers - Invalid StoreID", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/v1/catalog/offers?store_id=invalid", nil)
		w := httptest.NewRecorder()
		handler.GetOffers(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("GetOffers - Service Errors", func(t *testing.T) {
		errs := []error{
			&sharedErrors.AppError{Code: sharedErrors.ErrValidationFailed, Message: "V"},
			&sharedErrors.AppError{Code: sharedErrors.ErrInternal},
			errors.New("not app error"),
		}
		for _, e := range errs {
			service.getOffers = func(ctx context.Context, storeID uuid.UUID) (*model.OfferResponse, error) {
				return nil, e
			}
			req := httptest.NewRequest("GET", "/v1/catalog/offers?store_id="+uuid.New().String(), nil)
			w := httptest.NewRecorder()
			handler.GetOffers(w, req)
			assert.True(t, w.Code == http.StatusBadRequest || w.Code == http.StatusInternalServerError)
		}
	})
}
