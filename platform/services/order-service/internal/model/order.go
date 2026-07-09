package model

import (
	"time"

	"github.com/google/uuid"
)

type Order struct {
	ID                 uuid.UUID  `json:"id"`
	OrderNumber        string     `json:"order_number"`
	UserID             uuid.UUID  `json:"user_id"`
	StoreID            uuid.UUID  `json:"store_id"`
	SessionID          uuid.UUID  `json:"session_id"`
	Status             string     `json:"status"`
	SupplyType         string     `json:"supply_type"`
	Subtotal           float64    `json:"subtotal"`
	GSTTotal           float64    `json:"gst_total"`
	CGSTTotal          float64    `json:"cgst_total"`
	SGSTTotal          float64    `json:"sgst_total"`
	IGSTTotal          float64    `json:"igst_total"`
	DiscountTotal      float64    `json:"discount_total"`
	TotalAmount        float64    `json:"total_amount"`
	PaymentMethod      string     `json:"payment_method"`
	PaymentID          *uuid.UUID `json:"payment_id"`
	IRN                *string    `json:"irn,omitempty"`
	IRNAckNo           *string    `json:"irn_ack_no,omitempty"`
	IRNAckDate         *time.Time `json:"irn_ack_date,omitempty"`
	ExitToken          *string    `json:"exit_token,omitempty"`
	ExitTokenExpiresAt *time.Time `json:"exit_token_expires_at,omitempty"`
	InvoiceURL         *string    `json:"invoice_url,omitempty"`
	ReturnWindowEndsAt *time.Time `json:"return_window_ends_at,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`

	Items []OrderItem `json:"items,omitempty"`
}

type OrderItem struct {
	ID          uuid.UUID `json:"id"`
	OrderID     uuid.UUID `json:"order_id"`
	ProductID   uuid.UUID `json:"product_id"`
	Barcode     string    `json:"barcode"`
	ProductName string    `json:"product_name"`
	Quantity    int       `json:"quantity"`
	UnitPrice   float64   `json:"unit_price"`
	GSTRate     float64   `json:"gst_rate"`
	CGSTAmount  float64   `json:"cgst_amount"`
	SGSTAmount  float64   `json:"sgst_amount"`
	IGSTAmount  float64   `json:"igst_amount"`
	GSTAmount   float64   `json:"gst_amount"`
	TotalPrice  float64   `json:"total_price"`
	HSNCode     string    `json:"hsn_code"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Product struct {
	ID            uuid.UUID `json:"id"`
	StoreID       uuid.UUID `json:"store_id"`
	Barcode       string    `json:"barcode"`
	Name          string    `json:"name"`
	HSNCode       string    `json:"hsn_code"`
	MRP           float64   `json:"mrp"`
	SellingPrice  float64   `json:"selling_price"`
	GSTRate       float64   `json:"gst_rate"`
	IsReturnable  bool      `json:"is_returnable"`
	StockQuantity int       `json:"stock_quantity"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type ExitToken struct {
	ID        uuid.UUID  `json:"id"`
	OrderID   uuid.UUID  `json:"order_id"`
	UserID    uuid.UUID  `json:"user_id"`
	StoreID   uuid.UUID  `json:"store_id"`
	TokenHash string     `json:"token_hash"`
	IsUsed    bool       `json:"is_used"`
	UsedAt    *time.Time `json:"used_at,omitempty"`
	ExpiresAt time.Time  `json:"expires_at"`
	CreatedAt time.Time  `json:"created_at"`
}

type PaymentConfirmedEvent struct {
	PaymentID     uuid.UUID     `json:"payment_id"`
	UserID        uuid.UUID     `json:"user_id"`
	StoreID       uuid.UUID     `json:"store_id"`
	SessionID     uuid.UUID     `json:"session_id"`
	Amount        float64       `json:"amount"`
	Currency      string        `json:"currency"`
	PaymentMethod string        `json:"payment_method"`
	CorrelationID string        `json:"correlation_id"`
	Items         []ConfirmItem `json:"items"`
}

type ConfirmItem struct {
	ProductID uuid.UUID `json:"product_id"`
	Quantity  int       `json:"quantity"`
	UnitPrice float64   `json:"unit_price"`
}

type OrderCompletedEvent struct {
	EventType     string  `json:"event_type"`
	OrderID       string  `json:"order_id"`
	OrderNumber   string  `json:"order_number"`
	UserID        string  `json:"user_id"`
	StoreID       string  `json:"store_id"`
	TotalAmount   float64 `json:"total_amount"`
	ExitToken     string  `json:"exit_token"`
	Timestamp     string  `json:"timestamp"`
	CorrelationID string  `json:"correlation_id"`
}

type OrderCreationFailedEvent struct {
	EventType string `json:"event_type"`
	PaymentID string `json:"payment_id"`
	UserID    string `json:"user_id"`
	Reason    string `json:"reason"`
	Timestamp string `json:"timestamp"`
}

type InventoryMovementEvent struct {
	EventType string `json:"event_type"`
	StoreID   string `json:"store_id"`
	ProductID string `json:"product_id"`
	Quantity  int    `json:"quantity"`
	Type      string `json:"type"` // e.g. "RETURN"
	Timestamp string `json:"timestamp"`
}

type LoyaltyPointsReversedEvent struct {
	EventType string  `json:"event_type"`
	OrderID   string  `json:"order_id"`
	UserID    string  `json:"user_id"`
	Amount    float64 `json:"amount"` // total points reversed based on refund amount
	Timestamp string  `json:"timestamp"`
}
