package handler

import (
	"net/http"

	"github.com/google/uuid"

	"github.com/zippyra/platform/services/catalog-service/internal/service"
	sharedErrors "github.com/zippyra/platform/shared/errors"
)

type OfferHandler struct {
	offerService service.OfferService
}

func NewOfferHandler(offerService service.OfferService) *OfferHandler {
	return &OfferHandler{
		offerService: offerService,
	}
}

func (h *OfferHandler) GetOffers(w http.ResponseWriter, r *http.Request) {
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

	response, err := h.offerService.GetOffers(r.Context(), storeID)
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

	respondWithJSON(w, http.StatusOK, response)
}
