package model

import (
	"time"

	"github.com/google/uuid"
)

// Store represents a retail store in the system.
type Store struct {
	ID             uuid.UUID  `json:"id"`
	ChainID        uuid.UUID  `json:"chain_id"`
	Name           string     `json:"name"`
	Address        string     `json:"address"`
	City           string     `json:"city"`
	State          string     `json:"state"`
	Pincode        string     `json:"pincode"`
	Latitude       *float64   `json:"latitude,omitempty"`
	Longitude      *float64   `json:"longitude,omitempty"`
	Capacity       int        `json:"capacity"`
	CatalogVersion int        `json:"catalog_version"`
	QROnlyMode     bool       `json:"qr_only_mode"`
	IsActive       bool       `json:"is_active"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	DeletedAt      *time.Time `json:"deleted_at,omitempty"`
}

// StoreQRToken represents a QR token associated with a store entrance.
type StoreQRToken struct {
	ID        uuid.UUID `json:"id"`
	StoreID   uuid.UUID `json:"store_id"`
	Token     string    `json:"token"`
	TokenType string    `json:"token_type"` // ENTRANCE, EXIT, etc.
	UsedCount int       `json:"used_count"`
	IsActive  bool      `json:"is_active"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// StoreHours represents operating hours for a store.
type StoreHours struct {
	StoreID   uuid.UUID `json:"store_id"`
	DayOfWeek int       `json:"day_of_week"` // 0=Sunday, 6=Saturday
	OpensAt   string    `json:"opens_at"`     // HH:MM format
	ClosesAt  string    `json:"closes_at"`    // HH:MM format
}

// BindRequest is the request body for POST /v1/store/bind.
type BindRequest struct {
	QRToken  string `json:"qr_token" validate:"required"`
	DeviceID string `json:"device_id" validate:"required,uuid"`
}

// BindResponse is returned after a successful store bind.
type BindResponse struct {
	StoreID          uuid.UUID `json:"store_id"`
	StoreName        string    `json:"store_name"`
	CatalogVersion   int       `json:"catalog_version"`
	QROnlyMode       bool      `json:"qr_only_mode"`
	SessionExpiresAt time.Time `json:"session_expires_at"`
}

// StoreInfoResponse is the public store info response.
type StoreInfoResponse struct {
	ID             uuid.UUID `json:"id"`
	Name           string    `json:"name"`
	Address        string    `json:"address"`
	City           string    `json:"city"`
	State          string    `json:"state"`
	Pincode        string    `json:"pincode"`
	Latitude       *float64  `json:"latitude,omitempty"`
	Longitude      *float64  `json:"longitude,omitempty"`
	Capacity       int       `json:"capacity"`
	CatalogVersion int       `json:"catalog_version"`
	QROnlyMode     bool      `json:"qr_only_mode"`
	IsActive       bool      `json:"is_active"`
}

// NearbyStore includes distance information for nearby store queries.
type NearbyStore struct {
	StoreInfoResponse
	DistanceKM       float64 `json:"distance_km"`
	CurrentOccupancy int     `json:"current_occupancy"`
	OccupancyPct     int     `json:"occupancy_pct"`
	IsOpen           bool    `json:"is_open"`
}

// OccupancyResponse is the response for real-time occupancy queries.
type OccupancyResponse struct {
	StoreID          uuid.UUID `json:"store_id"`
	CurrentOccupancy int       `json:"current_occupancy"`
	Capacity         int       `json:"capacity"`
	OccupancyPct     int       `json:"occupancy_pct"`
	Status           string    `json:"status"` // normal, busy, near_capacity, at_capacity
}

// CapacityUpdateRequest is the request body for PUT /v1/store/{id}/capacity.
type CapacityUpdateRequest struct {
	Action string `json:"action" validate:"required,oneof=increment decrement"`
}

// CapacityUpdateResponse is returned after a capacity update.
type CapacityUpdateResponse struct {
	CurrentOccupancy int `json:"current_occupancy"`
}

// HoursResponse is the response for store hours queries.
type HoursResponse struct {
	IsOpen     bool    `json:"is_open"`
	OpensAt    string  `json:"opens_at"`
	ClosesAt   string  `json:"closes_at"`
	Timezone   string  `json:"timezone"`
	NextOpenAt *string `json:"next_open_at"`
}

// ExitResponse is the response for store exit.
type ExitResponse struct {
	Message string `json:"message"`
}
