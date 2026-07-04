package handler

import (
	"encoding/json"
	"net/http"

	"github.com/zippyra/platform/services/cart-service/internal/kafka"
	"github.com/zippyra/platform/services/cart-service/internal/model"
	"github.com/zippyra/platform/services/cart-service/internal/service"
)

type CheckoutHandler struct {
	checkoutService service.CheckoutService
	producer        kafka.Producer
}

func NewCheckoutHandler(checkoutService service.CheckoutService, producer kafka.Producer) *CheckoutHandler {
	return &CheckoutHandler{
		checkoutService: checkoutService,
		producer:        producer,
	}
}

func (h *CheckoutHandler) InitCheckout(w http.ResponseWriter, r *http.Request) {
	var req struct {
		StoreID string `json:"store_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		userID = "test-user"
	}

	res, err := h.checkoutService.InitCheckout(r.Context(), userID, req.StoreID)
	if err != nil {
		if err == service.ErrPriceChanged {
			http.Error(w, err.Error(), http.StatusConflict) // 409
			return
		}
		if err == service.ErrCartLocked {
			http.Error(w, err.Error(), http.StatusLocked) // 423
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Kafka event
	_ = h.producer.PublishCheckoutInitiated(r.Context(), kafka.CartEvent{
		UserID:      userID,
		StoreID:     req.StoreID,
		CheckoutID:  res.CheckoutID,
		TotalAmount: res.TotalAmount,
		ItemCount:   len(res.Items),
	})

	json.NewEncoder(w).Encode(res)
}

func (h *CheckoutHandler) CashPayment(w http.ResponseWriter, r *http.Request) {
	// Staff only - assume middleware checked this
	var req model.CashPaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Logic for cash payment (staff only)
	// Create order (bypass payment service)
	// Clear cart from Redis
	// Return order + exit token
	
	// This is a placeholder for the logic described in the prompt
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(`{"order_id": "uuid", "exit_token": "token", "status": "PAID"}`))
}
