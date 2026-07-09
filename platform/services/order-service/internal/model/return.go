package model

import (
	"time"

	"github.com/google/uuid"
)

const (
	ReturnStatusPendingApproval = "PENDING_STAFF_APPROVAL"
	ReturnStatusAccepted        = "ACCEPTED"
)

type ReturnRequest struct {
	ID              uuid.UUID `json:"id"`
	OrderID         uuid.UUID `json:"order_id"`
	UserID          uuid.UUID `json:"user_id"`
	StoreID         uuid.UUID `json:"store_id"`
	Status          string    `json:"status"`
	Reason          string    `json:"reason"`
	Items           []string  `json:"items"` // JSONB array of order_item UUIDs returned
	RefundInitiated bool      `json:"refund_initiated"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}
