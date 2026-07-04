package service

import (
	"context"
	"errors"
	"testing"

	"github.com/zippyra/platform/services/cart-service/internal/model"
)

func TestRemoveAndClear(t *testing.T) {
	cartRepo := &mockCartRepo{
		items: map[string]model.CartItem{
			"8901234567890": {Barcode: "8901234567890", ProductID: "P1", Quantity: 2},
		},
		stock: map[string]int64{"P1": 10},
	}
	productRepo := &mockProductRepo{
		products: map[string]*model.Product{
			"8901234567890": {Barcode: "8901234567890", ID: "P1", Price: 100.0, GSTRate: 18.0},
		},
	}
	s := NewCartService(cartRepo, productRepo)

	t.Run("Remove Item Success", func(t *testing.T) {
		_, err := s.RemoveItem(context.Background(), "user-1", "store-1", "8901234567890")
		if err != nil {
			t.Fatal(err)
		}
		if cartRepo.stock["P1"] != 12 {
			t.Errorf("expected stock 12, got %d", cartRepo.stock["P1"])
		}
	})

	t.Run("Clear Cart Success", func(t *testing.T) {
		cartRepo.items = map[string]model.CartItem{
			"8901234567890": {Barcode: "8901234567890", ProductID: "P1", Quantity: 5},
		}
		cartRepo.stock["P1"] = 10
		err := s.ClearCart(context.Background(), "user-1", "store-1")
		if err != nil {
			t.Fatal(err)
		}
		if cartRepo.stock["P1"] != 15 {
			t.Errorf("expected stock 15, got %d", cartRepo.stock["P1"])
		}
	})

	t.Run("Clear Cart Error", func(t *testing.T) {
		cartRepo.err = errors.New("clear error")
		err := s.ClearCart(context.Background(), "user-1", "store-1")
		if err == nil {
			t.Error("expected error")
		}
		cartRepo.err = nil
	})
}
