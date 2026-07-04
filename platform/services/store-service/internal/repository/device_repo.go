package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/zippyra/platform/services/store-service/internal/model"
)

// DeviceRepository handles device database operations.
type DeviceRepository struct {
	db dbPool
}

// NewDeviceRepository creates a new DeviceRepository.
func NewDeviceRepository(db dbPool) *DeviceRepository {
	return &DeviceRepository{db: db}
}

// ListByStoreID retrieves all devices for a given store.
func (r *DeviceRepository) ListByStoreID(ctx context.Context, storeID uuid.UUID) ([]model.Device, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	rows, err := r.db.Query(ctx,
		`SELECT id, store_id, device_type, serial_number, is_online,
		        last_heartbeat_at, firmware_version, created_at, updated_at
		 FROM devices WHERE store_id = $1 ORDER BY device_type, serial_number`, storeID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var devices []model.Device
	for rows.Next() {
		var d model.Device
		if err := rows.Scan(
			&d.ID, &d.StoreID, &d.DeviceType, &d.SerialNumber, &d.IsOnline,
			&d.LastHeartbeatAt, &d.FirmwareVersion, &d.CreatedAt, &d.UpdatedAt,
		); err != nil {
			return nil, err
		}
		devices = append(devices, d)
	}
	return devices, rows.Err()
}
