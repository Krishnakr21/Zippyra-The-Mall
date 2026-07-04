package service

import (
	"context"
	"errors"
	"testing"

	"github.com/zippyra/platform/services/cart-service/internal/model"
)

func TestApplyCouponFull(t *testing.T) {
	cartRepo := &mockCartRepo{
		items: make(map[string]model.CartItem),
		stock: make(map[string]int64),
	}
	productRepo := &mockProductRepo{
		session: "store-1",
	}
	cs := NewCartService(cartRepo, productRepo)
	s := NewCouponService(productRepo, cs)

	t.Run("Coupon Success", func(t *testing.T) {
		productRepo.products = map[string]*model.Product{
			"8901234567890": {Barcode: "8901234567890", ID: "P1", Price: 100.0, GSTRate: 18.0},
		}
		cartRepo.items = map[string]model.CartItem{
			"8901234567890": {Barcode: "8901234567890", ProductID: "P1", Quantity: 1, UnitPrice: 100.0},
		}
		productRepo.offerRules = []model.OfferRule{
			{
				ID:         "COUPON1",
				CouponCode: "SAVE10",
				Type:       "PERCENTAGE",
				Value:      10.0,
				IsActive:   true,
			},
		}

		cart, err := s.ApplyCoupon(context.Background(), "user-1", "store-1", "SAVE10")
		if err != nil {
			t.Fatalf("ApplyCoupon() unexpected error: %v", err)
		}
		if cart.OfferApplied == nil || cart.OfferApplied.OfferID != "COUPON1" {
			t.Errorf("offer not applied correctly")
		}
	})

	t.Run("Coupon Invalid", func(t *testing.T) {
		productRepo.offerRules = nil
		_, err := s.ApplyCoupon(context.Background(), "user-1", "store-1", "INVALID")
		if err != ErrCouponInvalid {
			t.Errorf("got %v, want %v", err, ErrCouponInvalid)
		}
	})

	t.Run("Coupon Limit Reached", func(t *testing.T) {
		productRepo.offerRules = []model.OfferRule{
			{ID: "COUPON2", CouponCode: "LIMIT1", IsActive: true, UsageLimit: 10, TimesUsed: 10},
		}
		_, err := s.ApplyCoupon(context.Background(), "user-1", "store-1", "LIMIT1")
		if err != ErrCouponLimit {
			t.Errorf("got %v, want %v", err, ErrCouponLimit)
		}
	})

	t.Run("Product Repo Error", func(t *testing.T) {
		productRepo.err = errors.New("error")
		_, err := s.ApplyCoupon(context.Background(), "user-1", "store-1", "SAVE10")
		if err == nil {
			t.Error("expected error")
		}
		productRepo.err = nil
	})

	t.Run("Get Cart Error", func(t *testing.T) {
		productRepo.offerRules = []model.OfferRule{{CouponCode: "SAVE10", IsActive: true}}
		cartRepo.getCartItemsCallCount = 0
		cartRepo.getCartItemsErr = errors.New("get cart error")
		_, err := s.ApplyCoupon(context.Background(), "error-on-first", "store-1", "SAVE10")
		if err == nil {
			t.Error("expected error")
		}
		cartRepo.getCartItemsErr = nil
	})
}
