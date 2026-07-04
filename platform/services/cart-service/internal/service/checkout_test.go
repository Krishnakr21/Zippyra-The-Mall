package service

import (
	"context"
	"errors"
	"testing"

	"github.com/zippyra/platform/services/cart-service/internal/model"
)

func TestInitCheckoutFull(t *testing.T) {
	cartRepo := &mockCartRepo{
		items: map[string]model.CartItem{
			"8901234567890": {Barcode: "8901234567890", ProductID: "P001", Quantity: 1, UnitPrice: 100.0, TotalPrice: 118.0},
		},
		stock: map[string]int64{"P001": 9},
	}
	productRepo := &mockProductRepo{
		products: map[string]*model.Product{
			"8901234567890": {ID: "P001", Barcode: "8901234567890", Name: "Test Product", Price: 100.0, GSTRate: 18.0},
		},
		session: "store-1",
	}
	cs := NewCartService(cartRepo, productRepo)
	s := NewCheckoutService(cartRepo, productRepo, cs)

	t.Run("Checkout Success", func(t *testing.T) {
		res, err := s.InitCheckout(context.Background(), "user-1", "store-1")
		if err != nil {
			t.Fatal(err)
		}
		if res.CheckoutID != "uuid" {
			t.Error("expected checkout uuid")
		}
	})

	t.Run("Lock Error", func(t *testing.T) {
		cartRepo.lockErr = errors.New("lock error")
		_, err := s.InitCheckout(context.Background(), "user-1", "store-1")
		if err == nil {
			t.Error("expected error")
		}
		cartRepo.lockErr = nil
	})

	t.Run("GetItems Error", func(t *testing.T) {
		cartRepo.getCartItemsCallCount = 0
		cartRepo.getCartItemsErr = errors.New("items error")
		_, err := s.InitCheckout(context.Background(), "error-on-first", "store-1")
		if err == nil {
			t.Error("expected error")
		}
		cartRepo.getCartItemsErr = nil
	})

	t.Run("Product Lookup Error", func(t *testing.T) {
		productRepo.err = errors.New("product error")
		_, err := s.InitCheckout(context.Background(), "user-1", "store-1")
		if err == nil {
			t.Error("expected error")
		}
		productRepo.err = nil
	})

	t.Run("Empty Cart User", func(t *testing.T) {
		_, err := s.InitCheckout(context.Background(), "empty-user", "store-1")
		if err != ErrCartEmpty {
			t.Errorf("got %v, want %v", err, ErrCartEmpty)
		}
	})

	t.Run("Locked Cart", func(t *testing.T) {
		cartRepo.isLocked = true
		_, err := s.InitCheckout(context.Background(), "user-1", "store-1")
		if err != ErrCartLocked {
			t.Errorf("got %v, want %v", err, ErrCartLocked)
		}
		cartRepo.isLocked = false
	})

	t.Run("Price Changed", func(t *testing.T) {
		productRepo.products["8901234567890"].Price = 110.0
		_, err := s.InitCheckout(context.Background(), "user-1", "store-1")
		if err != ErrPriceChanged {
			t.Errorf("got %v, want %v", err, ErrPriceChanged)
		}
		productRepo.products["8901234567890"].Price = 100.0
	})

	t.Run("Get Stock Error", func(t *testing.T) {
		cartRepo.getStockErr = errors.New("stock error")
		_, err := s.InitCheckout(context.Background(), "user-1", "store-1")
		if err == nil {
			t.Error("expected error")
		}
		cartRepo.getStockErr = nil
	})

	t.Run("Insufficient Stock", func(t *testing.T) {
		cartRepo.stock["P001"] = -1
		_, err := s.InitCheckout(context.Background(), "user-1", "store-1")
		if err != ErrInsufficientStock {
			t.Errorf("got %v, want %v", err, ErrInsufficientStock)
		}
		cartRepo.stock["P001"] = 9
	})

	t.Run("GetItems Error with Lock Release Fail", func(t *testing.T) {
		cartRepo.getCartItemsCallCount = 0
		cartRepo.getCartItemsErr = errors.New("items error")
		cartRepo.err = errors.New("release lock error")
		_, err := s.InitCheckout(context.Background(), "error-on-first", "store-1")
		if err == nil {
			t.Error("expected error")
		}
		cartRepo.getCartItemsErr = nil
		cartRepo.err = nil
	})

	t.Run("Get Cart Error", func(t *testing.T) {
		cartRepo.getCartItemsCallCount = 0
		cartRepo.getCartItemsErr = errors.New("get cart error")
		// Succeeds on 1st call (InitCheckout step 2), fails on 2nd call (inside CartService.GetCart called at step 4)
		_, err := s.InitCheckout(context.Background(), "user-1", "store-1")
		if err == nil {
			t.Error("expected error")
		}
		cartRepo.getCartItemsErr = nil
	})

	t.Run("Snapshot Error", func(t *testing.T) {
		cartRepo.snapshotErr = errors.New("snapshot error")
		_, err := s.InitCheckout(context.Background(), "user-1", "store-1")
		if err == nil || err.Error() != "snapshot error" {
			t.Errorf("got %v, want snapshot error", err)
		}
		cartRepo.snapshotErr = nil
	})
}
