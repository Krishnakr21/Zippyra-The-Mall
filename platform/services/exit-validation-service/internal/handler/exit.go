package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/zippyra/platform/services/exit-validation-service/internal/model"
	"github.com/zippyra/platform/services/exit-validation-service/internal/service"
	sharederrors "github.com/zippyra/platform/shared/errors"
	"github.com/zippyra/platform/shared/middleware"
)

type ExitHandler struct {
	exitSvc *service.ExitService
}

func NewExitHandler(exitSvc *service.ExitService) *ExitHandler {
	return &ExitHandler{
		exitSvc: exitSvc,
	}
}

func (h *ExitHandler) Validate(w http.ResponseWriter, r *http.Request) {
	reqID := chimiddleware.GetReqID(r.Context())
	claims := middleware.GetClaimsFromContext(r.Context())
	if claims == nil {
		sharederrors.WriteError(w, http.StatusUnauthorized, sharederrors.ErrUnauthorized, "missing authentication claims", reqID)
		return
	}

	var req model.ExitValidationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sharederrors.WriteError(w, http.StatusBadRequest, sharederrors.ErrValidationFailed, "invalid request body", reqID)
		return
	}

	if req.ExitToken == "" {
		sharederrors.WriteValidationError(w, "exit_token", "exit_token is required", reqID)
		return
	}
	if _, err := uuid.Parse(req.StoreID); err != nil {
		sharederrors.WriteValidationError(w, "store_id", "invalid store_id UUID", reqID)
		return
	}
	if req.GateID == "" {
		sharederrors.WriteValidationError(w, "gate_id", "gate_id is required", reqID)
		return
	}

	res, err := h.exitSvc.Validate(r.Context(), req.ExitToken, req.StoreID, req.GateID, claims.UserID)
	if err != nil {
		if appErr, ok := err.(*sharederrors.AppError); ok {
			sharederrors.WriteError(w, appErr.HTTPStatus, appErr.Code, appErr.Message, reqID)
			return
		}
		sharederrors.WriteError(w, http.StatusInternalServerError, sharederrors.ErrInternal, err.Error(), reqID)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(res)
}

func (h *ExitHandler) StaffOverride(w http.ResponseWriter, r *http.Request) {
	reqID := chimiddleware.GetReqID(r.Context())
	claims := middleware.GetClaimsFromContext(r.Context())
	if claims == nil {
		sharederrors.WriteError(w, http.StatusUnauthorized, sharederrors.ErrUnauthorized, "missing authentication claims", reqID)
		return
	}

	var req model.StaffOverrideRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sharederrors.WriteError(w, http.StatusBadRequest, sharederrors.ErrValidationFailed, "invalid request body", reqID)
		return
	}

	if _, err := uuid.Parse(req.StoreID); err != nil {
		sharederrors.WriteValidationError(w, "store_id", "invalid store_id UUID", reqID)
		return
	}
	if req.GateID == "" {
		sharederrors.WriteValidationError(w, "gate_id", "gate_id is required", reqID)
		return
	}
	if _, err := uuid.Parse(req.UserID); err != nil {
		sharederrors.WriteValidationError(w, "user_id", "invalid user_id UUID", reqID)
		return
	}
	if req.Reason == "" {
		sharederrors.WriteValidationError(w, "reason", "reason is required", reqID)
		return
	}

	res, err := h.exitSvc.StaffOverride(r.Context(), claims.UserID, &req)
	if err != nil {
		if appErr, ok := err.(*sharederrors.AppError); ok {
			sharederrors.WriteError(w, appErr.HTTPStatus, appErr.Code, appErr.Message, reqID)
			return
		}
		sharederrors.WriteError(w, http.StatusInternalServerError, sharederrors.ErrInternal, err.Error(), reqID)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(res)
}

func (h *ExitHandler) GetTokenStatus(w http.ResponseWriter, r *http.Request) {
	reqID := chimiddleware.GetReqID(r.Context())
	claims := middleware.GetClaimsFromContext(r.Context())
	if claims == nil {
		sharederrors.WriteError(w, http.StatusUnauthorized, sharederrors.ErrUnauthorized, "missing authentication claims", reqID)
		return
	}

	orderID := chi.URLParam(r, "order_id")
	if _, err := uuid.Parse(orderID); err != nil {
		sharederrors.WriteError(w, http.StatusBadRequest, sharederrors.ErrValidationFailed, "invalid order_id UUID", reqID)
		return
	}

	res, err := h.exitSvc.GetTokenStatus(r.Context(), orderID)
	if err != nil {
		if appErr, ok := err.(*sharederrors.AppError); ok {
			sharederrors.WriteError(w, appErr.HTTPStatus, appErr.Code, appErr.Message, reqID)
			return
		}
		sharederrors.WriteError(w, http.StatusInternalServerError, sharederrors.ErrInternal, err.Error(), reqID)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(res)
}
