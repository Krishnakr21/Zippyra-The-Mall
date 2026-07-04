package model

import (
	"time"

	"github.com/google/uuid"
)

// LoginAttempt tracks every OTP send/verify attempt for audit logging.
type LoginAttempt struct {
	ID        uuid.UUID `json:"id"`
	Phone     string    `json:"phone"`
	IPAddress string    `json:"ip_address"`
	UserAgent string    `json:"user_agent"`
	Status    string    `json:"status"` // SENT, SUCCESS, FAILED, BLOCKED
	CreatedAt time.Time `json:"created_at"`
}
