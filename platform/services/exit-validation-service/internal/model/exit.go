package model

import "time"

type ExitValidationRequest struct {
	ExitToken string `json:"exit_token"`
	StoreID   string `json:"store_id"`
	GateID    string `json:"gate_id"`
}

type ExitValidationResponse struct {
	Status      string `json:"status"`
	GateCommand string `json:"gate_command"`
	OrderID     string `json:"order_id"`
	Message     string `json:"message"`
}

type StaffOverrideRequest struct {
	StoreID string `json:"store_id"`
	GateID  string `json:"gate_id"`
	UserID  string `json:"user_id"`
	Reason  string `json:"reason"`
}

type ExitTokenStatusResponse struct {
	OrderID   string    `json:"order_id"`
	IsUsed    bool      `json:"is_used"`
	ExpiresAt time.Time `json:"expires_at"`
	IsExpired bool      `json:"is_expired"`
}
