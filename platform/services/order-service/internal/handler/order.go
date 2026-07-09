package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/zippyra/platform/services/order-service/internal/model"
	"github.com/zippyra/platform/services/order-service/internal/service"
	sharederrors "github.com/zippyra/platform/shared/errors"
	"github.com/zippyra/platform/shared/middleware"
)

type OrderHandler struct {
	orderSvc     *service.OrderService
	exitTokenSvc *service.ExitTokenService
}

func NewOrderHandler(orderSvc *service.OrderService, exitTokenSvc *service.ExitTokenService) *OrderHandler {
	return &OrderHandler{
		orderSvc:     orderSvc,
		exitTokenSvc: exitTokenSvc,
	}
}

func (h *OrderHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaimsFromContext(r.Context())
	if claims == nil {
		sharederrors.WriteError(w, http.StatusUnauthorized, sharederrors.ErrUnauthorized, "missing authentication claims", chimiddleware.GetReqID(r.Context()))
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		sharederrors.WriteError(w, http.StatusBadRequest, sharederrors.ErrValidationFailed, "missing order id", chimiddleware.GetReqID(r.Context()))
		return
	}

	order, err := h.orderSvc.GetByID(r.Context(), id)
	if err != nil {
		sharederrors.WriteError(w, http.StatusInternalServerError, sharederrors.ErrInternal, err.Error(), chimiddleware.GetReqID(r.Context()))
		return
	}
	if order == nil {
		sharederrors.WriteError(w, http.StatusNotFound, sharederrors.ErrOrderNotFound, "order not found", chimiddleware.GetReqID(r.Context()))
		return
	}

	// Customer can only view their own order
	if order.UserID.String() != claims.UserID {
		sharederrors.WriteError(w, http.StatusForbidden, sharederrors.ErrForbidden, "access denied to order", chimiddleware.GetReqID(r.Context()))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(order)
}

func (h *OrderHandler) GetHistory(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaimsFromContext(r.Context())
	if claims == nil {
		sharederrors.WriteError(w, http.StatusUnauthorized, sharederrors.ErrUnauthorized, "missing authentication claims", chimiddleware.GetReqID(r.Context()))
		return
	}

	storeID := r.URL.Query().Get("store_id")

	pageStr := r.URL.Query().Get("page")
	limitStr := r.URL.Query().Get("limit")

	page, err := strconv.Atoi(pageStr)
	if err != nil || page <= 0 {
		page = 1
	}

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 20
	}

	orders, err := h.orderSvc.GetHistory(r.Context(), claims.UserID, storeID, page, limit)
	if err != nil {
		sharederrors.WriteError(w, http.StatusInternalServerError, sharederrors.ErrInternal, err.Error(), chimiddleware.GetReqID(r.Context()))
		return
	}

	if orders == nil {
		orders = []model.Order{}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(orders)
}

func (h *OrderHandler) GetExitToken(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaimsFromContext(r.Context())
	if claims == nil {
		sharederrors.WriteError(w, http.StatusUnauthorized, sharederrors.ErrUnauthorized, "missing authentication claims", chimiddleware.GetReqID(r.Context()))
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		sharederrors.WriteError(w, http.StatusBadRequest, sharederrors.ErrValidationFailed, "missing order id", chimiddleware.GetReqID(r.Context()))
		return
	}

	order, err := h.orderSvc.GetByID(r.Context(), id)
	if err != nil {
		sharederrors.WriteError(w, http.StatusInternalServerError, sharederrors.ErrInternal, err.Error(), chimiddleware.GetReqID(r.Context()))
		return
	}
	if order == nil {
		sharederrors.WriteError(w, http.StatusNotFound, sharederrors.ErrOrderNotFound, "order not found", chimiddleware.GetReqID(r.Context()))
		return
	}

	if order.UserID.String() != claims.UserID {
		sharederrors.WriteError(w, http.StatusForbidden, sharederrors.ErrForbidden, "access denied to order", chimiddleware.GetReqID(r.Context()))
		return
	}

	var exitToken string
	if order.ExitToken == nil || *order.ExitToken == "" || order.ExitTokenExpiresAt == nil || order.ExitTokenExpiresAt.Before(time.Now()) {
		exitToken, err = h.exitTokenSvc.Refresh(r.Context(), order)
		if err != nil {
			sharederrors.WriteError(w, http.StatusInternalServerError, sharederrors.ErrInternal, "failed to refresh exit token", chimiddleware.GetReqID(r.Context()))
			return
		}
	} else {
		exitToken = *order.ExitToken
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"exit_token": exitToken,
	})
}
