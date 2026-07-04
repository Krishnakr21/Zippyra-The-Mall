package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/zippyra/platform/services/cart-service/internal/kafka"
	"github.com/zippyra/platform/services/cart-service/internal/service"
)

type CartHandler struct {
	cartService service.CartService
	producer    kafka.Producer
}

func NewCartHandler(cartService service.CartService, producer kafka.Producer) *CartHandler {
	return &CartHandler{
		cartService: cartService,
		producer:    producer,
	}
}

func (h *CartHandler) ScanItem(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Barcode string `json:"barcode"`
		StoreID string `json:"store_id"`
		Quantity int   `json:"quantity"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// In a real app, userID comes from JWT middleware
	userID := r.Header.Get("X-User-ID") 
	if userID == "" {
		userID = "test-user" // Fallback for tests or if not set
	}

	item, cart, err := h.cartService.ScanItem(r.Context(), userID, req.StoreID, req.Barcode, req.Quantity)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Kafka event
	_ = h.producer.PublishItemScanned(r.Context(), kafka.CartEvent{
		UserID:      userID,
		StoreID:     req.StoreID,
		Barcode:     req.Barcode,
		ProductID:   item.ProductID,
		ProductName: item.ProductName,
		Quantity:    item.Quantity,
		UnitPrice:   item.UnitPrice,
		GSTAmount:   item.GSTAmount,
		TotalAmount: item.TotalPrice,
		ItemCount:   cart.ItemCount,
	})

	json.NewEncoder(w).Encode(item)
}

func (h *CartHandler) GetCart(w http.ResponseWriter, r *http.Request) {
	storeID := r.URL.Query().Get("store_id")
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		userID = "test-user"
	}

	cart, err := h.cartService.GetCart(r.Context(), userID, storeID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(cart)
}

func (h *CartHandler) RemoveItem(w http.ResponseWriter, r *http.Request) {
	barcode := chi.URLParam(r, "barcode")
	storeID := r.URL.Query().Get("store_id")
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		userID = "test-user"
	}

	cart, err := h.cartService.RemoveItem(r.Context(), userID, storeID, barcode)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(cart)
}

func (h *CartHandler) ClearCart(w http.ResponseWriter, r *http.Request) {
	storeID := r.URL.Query().Get("store_id")
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		userID = "test-user"
	}

	err := h.cartService.ClearCart(r.Context(), userID, storeID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Write([]byte(`{"message": "Cart cleared"}`))
}
