package model

import (
	"time"

	"github.com/google/uuid"
)

// User represents a Zippyra customer.
type User struct {
	ID           uuid.UUID  `json:"id"`
	Phone        string     `json:"phone"`
	Email        *string    `json:"email,omitempty"`
	FullName     *string    `json:"full_name,omitempty"`
	IsActive     bool       `json:"is_active"`
	IsVerified   bool       `json:"is_verified"`
	AppVersion   *string    `json:"app_version,omitempty"`
	DeviceToken  *string    `json:"device_token,omitempty"`
	ReferralCode *string    `json:"referral_code,omitempty"`
	LastLoginAt  *time.Time `json:"last_login_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// AuthSession represents an active authentication session.
type AuthSession struct {
	ID           uuid.UUID  `json:"id"`
	UserID       uuid.UUID  `json:"user_id"`
	DeviceID     string     `json:"device_id"`
	DeviceModel  string     `json:"device_model"`
	IPAddress    string     `json:"ip_address"`
	UserAgent    string     `json:"user_agent"`
	LastActiveAt time.Time  `json:"last_active_at"`
	LoggedOutAt  *time.Time `json:"logged_out_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

// AppVersion represents a row from the app_versions table.
type AppVersion struct {
	ID                  uuid.UUID `json:"id"`
	Platform            string    `json:"platform"`
	Version             string    `json:"version"`
	MinSupportedVersion string    `json:"min_supported_version"`
	IsForceUpdate       bool      `json:"is_force_update"`
	ReleaseNotes        *string   `json:"release_notes,omitempty"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}
