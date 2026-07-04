package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/zippyra/platform/services/cart-service/internal/kafka"
	"github.com/zippyra/platform/services/cart-service/internal/model"
)

// Mocks
type mockCartService struct {
	ScanItemFunc   func(ctx context.Context, userID, storeID, barcode string, quantity int) (*model.CartItem, *model.Cart, error)
	GetCartFunc    func(ctx context.Context, userID, storeID string) (*model.Cart, error)
	RemoveItemFunc func(ctx context.Context, userID, storeID, barcode string) (*model.Cart, error)
	ClearCartFunc  func(ctx context.Context, userID, storeID string) error
}

func (m *mockCartService) ScanItem(ctx context.Context, userID, storeID, barcode string, quantity int) (*model.CartItem, *model.Cart, error) {
	return m.ScanItemFunc(ctx, userID, storeID, barcode, quantity)
}
func (m *mockCartService) GetCart(ctx context.Context, userID, storeID string) (*model.Cart, error) {
	return m.GetCartFunc(ctx, userID, storeID)
}
func (m *mockCartService) RemoveItem(ctx context.Context, userID, storeID, barcode string) (*model.Cart, error) {
	return m.RemoveItemFunc(ctx, userID, storeID, barcode)
}
func (m *mockCartService) ClearCart(ctx context.Context, userID, storeID string) error {
	return m.ClearCartFunc(ctx, userID, storeID)
}

type mockProducer struct {
	PublishItemScannedFunc       func(ctx context.Context, event kafka.CartEvent) error
	PublishCheckoutInitiatedFunc func(ctx context.Context, event kafka.CartEvent) error
}

func (m *mockProducer) PublishItemScanned(ctx context.Context, event kafka.CartEvent) error {
	return m.PublishItemScannedFunc(ctx, event)
}
func (m *mockProducer) PublishCheckoutInitiated(ctx context.Context, event kafka.CartEvent) error {
	return m.PublishCheckoutInitiatedFunc(ctx, event)
}
func (m *mockProducer) Close() error {
	return nil
}

func TestCartHandler(t *testing.T) {
	svc := &mockCartService{}
	prod := &mockProducer{}
	h := NewCartHandler(svc, prod)

	t.Run("ScanItem Success", func(t *testing.T) {
		reqBody, _ := json.Marshal(map[string]interface{}{
			"barcode":  "123",
			"store_id": "S1",
			"quantity": 1,
		})
		req := httptest.NewRequest("POST", "/scan", bytes.NewBuffer(reqBody))
		req.Header.Set("X-User-ID", "U1")
		w := httptest.NewRecorder()

		svc.ScanItemFunc = func(ctx context.Context, userID, storeID, barcode string, quantity int) (*model.CartItem, *model.Cart, error) {
			return &model.CartItem{ProductID: "P1"}, &model.Cart{ItemCount: 1}, nil
		}
		prod.PublishItemScannedFunc = func(ctx context.Context, event kafka.CartEvent) error {
			return nil
		}

		h.ScanItem(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var item model.CartItem
		json.NewDecoder(w.Body).Decode(&item)
		assert.Equal(t, "P1", item.ProductID)
	})

	t.Run("ScanItem Decode Fail", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/scan", bytes.NewBufferString("invalid"))
		w := httptest.NewRecorder()
		h.ScanItem(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("ScanItem Service Fail", func(t *testing.T) {
		reqBody, _ := json.Marshal(map[string]interface{}{"barcode": "1"})
		req := httptest.NewRequest("POST", "/scan", bytes.NewBuffer(reqBody))
		w := httptest.NewRecorder()

		svc.ScanItemFunc = func(ctx context.Context, userID, storeID, barcode string, quantity int) (*model.CartItem, *model.Cart, error) {
			return nil, nil, errors.New("service error")
		}

		h.ScanItem(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("GetCart Success", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/cart?store_id=S1", nil)
		req.Header.Set("X-User-ID", "U1")
		w := httptest.NewRecorder()

		svc.GetCartFunc = func(ctx context.Context, userID, storeID string) (*model.Cart, error) {
			return &model.Cart{UserID: "U1"}, nil
		}

		h.GetCart(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("GetCart Fail", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/cart", nil)
		w := httptest.NewRecorder()
		svc.GetCartFunc = func(ctx context.Context, userID, storeID string) (*model.Cart, error) {
			return nil, errors.New("fail")
		}
		h.GetCart(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("RemoveItem Success", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/cart/item/123?store_id=S1", nil)
		w := httptest.NewRecorder()
		svc.RemoveItemFunc = func(ctx context.Context, userID, storeID, barcode string) (*model.Cart, error) {
			return &model.Cart{}, nil
		}
		h.RemoveItem(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("RemoveItem Fail", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/cart/item/123", nil)
		w := httptest.NewRecorder()
		svc.RemoveItemFunc = func(ctx context.Context, userID, storeID, barcode string) (*model.Cart, error) {
			return nil, errors.New("fail")
		}
		h.RemoveItem(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("ClearCart Success", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/cart/clear?store_id=S1", nil)
		w := httptest.NewRecorder()
		svc.ClearCartFunc = func(ctx context.Context, userID, storeID string) error {
			return nil
		}
		h.ClearCart(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("ClearCart Fail", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/cart/clear", nil)
		w := httptest.NewRecorder()
		svc.ClearCartFunc = func(ctx context.Context, userID, storeID string) error {
			return errors.New("fail")
		}
		h.ClearCart(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}
