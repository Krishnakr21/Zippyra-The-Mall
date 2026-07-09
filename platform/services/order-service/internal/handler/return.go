package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/zippyra/platform/services/order-service/internal/service"
	sharederrors "github.com/zippyra/platform/shared/errors"
	"github.com/zippyra/platform/shared/jwt"
	"github.com/zippyra/platform/shared/middleware"
)

type ReturnHandler struct {
	returnSvc *service.ReturnService
}

func NewReturnHandler(returnSvc *service.ReturnService) *ReturnHandler {
	return &ReturnHandler{returnSvc: returnSvc}
}

type CreateReturnRequest struct {
	ItemIDs []string `json:"item_ids"`
	Reason  string   `json:"reason"`
}

type AcceptReturnRequest struct {
	ReturnID      string   `json:"return_id"`
	ItemsVerified []string `json:"items_verified"`
}

func (h *ReturnHandler) Create(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaimsFromContext(r.Context())
	if claims == nil {
		sharederrors.WriteError(w, http.StatusUnauthorized, sharederrors.ErrUnauthorized, "missing authentication claims", chimiddleware.GetReqID(r.Context()))
		return
	}

	orderID := chi.URLParam(r, "id")
	if orderID == "" {
		sharederrors.WriteError(w, http.StatusBadRequest, sharederrors.ErrValidationFailed, "missing order id", chimiddleware.GetReqID(r.Context()))
		return
	}

	var req CreateReturnRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sharederrors.WriteError(w, http.StatusBadRequest, sharederrors.ErrValidationFailed, "invalid request body", chimiddleware.GetReqID(r.Context()))
		return
	}

	if len(req.ItemIDs) == 0 {
		sharederrors.WriteError(w, http.StatusBadRequest, sharederrors.ErrValidationFailed, "item_ids cannot be empty", chimiddleware.GetReqID(r.Context()))
		return
	}
	if req.Reason == "" {
		sharederrors.WriteError(w, http.StatusBadRequest, sharederrors.ErrValidationFailed, "reason cannot be empty", chimiddleware.GetReqID(r.Context()))
		return
	}

	ret, err := h.returnSvc.CreateReturn(r.Context(), claims.UserID, orderID, req.ItemIDs, req.Reason)
	if err != nil {
		if errors.Is(err, service.ErrOrderNotFound) {
			sharederrors.WriteError(w, http.StatusNotFound, sharederrors.ErrOrderNotFound, "order not found", chimiddleware.GetReqID(r.Context()))
			return
		}
		if errors.Is(err, service.ErrForbidden) {
			sharederrors.WriteError(w, http.StatusForbidden, sharederrors.ErrForbidden, "access denied to order", chimiddleware.GetReqID(r.Context()))
			return
		}
		if errors.Is(err, service.ErrReturnWindowClosed) {
			sharederrors.WriteError(w, http.StatusBadRequest, sharederrors.ErrValidationFailed, "return window has closed (24h limit)", chimiddleware.GetReqID(r.Context()))
			return
		}
		if errors.Is(err, service.ErrItemNotReturnable) {
			sharederrors.WriteError(w, http.StatusBadRequest, sharederrors.ErrValidationFailed, "one or more items are not returnable", chimiddleware.GetReqID(r.Context()))
			return
		}
		if errors.Is(err, service.ErrReturnAlreadyExists) {
			sharederrors.WriteError(w, http.StatusConflict, sharederrors.ErrReturnAlreadyExists, "a return is already requested for this order", chimiddleware.GetReqID(r.Context()))
			return
		}
		if errors.Is(err, service.ErrItemNotFound) {
			sharederrors.WriteError(w, http.StatusBadRequest, sharederrors.ErrValidationFailed, "one or more item ids were not found in this order", chimiddleware.GetReqID(r.Context()))
			return
		}
		sharederrors.WriteError(w, http.StatusInternalServerError, sharederrors.ErrInternal, err.Error(), chimiddleware.GetReqID(r.Context()))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"return_id": ret.ID.String(),
		"status":    ret.Status,
	})
}

func (h *ReturnHandler) Accept(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaimsFromContext(r.Context())
	if claims == nil {
		sharederrors.WriteError(w, http.StatusUnauthorized, sharederrors.ErrUnauthorized, "missing authentication claims", chimiddleware.GetReqID(r.Context()))
		return
	}

	// Validate Staff auth
	if claims.UserType != jwt.UserTypeStaff {
		sharederrors.WriteError(w, http.StatusForbidden, sharederrors.ErrForbidden, "staff access required", chimiddleware.GetReqID(r.Context()))
		return
	}

	var req AcceptReturnRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sharederrors.WriteError(w, http.StatusBadRequest, sharederrors.ErrValidationFailed, "invalid request body", chimiddleware.GetReqID(r.Context()))
		return
	}

	if req.ReturnID == "" {
		sharederrors.WriteError(w, http.StatusBadRequest, sharederrors.ErrValidationFailed, "return_id is required", chimiddleware.GetReqID(r.Context()))
		return
	}
	if len(req.ItemsVerified) == 0 {
		sharederrors.WriteError(w, http.StatusBadRequest, sharederrors.ErrValidationFailed, "items_verified cannot be empty", chimiddleware.GetReqID(r.Context()))
		return
	}

	err := h.returnSvc.AcceptReturn(r.Context(), req.ReturnID, req.ItemsVerified)
	if err != nil {
		if errors.Is(err, service.ErrReturnNotFound) {
			sharederrors.WriteError(w, http.StatusNotFound, sharederrors.ErrNotFound, "return request not found", chimiddleware.GetReqID(r.Context()))
			return
		}
		if errors.Is(err, service.ErrOrderNotFound) {
			sharederrors.WriteError(w, http.StatusNotFound, sharederrors.ErrOrderNotFound, "associated order not found", chimiddleware.GetReqID(r.Context()))
			return
		}
		sharederrors.WriteError(w, http.StatusInternalServerError, sharederrors.ErrInternal, err.Error(), chimiddleware.GetReqID(r.Context()))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":           "ACCEPTED",
		"refund_initiated": true,
	})
}
