package handler

import (
	"encoding/json"
	"net/http"

	"github.com/zippyra/platform/services/cart-service/internal/service"
)

type CouponHandler struct {
	couponService service.CouponService
}

func NewCouponHandler(couponService service.CouponService) *CouponHandler {
	return &CouponHandler{couponService: couponService}
}

func (h *CouponHandler) ApplyCoupon(w http.ResponseWriter, r *http.Request) {
	var req struct {
		StoreID    string `json:"store_id"`
		CouponCode string `json:"coupon_code"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		userID = "test-user"
	}

	cart, err := h.couponService.ApplyCoupon(r.Context(), userID, req.StoreID, req.CouponCode)
	if err != nil {
		if err == service.ErrCouponInvalid || err == service.ErrCouponLimit {
			http.Error(w, err.Error(), http.StatusUnprocessableEntity)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(cart)
}
