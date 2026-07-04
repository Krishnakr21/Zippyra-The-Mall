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
	"github.com/zippyra/platform/services/cart-service/internal/model"
	"github.com/zippyra/platform/services/cart-service/internal/service"
)

type mockCouponService struct {
	ApplyCouponFunc func(ctx context.Context, userID, storeID, couponCode string) (*model.Cart, error)
}

func (m *mockCouponService) ApplyCoupon(ctx context.Context, userID, storeID, couponCode string) (*model.Cart, error) {
	return m.ApplyCouponFunc(ctx, userID, storeID, couponCode)
}

func TestCouponHandler(t *testing.T) {
	svc := &mockCouponService{}
	h := NewCouponHandler(svc)

	t.Run("ApplyCoupon Success", func(t *testing.T) {
		reqBody, _ := json.Marshal(map[string]string{"store_id": "S1", "coupon_code": "SAVE10"})
		req := httptest.NewRequest("POST", "/coupon/apply", bytes.NewBuffer(reqBody))
		w := httptest.NewRecorder()

		svc.ApplyCouponFunc = func(ctx context.Context, userID, storeID, couponCode string) (*model.Cart, error) {
			return &model.Cart{UserID: userID}, nil
		}

		h.ApplyCoupon(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("ApplyCoupon Invalid/Limit", func(t *testing.T) {
		reqBody, _ := json.Marshal(map[string]string{"store_id": "S1", "coupon_code": "INVALID"})
		req := httptest.NewRequest("POST", "/coupon/apply", bytes.NewBuffer(reqBody))
		w := httptest.NewRecorder()

		svc.ApplyCouponFunc = func(ctx context.Context, userID, storeID, couponCode string) (*model.Cart, error) {
			return nil, service.ErrCouponInvalid
		}

		h.ApplyCoupon(w, req)
		assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	})

	t.Run("ApplyCoupon Fail", func(t *testing.T) {
		reqBody, _ := json.Marshal(map[string]string{"store_id": "S1", "coupon_code": "FAIL"})
		req := httptest.NewRequest("POST", "/coupon/apply", bytes.NewBuffer(reqBody))
		w := httptest.NewRecorder()

		svc.ApplyCouponFunc = func(ctx context.Context, userID, storeID, couponCode string) (*model.Cart, error) {
			return nil, errors.New("fail")
		}

		h.ApplyCoupon(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("ApplyCoupon Decode Fail", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/coupon/apply", bytes.NewBufferString("invalid"))
		w := httptest.NewRecorder()
		h.ApplyCoupon(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}
