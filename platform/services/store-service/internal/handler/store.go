package handler

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	mw "github.com/zippyra/platform/services/store-service/internal/middleware"
	"github.com/zippyra/platform/services/store-service/internal/model"
	"github.com/zippyra/platform/shared/errors"
)

// storeService defines the interface for store operations.
type storeService interface {
	Bind(ctx context.Context, userID, qrToken, deviceID string) (*model.BindResponse, error)
	GetStore(ctx context.Context, storeID uuid.UUID) (*model.StoreInfoResponse, error)
	NearbyStores(ctx context.Context, lat, lng, radiusKM float64) ([]model.NearbyStore, error)
	Hours(ctx context.Context, storeID uuid.UUID) (*model.HoursResponse, error)
	UpdateCapacity(ctx context.Context, storeID, action string) (*model.CapacityUpdateResponse, error)
	Exit(ctx context.Context, userID, storeID string) (*model.ExitResponse, error)
	Occupancy(ctx context.Context, storeID uuid.UUID) (*model.OccupancyResponse, error)
}

// StoreHandler handles all store HTTP endpoints.
type StoreHandler struct {
	svc storeService
}

// NewStoreHandler creates a new StoreHandler.
func NewStoreHandler(svc storeService) *StoreHandler {
	return &StoreHandler{svc: svc}
}

// Bind handles POST /v1/store/bind
func (h *StoreHandler) Bind(w http.ResponseWriter, r *http.Request) {
	requestID := r.Header.Get("X-Request-ID")
	userID := mw.GetUserIDFromContext(r.Context())

	var req model.BindRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errors.WriteError(w, http.StatusBadRequest, errors.ErrValidationFailed,
			"Invalid request body", requestID)
		return
	}

	if req.QRToken == "" {
		errors.WriteValidationError(w, "qr_token", "qr_token is required", requestID)
		return
	}
	if req.DeviceID == "" {
		errors.WriteValidationError(w, "device_id", "device_id is required", requestID)
		return
	}

	result, err := h.svc.Bind(r.Context(), userID, req.QRToken, req.DeviceID)
	if err != nil {
		writeAppError(w, err, requestID)
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// GetStore handles GET /v1/store/{id}
func (h *StoreHandler) GetStore(w http.ResponseWriter, r *http.Request) {
	requestID := r.Header.Get("X-Request-ID")
	storeID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		errors.WriteError(w, http.StatusBadRequest, errors.ErrValidationFailed,
			"Invalid store ID", requestID)
		return
	}

	result, err := h.svc.GetStore(r.Context(), storeID)
	if err != nil {
		writeAppError(w, err, requestID)
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// Nearby handles GET /v1/store/nearby
func (h *StoreHandler) Nearby(w http.ResponseWriter, r *http.Request) {
	requestID := r.Header.Get("X-Request-ID")

	latStr := r.URL.Query().Get("lat")
	lngStr := r.URL.Query().Get("lng")
	radiusStr := r.URL.Query().Get("radius")

	lat, err := strconv.ParseFloat(latStr, 64)
	if err != nil || lat < -90 || lat > 90 {
		errors.WriteValidationError(w, "lat", "Invalid latitude. Must be between -90 and 90", requestID)
		return
	}

	lng, err := strconv.ParseFloat(lngStr, 64)
	if err != nil || lng < -180 || lng > 180 {
		errors.WriteValidationError(w, "lng", "Invalid longitude. Must be between -180 and 180", requestID)
		return
	}

	radius := 5.0 // default 5km
	if radiusStr != "" {
		radius, err = strconv.ParseFloat(radiusStr, 64)
		if err != nil || radius < 1 || radius > 50 {
			errors.WriteValidationError(w, "radius", "Radius must be between 1 and 50 km", requestID)
			return
		}
	}

	// Round to avoid floating point precision issues
	lat = math.Round(lat*1e6) / 1e6
	lng = math.Round(lng*1e6) / 1e6

	result, err := h.svc.NearbyStores(r.Context(), lat, lng, radius)
	if err != nil {
		writeAppError(w, err, requestID)
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// Hours handles GET /v1/store/{id}/hours
func (h *StoreHandler) Hours(w http.ResponseWriter, r *http.Request) {
	requestID := r.Header.Get("X-Request-ID")
	storeID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		errors.WriteError(w, http.StatusBadRequest, errors.ErrValidationFailed,
			"Invalid store ID", requestID)
		return
	}

	result, err := h.svc.Hours(r.Context(), storeID)
	if err != nil {
		writeAppError(w, err, requestID)
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// UpdateCapacity handles PUT /v1/store/{id}/capacity
func (h *StoreHandler) UpdateCapacity(w http.ResponseWriter, r *http.Request) {
	requestID := r.Header.Get("X-Request-ID")
	storeID := chi.URLParam(r, "id")
	if _, err := uuid.Parse(storeID); err != nil {
		errors.WriteError(w, http.StatusBadRequest, errors.ErrValidationFailed,
			"Invalid store ID", requestID)
		return
	}

	var req model.CapacityUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errors.WriteError(w, http.StatusBadRequest, errors.ErrValidationFailed,
			"Invalid request body", requestID)
		return
	}

	if req.Action != "increment" && req.Action != "decrement" {
		errors.WriteValidationError(w, "action", "Action must be 'increment' or 'decrement'", requestID)
		return
	}

	result, err := h.svc.UpdateCapacity(r.Context(), storeID, req.Action)
	if err != nil {
		writeAppError(w, err, requestID)
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// Exit handles POST /v1/store/{id}/exit
func (h *StoreHandler) Exit(w http.ResponseWriter, r *http.Request) {
	requestID := r.Header.Get("X-Request-ID")
	userID := mw.GetUserIDFromContext(r.Context())
	storeID := chi.URLParam(r, "id")
	if _, err := uuid.Parse(storeID); err != nil {
		errors.WriteError(w, http.StatusBadRequest, errors.ErrValidationFailed,
			"Invalid store ID", requestID)
		return
	}

	result, err := h.svc.Exit(r.Context(), userID, storeID)
	if err != nil {
		writeAppError(w, err, requestID)
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// Occupancy handles GET /v1/store/{id}/occupancy
func (h *StoreHandler) Occupancy(w http.ResponseWriter, r *http.Request) {
	requestID := r.Header.Get("X-Request-ID")
	storeID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		errors.WriteError(w, http.StatusBadRequest, errors.ErrValidationFailed,
			"Invalid store ID", requestID)
		return
	}

	result, err := h.svc.Occupancy(r.Context(), storeID)
	if err != nil {
		writeAppError(w, err, requestID)
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// writeJSON writes a JSON response.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Error().Err(err).Msg("failed to encode response")
	}
}

// writeAppError maps an AppError to an HTTP error response.
func writeAppError(w http.ResponseWriter, err error, requestID string) {
	if appErr, ok := err.(*errors.AppError); ok {
		errors.WriteError(w, appErr.HTTPStatus, appErr.Code, appErr.Message, requestID)
		return
	}
	log.Error().Err(err).Msg("unexpected error")
	errors.WriteInternalError(w, requestID)
}
