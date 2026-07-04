package handler

import (
	"encoding/json"
	"net/http"

	"github.com/zippyra/platform/services/catalog-service/internal/model"
	"github.com/zippyra/platform/services/catalog-service/internal/service"
	sharedErrors "github.com/zippyra/platform/shared/errors"
)

type SyncHandler struct {
	syncService service.SyncService
}

func NewSyncHandler(syncService service.SyncService) *SyncHandler {
	return &SyncHandler{
		syncService: syncService,
	}
}

func (h *SyncHandler) Sync(w http.ResponseWriter, r *http.Request) {
	var req model.SyncRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	response, err := h.syncService.Sync(r.Context(), &req)
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
