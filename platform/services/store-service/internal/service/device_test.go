package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/zippyra/platform/services/store-service/internal/model"
)

// ── Mock DeviceRepo ─────────────────────────────────────────────────

type mockDeviceRepo struct {
	listFn func(ctx context.Context, storeID uuid.UUID) ([]model.Device, error)
}

func (m *mockDeviceRepo) ListByStoreID(ctx context.Context, storeID uuid.UUID) ([]model.Device, error) {
	if m.listFn != nil {
		return m.listFn(ctx, storeID)
	}
	return nil, nil
}

// ── Tests ───────────────────────────────────────────────────────────

func TestListDevices_Service(t *testing.T) {
	storeID := uuid.New()
	deviceID := uuid.New()

	repo := &mockDeviceRepo{
		listFn: func(ctx context.Context, id uuid.UUID) ([]model.Device, error) {
			return []model.Device{
				{ID: deviceID, SerialNumber: "SN123"},
			}, nil
		},
	}

	// mock redis for heartbeats - use RFC3339 timestamp within last 5 minutes
	redis := newMockRedis()
	redis.data["device_heartbeat:"+deviceID.String()] = time.Now().Add(-1 * time.Minute).Format(time.RFC3339)

	svc := NewDeviceService(repo, redis)

	res, err := svc.ListDevices(context.Background(), storeID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(res.Devices) != 1 {
		t.Errorf("expected 1 device, got %d", len(res.Devices))
	}

	if !res.Devices[0].IsOnline {
		t.Error("expected device to be online")
	}

	if res.Summary.Total != 1 || res.Summary.Online != 1 {
		t.Errorf("unexpected summary: %+v", res.Summary)
	}
}

func TestListDevices_RepoError(t *testing.T) {
	repo := &mockDeviceRepo{
		listFn: func(ctx context.Context, id uuid.UUID) ([]model.Device, error) {
			return nil, fmt.Errorf("db error")
		},
	}

	svc := NewDeviceService(repo, &mockRedis{})

	_, err := svc.ListDevices(context.Background(), uuid.New())
	if err == nil {
		t.Error("expected error, got nil")
	}
}
