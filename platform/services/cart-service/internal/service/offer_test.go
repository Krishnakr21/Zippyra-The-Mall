package service

import (
	"testing"

	"github.com/zippyra/platform/services/cart-service/internal/model"
)

func TestApplyOffersFull(t *testing.T) {
	cart := &model.Cart{
		Subtotal:  1000.0,
		ItemCount: 2,
		Items: []model.CartItem{
			{ProductID: "P1", Barcode: "B1", UnitPrice: 500.0, Quantity: 2},
		},
	}

	tests := []struct {
		name         string
		rules        []model.OfferRule
		wantOfferID  string
		wantDiscount float64
	}{
		{
			name: "PERCENTAGE",
			rules: []model.OfferRule{
				{ID: "O1", Type: "PERCENTAGE", Value: 10, IsActive: true},
			},
			wantOfferID:  "O1",
			wantDiscount: 100,
		},
		{
			name: "FLAT",
			rules: []model.OfferRule{
				{ID: "O2", Type: "FLAT", Value: 50, IsActive: true},
			},
			wantOfferID:  "O2",
			wantDiscount: 50,
		},
		{
			name: "BOGO success",
			rules: []model.OfferRule{
				{ID: "O3", Type: "BOGO", ProductID: "P1", BuyQuantity: 1, FreeQuantity: 1, IsActive: true},
			},
			wantOfferID:  "O3",
			wantDiscount: 500,
		},
		{
			name: "BOGO non-matching product",
			rules: []model.OfferRule{
				{ID: "O3A", Type: "BOGO", ProductID: "P_OTHER", BuyQuantity: 1, FreeQuantity: 1, IsActive: true},
			},
			wantOfferID:  "",
			wantDiscount: 0,
		},
		{
			name: "BOGO zero guard",
			rules: []model.OfferRule{
				{ID: "O3B", Type: "BOGO", BuyQuantity: 0, FreeQuantity: 0, IsActive: true},
			},
			wantOfferID:  "",
			wantDiscount: 0,
		},
		{
			name: "MIN_QUANTITY success",
			rules: []model.OfferRule{
				{ID: "O4", Type: "MIN_QUANTITY", MinQuantity: 2, Value: 200, IsActive: true},
			},
			wantOfferID:  "O4",
			wantDiscount: 200,
		},
		{
			name: "MIN_QUANTITY not met",
			rules: []model.OfferRule{
				{ID: "O5", Type: "MIN_QUANTITY", MinQuantity: 3, Value: 200, IsActive: true},
			},
			wantOfferID:  "",
			wantDiscount: 0,
		},
		{
			name: "Max Discount",
			rules: []model.OfferRule{
				{ID: "O6", Type: "PERCENTAGE", Value: 50, MaxDiscount: 100, IsActive: true},
			},
			wantOfferID:  "O6",
			wantDiscount: 100,
		},
		{
			name: "Inactive rule",
			rules: []model.OfferRule{
				{ID: "O7", Type: "PERCENTAGE", Value: 50, IsActive: false},
			},
			wantOfferID:  "",
			wantDiscount: 0,
		},
		{
			name: "Unknown Type",
			rules: []model.OfferRule{
				{ID: "O8", Type: "UNKNOWN", IsActive: true},
			},
			wantOfferID:  "",
			wantDiscount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ApplyOffers(cart, tt.rules)
			if tt.wantOfferID == "" {
				if got != nil {
					t.Errorf("got %v, want nil", got.OfferID)
				}
			} else {
				if got == nil {
					t.Fatalf("got nil, want %v", tt.wantOfferID)
				}
				if got.OfferID != tt.wantOfferID {
					t.Errorf("got %v, want %v", got.OfferID, tt.wantOfferID)
				}
				if got.DiscountValue != tt.wantDiscount {
					t.Errorf("got %v, want %v", got.DiscountValue, tt.wantDiscount)
				}
			}
		})
	}
}
