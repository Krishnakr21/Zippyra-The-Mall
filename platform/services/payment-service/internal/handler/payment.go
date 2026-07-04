package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/zippyra/platform/services/payment-service/internal/model"
	"github.com/zippyra/platform/services/payment-service/internal/service"
	"github.com/zippyra/platform/shared/middleware"
)

type PaymentHandler struct {
	svc *service.PaymentService
}

type InitiateRequest struct {
	OrderID        uuid.UUID `json:"order_id"`
	StoreID        uuid.UUID `json:"store_id"`
	AmountPaise    int64     `json:"amount_paise"`
	PaymentMethod  string    `json:"payment_method"`
	IdempotencyKey string    `json:"idempotency_key"`
}

type RefundRequest struct {
	Amount int64 `json:"amount_paise"`
}

func NewPaymentHandler(svc *service.PaymentService) *PaymentHandler {
	return &PaymentHandler{svc: svc}
}

func (h *PaymentHandler) Initiate(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaimsFromContext(r.Context())
	if claims == nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	var req InitiateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	p := &model.Payment{
		UserID:         uuid.MustParse(claims.UserID),
		OrderID:        req.OrderID,
		StoreID:        req.StoreID,
		AmountPaise:    req.AmountPaise,
		Currency:       "INR",
		Status:         model.PaymentStatusPending,
		PaymentMethod:  (*model.PaymentMethod)(&req.PaymentMethod),
		IdempotencyKey: req.IdempotencyKey,
	}

	initiated, err := h.svc.InitiatePayment(r.Context(), p)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(initiated)
}

func (h *PaymentHandler) Status(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaimsFromContext(r.Context())
	if claims == nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	id := chi.URLParam(r, "payment_id")
	p, err := h.svc.GetStatus(r.Context(), id)
	if err != nil || p == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// AUTH CHECK
	if p.UserID.String() != claims.UserID {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(p)
}

func (h *PaymentHandler) History(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaimsFromContext(r.Context())
	if claims == nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 10
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	payments, err := h.svc.ListHistory(r.Context(), claims.UserID, limit, offset)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(payments)
}

func (h *PaymentHandler) Refund(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaimsFromContext(r.Context())
	if claims == nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	id := chi.URLParam(r, "payment_id")
	var req RefundRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if err := h.svc.InitiateRefund(r.Context(), id, req.Amount); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
