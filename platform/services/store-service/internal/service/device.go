package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/zippyra/platform/services/store-service/internal/model"
	"github.com/zippyra/platform/shared/errors"
)

// DeviceService handles device-related operations.
type DeviceService struct {
	deviceRepo DeviceRepo
	redis      RedisStore
}

// NewDeviceService creates a new DeviceService.
func NewDeviceService(deviceRepo DeviceRepo, redis RedisStore) *DeviceService {
	return &DeviceService{
		deviceRepo: deviceRepo,
		redis:      redis,
	}
}

// ListDevices returns all registered devices for a store with online/offline status.
func (s *DeviceService) ListDevices(ctx context.Context, storeID uuid.UUID) (*model.DeviceListResponse, error) {
	devices, err := s.deviceRepo.ListByStoreID(ctx, storeID)
	if err != nil {
		return nil, &errors.AppError{
			Code: errors.ErrInternal, Message: "Failed to list devices",
			HTTPStatus: 500, Err: err,
		}
	}

	if devices == nil {
		devices = []model.Device{}
	}

	// Check online status via Redis heartbeat
	for i := range devices {
		lastHeartbeat, err := s.redis.Get(ctx, "device_heartbeat:"+devices[i].ID.String())
		if err == nil && lastHeartbeat != "" {
			// Consider online if heartbeat within last 5 minutes
			if t, err := time.Parse(time.RFC3339, lastHeartbeat); err == nil {
				devices[i].IsOnline = time.Since(t) < 5*time.Minute
			}
		}
	}

	// Build summary
	online := 0
	for _, d := range devices {
		if d.IsOnline {
			online++
		}
	}

	return &model.DeviceListResponse{
		Devices: devices,
		Summary: model.DeviceSummary{
			Total:   len(devices),
			Online:  online,
			Offline: len(devices) - online,
		},
	}, nil
}
