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
	"github.com/zippyra/platform/services/cart-service/internal/service"
)

type mockCheckoutService struct {
	InitCheckoutFunc func(ctx context.Context, userID, storeID string) (*model.CheckoutResponse, error)
}

func (m *mockCheckoutService) InitCheckout(ctx context.Context, userID, storeID string) (*model.CheckoutResponse, error) {
	return m.InitCheckoutFunc(ctx, userID, storeID)
}

func TestCheckoutHandler(t *testing.T) {
	svc := &mockCheckoutService{}
	prod := &mockProducer{}
	h := NewCheckoutHandler(svc, prod)

	t.Run("InitCheckout Success", func(t *testing.T) {
		reqBody, _ := json.Marshal(map[string]string{"store_id": "S1"})
		req := httptest.NewRequest("POST", "/checkout/init", bytes.NewBuffer(reqBody))
		w := httptest.NewRecorder()

		svc.InitCheckoutFunc = func(ctx context.Context, userID, storeID string) (*model.CheckoutResponse, error) {
			return &model.CheckoutResponse{CheckoutID: "C1", TotalAmount: 100}, nil
		}
		prod.PublishCheckoutInitiatedFunc = func(ctx context.Context, event kafka.CartEvent) error {
			return nil
		}

		h.InitCheckout(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("InitCheckout Price Changed", func(t *testing.T) {
		reqBody, _ := json.Marshal(map[string]string{"store_id": "S1"})
		req := httptest.NewRequest("POST", "/checkout/init", bytes.NewBuffer(reqBody))
		w := httptest.NewRecorder()

		svc.InitCheckoutFunc = func(ctx context.Context, userID, storeID string) (*model.CheckoutResponse, error) {
			return nil, service.ErrPriceChanged
		}

		h.InitCheckout(w, req)
		assert.Equal(t, http.StatusConflict, w.Code)
	})

	t.Run("InitCheckout Cart Locked", func(t *testing.T) {
		reqBody, _ := json.Marshal(map[string]string{"store_id": "S1"})
		req := httptest.NewRequest("POST", "/checkout/init", bytes.NewBuffer(reqBody))
		w := httptest.NewRecorder()

		svc.InitCheckoutFunc = func(ctx context.Context, userID, storeID string) (*model.CheckoutResponse, error) {
			return nil, service.ErrCartLocked
		}

		h.InitCheckout(w, req)
		assert.Equal(t, http.StatusLocked, w.Code)
	})

	t.Run("InitCheckout Generic Fail", func(t *testing.T) {
		reqBody, _ := json.Marshal(map[string]string{"store_id": "S1"})
		req := httptest.NewRequest("POST", "/checkout/init", bytes.NewBuffer(reqBody))
		w := httptest.NewRecorder()

		svc.InitCheckoutFunc = func(ctx context.Context, userID, storeID string) (*model.CheckoutResponse, error) {
			return nil, errors.New("fail")
		}

		h.InitCheckout(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("InitCheckout Decode Fail", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/checkout/init", bytes.NewBufferString("invalid"))
		w := httptest.NewRecorder()
		h.InitCheckout(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("CashPayment Success", func(t *testing.T) {
		reqBody, _ := json.Marshal(model.CashPaymentRequest{CartID: "C1"})
		req := httptest.NewRequest("POST", "/checkout/cash", bytes.NewBuffer(reqBody))
		w := httptest.NewRecorder()
		h.CashPayment(w, req)
		assert.Equal(t, http.StatusCreated, w.Code)
	})

	t.Run("CashPayment Decode Fail", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/checkout/cash", bytes.NewBufferString("invalid"))
		w := httptest.NewRecorder()
		h.CashPayment(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}
