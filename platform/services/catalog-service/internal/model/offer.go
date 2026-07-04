package model

import (
	"time"

	"github.com/google/uuid"
)

type OfferRule struct {
	ID          uuid.UUID  `json:"id" db:"id"`
	StoreID     uuid.UUID  `json:"store_id" db:"store_id"`
	Name        string     `json:"name" db:"name"`
	Description *string    `json:"description,omitempty" db:"description"`
	Type        string     `json:"type" db:"type"` // discount, bogo, bundle
	Value       float64    `json:"value" db:"value"`
	MinAmount   *float64   `json:"min_amount,omitempty" db:"min_amount"`
	MaxDiscount *float64   `json:"max_discount,omitempty" db:"max_discount"`
	Category    *string    `json:"category,omitempty" db:"category"`
	ProductIDs  []uuid.UUID `json:"product_ids,omitempty" db:"product_ids"`
	Priority    int        `json:"priority" db:"priority"`
	IsActive    bool       `json:"is_active" db:"is_active"`
	ValidFrom   time.Time  `json:"valid_from" db:"valid_from"`
	ValidUntil  *time.Time `json:"valid_until,omitempty" db:"valid_until"`
	CreatedAt   time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at" db:"updated_at"`
}

type OfferResponse struct {
	Offers []OfferRule `json:"offers"`
}
