package model

import (
	"time"
)

type Cart struct {
	UserID        string        `json:"user_id"`
	StoreID       string        `json:"store_id"`
	Items         []CartItem    `json:"items"`
	Subtotal      float64       `json:"subtotal"`
	GSTTotal      float64       `json:"gst_total"`
	CGSTTotal     float64       `json:"cgst_total"`
	SGSTTotal     float64       `json:"sgst_total"`
	IGSTTotal     float64       `json:"igst_total"`
	DiscountTotal float64       `json:"discount_total"`
	TotalAmount   float64       `json:"total_amount"`
	ItemCount     int           `json:"item_count"`
	OfferApplied  *AppliedOffer `json:"offer_applied"`
}

type CartItem struct {
	Barcode     string  `json:"barcode"`
	ProductID   string  `json:"product_id"`
	ProductName string  `json:"product_name"`
	Quantity    int     `json:"quantity"`
	UnitPrice   float64 `json:"unit_price"`
	GSTAmount   float64 `json:"gst_amount"`
	TotalPrice  float64 `json:"total_price"` // Quantity * UnitPrice + GSTAmount? Or (UnitPrice + GSTPerUnit) * Quantity?
	// Usually UnitPrice includes tax or tax is added. The requirement says:
	// "unit_price": 90.00, "gst_amount": 16.20, "total_price": 106.20
	// So TotalPrice = (UnitPrice * Quantity) + GSTAmount
}

type Product struct {
	ID          string  `json:"id"`
	Barcode     string  `json:"barcode"`
	Name        string  `json:"name"`
	Price       float64 `json:"price"`
	HSNCode     string  `json:"hsn_code"`
	GSTRate     float64 `json:"gst_rate"`
	Stock       int     `json:"stock"`
	StoreID     string  `json:"store_id"`
}

type OfferRule struct {
	ID            string    `json:"id"`
	StoreID       string    `json:"store_id"`
	Type          string    `json:"type"` // PERCENTAGE, FLAT, BOGO, MIN_QUANTITY
	Value         float64   `json:"value"`
	ProductID     string    `json:"product_id,omitempty"`
	MinQuantity   int       `json:"min_quantity,omitempty"`
	FreeQuantity  int       `json:"free_quantity,omitempty"`
	BuyQuantity   int       `json:"buy_quantity,omitempty"`
	MaxDiscount   float64   `json:"max_discount,omitempty"`
	ExpiryDate    time.Time `json:"expiry_date"`
	UsageLimit    int       `json:"usage_limit"`
	TimesUsed     int       `json:"times_used"`
	CouponCode    string    `json:"coupon_code,omitempty"`
	IsActive      bool      `json:"is_active"`
}

type AppliedOffer struct {
	OfferID      string  `json:"offer_id"`
	Description  string  `json:"description"`
	DiscountValue float64 `json:"discount_value"`
}
