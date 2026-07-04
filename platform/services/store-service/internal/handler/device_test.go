package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/zippyra/platform/services/store-service/internal/model"
)

// ── Mock DeviceService ──────────────────────────────────────────────

type mockDeviceService struct {
	listDevicesFn func(ctx context.Context, storeID uuid.UUID) (*model.DeviceListResponse, error)
}

func (m *mockDeviceService) ListDevices(ctx context.Context, storeID uuid.UUID) (*model.DeviceListResponse, error) {
	if m.listDevicesFn != nil {
		return m.listDevicesFn(ctx, storeID)
	}
	return nil, nil
}

// ── Tests ───────────────────────────────────────────────────────────

func TestListDevices_Handler(t *testing.T) {
	storeID := uuid.New()
	svc := &mockDeviceService{
		listDevicesFn: func(ctx context.Context, id uuid.UUID) (*model.DeviceListResponse, error) {
			return &model.DeviceListResponse{
				Devices: []model.Device{{ID: uuid.New(), SerialNumber: "SN123"}},
				Summary: model.DeviceSummary{Total: 1, Online: 1},
			}, nil
		},
	}

	h := NewDeviceHandler(svc)

	req := httptest.NewRequest("GET", "/v1/store/"+storeID.String()+"/devices", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", storeID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	h.ListDevices(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var res model.DeviceListResponse
	json.Unmarshal(w.Body.Bytes(), &res)
	if res.Summary.Total != 1 {
		t.Errorf("expected 1 device, got %d", res.Summary.Total)
	}
}

func TestListDevices_InvalidUUID(t *testing.T) {
	h := NewDeviceHandler(&mockDeviceService{})

	req := httptest.NewRequest("GET", "/v1/store/invalid-uuid/devices", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "invalid-uuid")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	h.ListDevices(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestListDevices_ServiceError(t *testing.T) {
	storeID := uuid.New()
	svc := &mockDeviceService{
		listDevicesFn: func(ctx context.Context, id uuid.UUID) (*model.DeviceListResponse, error) {
			return nil, errors.New("service error")
		},
	}

	h := NewDeviceHandler(svc)

	req := httptest.NewRequest("GET", "/v1/store/"+storeID.String()+"/devices", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", storeID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	h.ListDevices(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}
