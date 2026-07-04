package handler

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/zippyra/platform/services/store-service/internal/model"
	"github.com/zippyra/platform/shared/errors"
)

// deviceService defines the interface for device operations.
type deviceService interface {
	ListDevices(ctx context.Context, storeID uuid.UUID) (*model.DeviceListResponse, error)
}

// DeviceHandler handles device HTTP endpoints.
type DeviceHandler struct {
	svc deviceService
}

// NewDeviceHandler creates a new DeviceHandler.
func NewDeviceHandler(svc deviceService) *DeviceHandler {
	return &DeviceHandler{svc: svc}
}

// ListDevices handles GET /v1/store/{id}/devices
func (h *DeviceHandler) ListDevices(w http.ResponseWriter, r *http.Request) {
	requestID := r.Header.Get("X-Request-ID")
	storeID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		errors.WriteError(w, http.StatusBadRequest, errors.ErrValidationFailed,
			"Invalid store ID", requestID)
		return
	}

	result, err := h.svc.ListDevices(r.Context(), storeID)
	if err != nil {
		writeAppError(w, err, requestID)
		return
	}

	writeJSON(w, http.StatusOK, result)
}
