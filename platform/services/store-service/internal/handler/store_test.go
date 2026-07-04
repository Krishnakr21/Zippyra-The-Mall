package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/zippyra/platform/services/store-service/internal/middleware"
	"github.com/zippyra/platform/services/store-service/internal/model"
	apperrors "github.com/zippyra/platform/shared/errors"
)

// ── Mock StoreService ────────────────────────────────────────────────

type mockStoreService struct {
	bindFn           func(ctx context.Context, userID, qrToken, deviceID string) (*model.BindResponse, error)
	getStoreFn       func(ctx context.Context, storeID uuid.UUID) (*model.StoreInfoResponse, error)
	nearbyStoresFn   func(ctx context.Context, lat, lng, radiusKM float64) ([]model.NearbyStore, error)
	hoursFn          func(ctx context.Context, storeID uuid.UUID) (*model.HoursResponse, error)
	updateCapacityFn func(ctx context.Context, storeID, action string) (*model.CapacityUpdateResponse, error)
	exitFn           func(ctx context.Context, userID, storeID string) (*model.ExitResponse, error)
	occupancyFn      func(ctx context.Context, storeID uuid.UUID) (*model.OccupancyResponse, error)
}

func (m *mockStoreService) Bind(ctx context.Context, userID, qrToken, deviceID string) (*model.BindResponse, error) {
	if m.bindFn != nil {
		return m.bindFn(ctx, userID, qrToken, deviceID)
	}
	return nil, nil
}

func (m *mockStoreService) GetStore(ctx context.Context, storeID uuid.UUID) (*model.StoreInfoResponse, error) {
	if m.getStoreFn != nil {
		return m.getStoreFn(ctx, storeID)
	}
	return nil, nil
}

func (m *mockStoreService) NearbyStores(ctx context.Context, lat, lng, radiusKM float64) ([]model.NearbyStore, error) {
	if m.nearbyStoresFn != nil {
		return m.nearbyStoresFn(ctx, lat, lng, radiusKM)
	}
	return nil, nil
}

func (m *mockStoreService) Hours(ctx context.Context, storeID uuid.UUID) (*model.HoursResponse, error) {
	if m.hoursFn != nil {
		return m.hoursFn(ctx, storeID)
	}
	return nil, nil
}

func (m *mockStoreService) UpdateCapacity(ctx context.Context, storeID, action string) (*model.CapacityUpdateResponse, error) {
	if m.updateCapacityFn != nil {
		return m.updateCapacityFn(ctx, storeID, action)
	}
	return nil, nil
}

func (m *mockStoreService) Exit(ctx context.Context, userID, storeID string) (*model.ExitResponse, error) {
	if m.exitFn != nil {
		return m.exitFn(ctx, userID, storeID)
	}
	return nil, nil
}

func (m *mockStoreService) Occupancy(ctx context.Context, storeID uuid.UUID) (*model.OccupancyResponse, error) {
	if m.occupancyFn != nil {
		return m.occupancyFn(ctx, storeID)
	}
	return nil, nil
}

// ── Tests ───────────────────────────────────────────────────────────

func TestBind_Handler(t *testing.T) {
	userID := uuid.New().String()
	storeID := uuid.New()

	svc := &mockStoreService{
		bindFn: func(ctx context.Context, u, q, d string) (*model.BindResponse, error) {
			if u != userID {
				return nil, fmt.Errorf("wrong user")
			}
			return &model.BindResponse{
				StoreID:        storeID,
				StoreName:      "Test Store",
				CatalogVersion: 1,
			}, nil
		},
	}

	h := NewStoreHandler(svc)

	body := map[string]string{
		"qr_token":  "token-123",
		"device_id": uuid.New().String(),
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/v1/store/bind", bytes.NewBuffer(jsonBody))
	ctx := context.WithValue(req.Context(), middleware.ContextUserID, userID)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	h.Bind(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var res model.BindResponse
	json.Unmarshal(w.Body.Bytes(), &res)
	if res.StoreID != storeID {
		t.Errorf("expected store id %s, got %s", storeID, res.StoreID)
	}
}

func TestNearby_Handler(t *testing.T) {
	svc := &mockStoreService{
		nearbyStoresFn: func(ctx context.Context, lat, lng, rad float64) ([]model.NearbyStore, error) {
			return []model.NearbyStore{
				{StoreInfoResponse: model.StoreInfoResponse{Name: "Store A"}, DistanceKM: 1.5},
			}, nil
		},
	}

	h := NewStoreHandler(svc)

	req := httptest.NewRequest("GET", "/v1/store/nearby?lat=12.97&lng=77.59&radius=5", nil)
	w := httptest.NewRecorder()
	h.Nearby(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var res []model.NearbyStore
	json.Unmarshal(w.Body.Bytes(), &res)
	if len(res) != 1 || res[0].Name != "Store A" {
		t.Errorf("unexpected response: %+v", res)
	}
}

func TestGetStore_Handler(t *testing.T) {
	storeID := uuid.New()
	svc := &mockStoreService{
		getStoreFn: func(ctx context.Context, id uuid.UUID) (*model.StoreInfoResponse, error) {
			return &model.StoreInfoResponse{ID: id, Name: "Found Store"}, nil
		},
	}

	h := NewStoreHandler(svc)

	req := httptest.NewRequest("GET", "/v1/store/"+storeID.String(), nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", storeID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	h.GetStore(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var res model.StoreInfoResponse
	json.Unmarshal(w.Body.Bytes(), &res)
	if res.Name != "Found Store" {
		t.Errorf("expected 'Found Store', got '%s'", res.Name)
	}
}

func TestUpdateCapacity_Handler(t *testing.T) {
	storeID := uuid.New()
	svc := &mockStoreService{
		updateCapacityFn: func(ctx context.Context, id, action string) (*model.CapacityUpdateResponse, error) {
			return &model.CapacityUpdateResponse{CurrentOccupancy: 10}, nil
		},
	}

	h := NewStoreHandler(svc)

	body := map[string]string{"action": "increment"}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("PUT", "/v1/store/"+storeID.String()+"/capacity", bytes.NewBuffer(jsonBody))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", storeID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	h.UpdateCapacity(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestExit_Handler(t *testing.T) {
	storeID := uuid.New()
	userID := uuid.New().String()

	svc := &mockStoreService{
		exitFn: func(ctx context.Context, u, s string) (*model.ExitResponse, error) {
			return &model.ExitResponse{Message: "Bye"}, nil
		},
	}

	h := NewStoreHandler(svc)

	req := httptest.NewRequest("POST", "/v1/store/"+storeID.String()+"/exit", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", storeID.String())
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = context.WithValue(ctx, middleware.ContextUserID, userID)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	h.Exit(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHours_Handler(t *testing.T) {
	storeID := uuid.New()
	svc := &mockStoreService{
		hoursFn: func(ctx context.Context, id uuid.UUID) (*model.HoursResponse, error) {
			return &model.HoursResponse{IsOpen: true}, nil
		},
	}

	h := NewStoreHandler(svc)

	req := httptest.NewRequest("GET", "/v1/store/"+storeID.String()+"/hours", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", storeID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	h.Hours(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestOccupancy_Handler(t *testing.T) {
	storeID := uuid.New()
	svc := &mockStoreService{
		occupancyFn: func(ctx context.Context, id uuid.UUID) (*model.OccupancyResponse, error) {
			return &model.OccupancyResponse{CurrentOccupancy: 5}, nil
		},
	}

	h := NewStoreHandler(svc)

	req := httptest.NewRequest("GET", "/v1/store/"+storeID.String()+"/occupancy", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", storeID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	h.Occupancy(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHandler_Errors(t *testing.T) {
	svc := &mockStoreService{
		getStoreFn: func(ctx context.Context, id uuid.UUID) (*model.StoreInfoResponse, error) {
			return nil, &apperrors.AppError{Code: apperrors.ErrNotFound, HTTPStatus: 404}
		},
	}

	h := NewStoreHandler(svc)

	req := httptest.NewRequest("GET", "/v1/store/"+uuid.New().String(), nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", uuid.New().String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	h.GetStore(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// ── Additional Handler Tests for 100% Coverage ────────────────────────

func TestNewStoreHandler(t *testing.T) {
	svc := &mockStoreService{}
	h := NewStoreHandler(svc)
	if h == nil {
		t.Error("expected handler to not be nil")
	}
	if h.svc != svc {
		t.Error("expected svc to be set")
	}
}

func TestBind_InvalidJSON(t *testing.T) {
	svc := &mockStoreService{}
	h := NewStoreHandler(svc)

	req := httptest.NewRequest("POST", "/v1/store/bind", bytes.NewBufferString("invalid json"))
	w := httptest.NewRecorder()

	h.Bind(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestBind_MissingQRToken(t *testing.T) {
	svc := &mockStoreService{}
	h := NewStoreHandler(svc)

	body := map[string]string{"device_id": "device-1"}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/v1/store/bind", bytes.NewBuffer(jsonBody))
	w := httptest.NewRecorder()

	h.Bind(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestBind_MissingDeviceID(t *testing.T) {
	svc := &mockStoreService{}
	h := NewStoreHandler(svc)

	body := map[string]string{"qr_token": "token-1"}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/v1/store/bind", bytes.NewBuffer(jsonBody))
	w := httptest.NewRecorder()

	h.Bind(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestGetStore_InvalidUUID(t *testing.T) {
	svc := &mockStoreService{}
	h := NewStoreHandler(svc)

	req := httptest.NewRequest("GET", "/v1/store/invalid-uuid", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "invalid-uuid")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	h.GetStore(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestNearby_InvalidLat(t *testing.T) {
	svc := &mockStoreService{}
	h := NewStoreHandler(svc)

	req := httptest.NewRequest("GET", "/v1/store/nearby?lat=invalid&lng=80.0", nil)
	w := httptest.NewRecorder()

	h.Nearby(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestNearby_LatOutOfRange(t *testing.T) {
	svc := &mockStoreService{}
	h := NewStoreHandler(svc)

	req := httptest.NewRequest("GET", "/v1/store/nearby?lat=95&lng=80.0", nil)
	w := httptest.NewRecorder()

	h.Nearby(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestNearby_InvalidLng(t *testing.T) {
	svc := &mockStoreService{}
	h := NewStoreHandler(svc)

	req := httptest.NewRequest("GET", "/v1/store/nearby?lat=12.0&lng=invalid", nil)
	w := httptest.NewRecorder()

	h.Nearby(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestNearby_LngOutOfRange(t *testing.T) {
	svc := &mockStoreService{}
	h := NewStoreHandler(svc)

	req := httptest.NewRequest("GET", "/v1/store/nearby?lat=12.0&lng=200", nil)
	w := httptest.NewRecorder()

	h.Nearby(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestNearby_RadiusOutOfRange(t *testing.T) {
	svc := &mockStoreService{}
	h := NewStoreHandler(svc)

	req := httptest.NewRequest("GET", "/v1/store/nearby?lat=12.0&lng=80.0&radius=100", nil)
	w := httptest.NewRecorder()

	h.Nearby(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHours_InvalidUUID(t *testing.T) {
	svc := &mockStoreService{}
	h := NewStoreHandler(svc)

	req := httptest.NewRequest("GET", "/v1/store/invalid/hours", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "invalid")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	h.Hours(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestUpdateCapacity_InvalidUUID(t *testing.T) {
	svc := &mockStoreService{}
	h := NewStoreHandler(svc)

	req := httptest.NewRequest("PUT", "/v1/store/invalid/capacity", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "invalid")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	h.UpdateCapacity(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestUpdateCapacity_InvalidJSON(t *testing.T) {
	svc := &mockStoreService{}
	h := NewStoreHandler(svc)

	req := httptest.NewRequest("PUT", "/v1/store/"+uuid.New().String()+"/capacity", bytes.NewBufferString("invalid"))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", uuid.New().String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	h.UpdateCapacity(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestUpdateCapacity_InvalidAction(t *testing.T) {
	svc := &mockStoreService{}
	h := NewStoreHandler(svc)

	body := map[string]string{"action": "invalid"}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("PUT", "/v1/store/"+uuid.New().String()+"/capacity", bytes.NewBuffer(jsonBody))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", uuid.New().String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	h.UpdateCapacity(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestExit_InvalidUUID(t *testing.T) {
	svc := &mockStoreService{}
	h := NewStoreHandler(svc)

	req := httptest.NewRequest("POST", "/v1/store/invalid/exit", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "invalid")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	h.Exit(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestOccupancy_InvalidUUID(t *testing.T) {
	svc := &mockStoreService{}
	h := NewStoreHandler(svc)

	req := httptest.NewRequest("GET", "/v1/store/invalid/occupancy", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "invalid")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	h.Occupancy(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestBind_ServiceError(t *testing.T) {
	svc := &mockStoreService{
		bindFn: func(ctx context.Context, userID, qrToken, deviceID string) (*model.BindResponse, error) {
			return nil, errors.New("service error")
		},
	}
	h := NewStoreHandler(svc)

	body, _ := json.Marshal(model.BindRequest{QRToken: "token-1", DeviceID: "device-1"})
	req := httptest.NewRequest("POST", "/v1/store/bind", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	h.Bind(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestHours_ServiceError(t *testing.T) {
	svc := &mockStoreService{
		hoursFn: func(ctx context.Context, storeID uuid.UUID) (*model.HoursResponse, error) {
			return nil, errors.New("service error")
		},
	}
	h := NewStoreHandler(svc)

	req := httptest.NewRequest("GET", "/v1/store/"+uuid.New().String()+"/hours", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", uuid.New().String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	h.Hours(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestUpdateCapacity_ServiceError(t *testing.T) {
	svc := &mockStoreService{
		updateCapacityFn: func(ctx context.Context, storeID, action string) (*model.CapacityUpdateResponse, error) {
			return nil, errors.New("service error")
		},
	}
	h := NewStoreHandler(svc)

	body, _ := json.Marshal(model.CapacityUpdateRequest{Action: "increment"})
	req := httptest.NewRequest("PUT", "/v1/store/"+uuid.New().String()+"/capacity", bytes.NewBuffer(body))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", uuid.New().String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	h.UpdateCapacity(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestExit_ServiceError(t *testing.T) {
	svc := &mockStoreService{
		exitFn: func(ctx context.Context, userID, storeID string) (*model.ExitResponse, error) {
			return nil, errors.New("service error")
		},
	}
	h := NewStoreHandler(svc)

	req := httptest.NewRequest("POST", "/v1/store/"+uuid.New().String()+"/exit", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", uuid.New().String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	h.Exit(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestOccupancy_ServiceError(t *testing.T) {
	svc := &mockStoreService{
		occupancyFn: func(ctx context.Context, storeID uuid.UUID) (*model.OccupancyResponse, error) {
			return nil, errors.New("service error")
		},
	}
	h := NewStoreHandler(svc)

	req := httptest.NewRequest("GET", "/v1/store/"+uuid.New().String()+"/occupancy", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", uuid.New().String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	h.Occupancy(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestNearby_ServiceError(t *testing.T) {
	svc := &mockStoreService{
		nearbyStoresFn: func(ctx context.Context, lat, lng, radiusKM float64) ([]model.NearbyStore, error) {
			return nil, errors.New("service error")
		},
	}
	h := NewStoreHandler(svc)

	req := httptest.NewRequest("GET", "/v1/store/nearby?lat=12.0&lng=80.0&radius=5", nil)
	w := httptest.NewRecorder()

	h.Nearby(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestGetStore_ServiceError(t *testing.T) {
	svc := &mockStoreService{
		getStoreFn: func(ctx context.Context, storeID uuid.UUID) (*model.StoreInfoResponse, error) {
			return nil, errors.New("service error")
		},
	}
	h := NewStoreHandler(svc)

	req := httptest.NewRequest("GET", "/v1/store/"+uuid.New().String(), nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", uuid.New().String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	h.GetStore(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestWriteAppError_NonAppError(t *testing.T) {
	w := httptest.NewRecorder()
	writeAppError(w, fmt.Errorf("generic error"), "req-1")

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestWriteJSON_EncodeError(t *testing.T) {
	// Test with a type that can't be JSON encoded
	w := httptest.NewRecorder()
	writeJSON(w, http.StatusOK, make(chan int)) // channels can't be JSON encoded

	// Should still write headers but encoding will fail (silently logged)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}
