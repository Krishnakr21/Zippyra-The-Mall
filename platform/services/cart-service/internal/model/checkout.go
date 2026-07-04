package model

import (
	"time"
)

type CheckoutInitRequest struct {
	StoreID string `json:"store_id"`
}

type CheckoutResponse struct {
	CheckoutID    string     `json:"checkout_id"`
	StoreID       string     `json:"store_id"`
	Items         []CartItem `json:"items"`
	Subtotal      float64    `json:"subtotal"`
	GSTTotal      float64    `json:"gst_total"`
	DiscountTotal float64    `json:"discount_total"`
	TotalAmount   float64    `json:"total_amount"`
	ExpiresAt     time.Time  `json:"expires_at"`
}

type CouponRequest struct {
	StoreID    string `json:"store_id"`
	CouponCode string `json:"coupon_code"`
}

type CashPaymentRequest struct {
	CartID             string  `json:"cart_id"`
	UserID             string  `json:"user_id"`
	CashCollectedPaise int64   `json:"cash_collected_paise"`
}

type OrderResponse struct {
	OrderID       string    `json:"order_id"`
	ExitToken     string    `json:"exit_token"`
	Status        string    `json:"status"`
	PaymentMethod string    `json:"payment_method"`
	CreatedAt     time.Time `json:"created_at"`
}
