package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/zippyra/platform/services/catalog-service/internal/model"
	"github.com/zippyra/platform/services/catalog-service/internal/service"
	sharedErrors "github.com/zippyra/platform/shared/errors"
)

type BarcodeHandler struct {
	barcodeService service.BarcodeService
}

func NewBarcodeHandler(barcodeService service.BarcodeService) *BarcodeHandler {
	return &BarcodeHandler{
		barcodeService: barcodeService,
	}
}

func (h *BarcodeHandler) Lookup(w http.ResponseWriter, r *http.Request) {
	barcode := chi.URLParam(r, "barcode")
	storeIDStr := r.URL.Query().Get("store_id")

	if storeIDStr == "" {
		respondWithError(w, http.StatusBadRequest, "store_id query parameter is required")
		return
	}

	storeID, err := uuid.Parse(storeIDStr)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "invalid store_id")
		return
	}

	response, err := h.barcodeService.Lookup(r.Context(), storeID, barcode)
	if err != nil {
		if appErr, ok := err.(*sharedErrors.AppError); ok {
			switch appErr.Code {
			case sharedErrors.ErrValidationFailed:
				respondWithError(w, http.StatusBadRequest, appErr.Message)
				return
			case sharedErrors.ErrBarcodeNotFound, sharedErrors.ErrProductNotFound, sharedErrors.ErrNotFound:
				respondWithError(w, http.StatusNotFound, "product not found")
				return
			default:
				respondWithError(w, http.StatusInternalServerError, "internal server error")
				return
			}
		}
		respondWithError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	respondWithJSON(w, http.StatusOK, response)
}

func (h *BarcodeHandler) UpsertProduct(w http.ResponseWriter, r *http.Request) {
	var req model.UpsertProductRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	product, err := h.barcodeService.UpsertProduct(r.Context(), &req)
	if err != nil {
		if appErr, ok := err.(*sharedErrors.AppError); ok {
			switch appErr.Code {
			case sharedErrors.ErrValidationFailed:
				respondWithError(w, http.StatusBadRequest, appErr.Message)
				return
			default:
				respondWithError(w, http.StatusInternalServerError, "internal server error")
				return
			}
		}
		respondWithError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	respondWithJSON(w, http.StatusOK, product)
}
