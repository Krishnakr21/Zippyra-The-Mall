package middleware

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/zippyra/platform/shared/errors"
)

// StoreChainLookup is the interface for looking up a store's chain_id.
type StoreChainLookup interface {
	GetStoreChainID(ctx context.Context, storeID uuid.UUID) (uuid.UUID, error)
}

// ChainScope middleware enforces that CHAIN_HQ and STAFF users
// can only access stores belonging to their chain.
// NEVER skip this middleware on any store endpoint.
func ChainScope(lookup StoreChainLookup) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := r.Header.Get("X-Request-ID")

			// Get user type from context
			userType := GetUserTypeFromContext(r.Context())
			if userType == "" {
				next.ServeHTTP(w, r)
				return
			}

			// CUSTOMER or ADMIN user_type: no chain restriction
			if userType == "CUSTOMER" || userType == "ADMIN" {
				next.ServeHTTP(w, r)
				return
			}

			// STAFF and CHAIN_HQ: enforce chain_id from JWT
			storeID := chi.URLParam(r, "id")
			if storeID == "" {
				next.ServeHTTP(w, r)
				return
			}

			storeUUID, err := uuid.Parse(storeID)
			if err != nil {
				errors.WriteError(w, http.StatusBadRequest, errors.ErrValidationFailed,
					"Invalid store ID format", requestID)
				return
			}

			// chain_id extracted from JWT claims — NEVER from request body
			jwtChainID := GetChainIDFromContext(r.Context())
			if jwtChainID == "" {
				errors.WriteError(w, http.StatusForbidden, errors.ErrForbidden,
					"No chain_id in token", requestID)
				return
			}

			// Verify store belongs to JWT chain_id
			storeChainID, err := lookup.GetStoreChainID(r.Context(), storeUUID)
			if err != nil {
				log.Error().Err(err).Str("store_id", storeID).Msg("chain scope: store lookup failed")
				errors.WriteInternalError(w, requestID)
				return
			}

			if storeChainID.String() != jwtChainID {
				// If mismatch → 403 ErrForbidden
				errors.WriteError(w, http.StatusForbidden, errors.ErrForbidden,
					"Store does not belong to your chain", requestID)
				return
			}

			// If match → proceed
			next.ServeHTTP(w, r)
		})
	}
}
