package model

import (
	"time"

	"github.com/google/uuid"
)

type PaymentStatus string

const (
	PaymentStatusPending    PaymentStatus = "PENDING"
	PaymentStatusAuthorized PaymentStatus = "AUTHORIZED"
	PaymentStatusSuccess    PaymentStatus = "SUCCESS"
	PaymentStatusFailed     PaymentStatus = "FAILED"
	PaymentStatusRefunded   PaymentStatus = "REFUNDED"
)

type PaymentMethod string

const (
	PaymentMethodUPI  PaymentMethod = "UPI"
	PaymentMethodCard PaymentMethod = "CARD"
)

type Payment struct {
	ID                uuid.UUID     `json:"payment_id"`
	OrderID           uuid.UUID     `json:"order_id"`
	UserID            uuid.UUID     `json:"user_id"`
	StoreID           uuid.UUID     `json:"store_id"`
	AmountPaise       int64         `json:"amount_paise"`
	Currency          string        `json:"currency"`
	Status            PaymentStatus `json:"status"`
	PaymentMethod     *PaymentMethod `json:"payment_method"`
	Gateway           string        `json:"gateway"`
	GatewayOrderID    *string       `json:"gateway_order_id,omitempty"`
	GatewayPaymentID  *string       `json:"gateway_payment_id,omitempty"`
	UPITransactionID  *string       `json:"upi_transaction_id"`
	IdempotencyKey    string        `json:"idempotency_key"`
	FailureReason     *string       `json:"failure_reason"`
	RefundID          *string        `json:"refund_id,omitempty"`
	RefundAmountPaise int64         `json:"refund_amount_paise,omitempty"`
	WebhookReceivedAt *time.Time    `json:"webhook_received_at,omitempty"`
	CreatedAt         time.Time     `json:"created_at"`
	UpdatedAt         time.Time     `json:"updated_at"`
}

type GatewayOrderResponse struct {
	GatewayOrderID string `json:"gateway_order_id"`
}

type GatewayRefundResponse struct {
	GatewayRefundID string `json:"gateway_refund_id"`
}

type WebhookEvent struct {
	ID           uuid.UUID `json:"id"`
	EventID      string    `json:"event_id"`
	Gateway      string    `json:"gateway"`
	EventType    string    `json:"event_type"`
	Payload      []byte    `json:"payload"`
	Processed    bool      `json:"processed"`
	HMACVerified bool      `json:"hmac_verified"`
	ErrorMessage string    `json:"error_message,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

type OutboxMessage struct {
	ID          string    `json:"id"`
	Topic       string    `json:"topic"`
	Payload     []byte    `json:"payload"`
	PublishedAt *time.Time `json:"published_at,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

type RazorpayWebhookEvent struct {
	EventID string `json:"id"`
	Event   string `json:"event"`
	Payload struct {
		Payment struct {
			Entity struct {
				ID               string `json:"id"`
				Amount           int64  `json:"amount"`
				Status           string `json:"status"`
				ErrorDescription string `json:"error_description"`
				Notes            struct {
					PaymentID string `json:"payment_id"`
				} `json:"notes"`
			} `json:"entity"`
		} `json:"payment"`
	} `json:"payload"`
}

type CashfreeWebhookEvent struct {
	Data struct {
		Order struct {
			OrderID string `json:"order_id"`
		} `json:"order"`
		Payment struct {
			CfPaymentID string  `json:"cf_payment_id"`
			PaymentStatus string `json:"payment_status"`
		} `json:"payment"`
	} `json:"data"`
	EventType string `json:"event_type"`
}