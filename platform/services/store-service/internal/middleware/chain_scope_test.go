package middleware

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// ── Mock StoreChainLookup ───────────────────────────────────────────

type mockChainLookup struct {
	chainIDs map[string]uuid.UUID
}

func (m *mockChainLookup) GetStoreChainID(ctx context.Context, storeID uuid.UUID) (uuid.UUID, error) {
	if chainID, ok := m.chainIDs[storeID.String()]; ok {
		return chainID, nil
	}
	return uuid.Nil, nil
}

// ── Test helpers ────────────────────────────────────────────────────

func newTestRequest(storeID, userType, chainID string) *http.Request {
	r := httptest.NewRequest("GET", "/v1/store/"+storeID, nil)

	// Create a chi context with the URL params
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", storeID)
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

	// Set context values (as set by JWT middleware)
	ctx := context.WithValue(r.Context(), ContextUserType, userType)
	ctx = context.WithValue(ctx, ContextChainID, chainID)
	ctx = context.WithValue(ctx, ContextUserID, uuid.New().String())
	r = r.WithContext(ctx)

	return r
}

// ── Tests ───────────────────────────────────────────────────────────

func TestChainScope_ChainHQ_CrossChain_403(t *testing.T) {
	chainA := uuid.New()
	chainB := uuid.New()
	storeID := uuid.New() // belongs to chain B

	lookup := &mockChainLookup{
		chainIDs: map[string]uuid.UUID{
			storeID.String(): chainB,
		},
	}

	handler := ChainScope(lookup)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// CHAIN_HQ user from chain A accessing chain B store → 403
	r := newTestRequest(storeID.String(), "CHAIN_HQ", chainA.String())
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestChainScope_ChainHQ_SameChain_200(t *testing.T) {
	chainA := uuid.New()
	storeID := uuid.New() // belongs to chain A

	lookup := &mockChainLookup{
		chainIDs: map[string]uuid.UUID{
			storeID.String(): chainA,
		},
	}

	handler := ChainScope(lookup)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// CHAIN_HQ user from chain A accessing chain A store → 200
	r := newTestRequest(storeID.String(), "CHAIN_HQ", chainA.String())
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestChainScope_Customer_AnyStore_200(t *testing.T) {
	chainA := uuid.New()
	storeID := uuid.New()

	lookup := &mockChainLookup{
		chainIDs: map[string]uuid.UUID{
			storeID.String(): chainA,
		},
	}

	handler := ChainScope(lookup)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// CUSTOMER accessing any store → 200 (no chain restriction)
	r := newTestRequest(storeID.String(), "CUSTOMER", "")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestChainScope_Staff_CrossStore_403(t *testing.T) {
	chainA := uuid.New()
	chainB := uuid.New()
	storeID := uuid.New() // belongs to chain B

	lookup := &mockChainLookup{
		chainIDs: map[string]uuid.UUID{
			storeID.String(): chainB,
		},
	}

	handler := ChainScope(lookup)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// STAFF from chain A accessing chain B's store → 403
	r := newTestRequest(storeID.String(), "STAFF", chainA.String())
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestChainScope_Staff_SameChain_200(t *testing.T) {
	chainA := uuid.New()
	storeID := uuid.New() // belongs to chain A

	lookup := &mockChainLookup{
		chainIDs: map[string]uuid.UUID{
			storeID.String(): chainA,
		},
	}

	handler := ChainScope(lookup)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// STAFF from chain A accessing chain A store → 200
	r := newTestRequest(storeID.String(), "STAFF", chainA.String())
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestChainScope_NoUserType_PassThrough(t *testing.T) {
	storeID := uuid.New()

	lookup := &mockChainLookup{
		chainIDs: map[string]uuid.UUID{},
	}

	handler := ChainScope(lookup)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// No user type in context → pass through
	r := newTestRequest(storeID.String(), "", "")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestChainScope_NoStoreID_PassThrough(t *testing.T) {
	lookup := &mockChainLookup{
		chainIDs: map[string]uuid.UUID{},
	}

	handler := ChainScope(lookup)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// No store ID in URL → pass through
	r := httptest.NewRequest("GET", "/v1/store/nearby", nil)
	ctx := context.WithValue(r.Context(), ContextUserType, "STAFF")
	ctx = context.WithValue(ctx, ContextChainID, uuid.New().String())
	r = r.WithContext(ctx)

	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestChainScope_Staff_NoChainIDInToken_403(t *testing.T) {
	storeID := uuid.New()

	lookup := &mockChainLookup{
		chainIDs: map[string]uuid.UUID{
			storeID.String(): uuid.New(),
		},
	}

	handler := ChainScope(lookup)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// STAFF with no chain_id in JWT → 403
	r := newTestRequest(storeID.String(), "STAFF", "")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestChainScope_InvalidStoreID_400(t *testing.T) {
	lookup := &mockChainLookup{
		chainIDs: map[string]uuid.UUID{},
	}

	handler := ChainScope(lookup)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Staff with invalid UUID store ID → 400
	r := newTestRequest("not-a-uuid", "STAFF", uuid.New().String())
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}
func TestChainScope_Admin_AnyStore_200(t *testing.T) {
	storeID := uuid.New()
	lookup := &mockChainLookup{}

	handler := ChainScope(lookup)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// ADMIN accessing any store → 200
	r := newTestRequest(storeID.String(), "ADMIN", "")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

type errorChainLookup struct{}

func (e *errorChainLookup) GetStoreChainID(ctx context.Context, storeID uuid.UUID) (uuid.UUID, error) {
	return uuid.Nil, fmt.Errorf("db error")
}

func TestChainScope_LookupError_500(t *testing.T) {
	storeID := uuid.New()
	lookup := &errorChainLookup{}

	handler := ChainScope(lookup)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	r := newTestRequest(storeID.String(), "STAFF", uuid.New().String())
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}
