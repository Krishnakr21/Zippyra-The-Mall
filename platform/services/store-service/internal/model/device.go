package model

import (
	"time"

	"github.com/google/uuid"
)

// Device represents a registered device in a store.
type Device struct {
	ID              uuid.UUID `json:"id"`
	StoreID         uuid.UUID `json:"store_id"`
	DeviceType      string    `json:"device_type"` // GATE_SENSOR, CAMERA, POS, etc.
	SerialNumber    string    `json:"serial_number"`
	IsOnline        bool      `json:"is_online"`
	LastHeartbeatAt time.Time `json:"last_heartbeat_at"`
	FirmwareVersion string    `json:"firmware_version"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// DeviceSummary contains aggregate device stats for a store.
type DeviceSummary struct {
	Total   int `json:"total"`
	Online  int `json:"online"`
	Offline int `json:"offline"`
}

// DeviceListResponse is the response for GET /v1/store/{id}/devices.
type DeviceListResponse struct {
	Devices []Device      `json:"devices"`
	Summary DeviceSummary `json:"summary"`
}
