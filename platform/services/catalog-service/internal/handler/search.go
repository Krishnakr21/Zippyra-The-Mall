package handler

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/zippyra/platform/services/catalog-service/internal/model"
	"github.com/zippyra/platform/services/catalog-service/internal/service"
	sharedErrors "github.com/zippyra/platform/shared/errors"
)

type SearchHandler struct {
	searchService service.SearchService
}

func NewSearchHandler(searchService service.SearchService) *SearchHandler {
	return &SearchHandler{
		searchService: searchService,
	}
}

func (h *SearchHandler) Search(w http.ResponseWriter, r *http.Request) {
	storeIDStr := r.URL.Query().Get("store_id")
	query := r.URL.Query().Get("q")
	category := r.URL.Query().Get("category")
	pageStr := r.URL.Query().Get("page")
	limitStr := r.URL.Query().Get("limit")

	if storeIDStr == "" {
		respondWithError(w, http.StatusBadRequest, "store_id query parameter is required")
		return
	}

	if query == "" {
		respondWithError(w, http.StatusBadRequest, "q query parameter is required")
		return
	}

	storeID, err := uuid.Parse(storeIDStr)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "invalid store_id")
		return
	}

	page := 1
	if pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	limit := 20
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	req := &model.ProductSearchRequest{
		StoreID: storeID,
		Query:   query,
		Page:    page,
		Limit:   limit,
	}

	if category != "" {
		req.Category = &category
	}

	response, err := h.searchService.Search(r.Context(), req)
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

func (h *SearchHandler) ByCategory(w http.ResponseWriter, r *http.Request) {
	category := chi.URLParam(r, "category")
	storeIDStr := r.URL.Query().Get("store_id")
	pageStr := r.URL.Query().Get("page")
	limitStr := r.URL.Query().Get("limit")

	if storeIDStr == "" {
		respondWithError(w, http.StatusBadRequest, "store_id query parameter is required")
		return
	}

	if category == "" {
		respondWithError(w, http.StatusBadRequest, "category is required")
		return
	}

	storeID, err := uuid.Parse(storeIDStr)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "invalid store_id")
		return
	}

	page := 1
	if pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	limit := 50
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	response, err := h.searchService.ByCategory(r.Context(), storeID, category, page, limit)
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
