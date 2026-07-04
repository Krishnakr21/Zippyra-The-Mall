package model

import "time"

// AuthEvent is an immutable audit trail entry.
type AuthEvent struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	EventType string    `json:"event_type"`
	DeviceID  *string   `json:"device_id,omitempty"`
	IP        *string   `json:"ip,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}
