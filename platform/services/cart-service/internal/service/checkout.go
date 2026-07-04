package service

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/zippyra/platform/services/cart-service/internal/model"
	"github.com/zippyra/platform/services/cart-service/internal/repository"
)

var (
	ErrPriceChanged = errors.New("price changed during checkout")
	ErrCartLocked   = errors.New("cart is already being checked out")
)

type CheckoutService interface {
	InitCheckout(ctx context.Context, userID, storeID string) (*model.CheckoutResponse, error)
}

type checkoutService struct {
	cartRepo    repository.CartRepository
	productRepo repository.ProductRepository
	cartService CartService
}

func NewCheckoutService(cartRepo repository.CartRepository, productRepo repository.ProductRepository, cartService CartService) CheckoutService {
	return &checkoutService{
		cartRepo:    cartRepo,
		productRepo: productRepo,
		cartService: cartService,
	}
}

func (s *checkoutService) InitCheckout(ctx context.Context, userID, storeID string) (*model.CheckoutResponse, error) {
	// 1. Get raw items from repo
	items, err := s.cartRepo.GetCartItems(ctx, userID, storeID)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, ErrCartEmpty
	}

	// 2. Checkout lock (5 min)
	locked, err := s.cartRepo.AcquireCheckoutLock(ctx, userID, 5*time.Minute)
	if err != nil {
		return nil, err
	}
	if !locked {
		return nil, ErrCartLocked
	}

	// 3. Re-validate prices and stock
	for i := range items {
		item := &items[i]
		product, err := s.productRepo.GetProductByBarcode(ctx, storeID, item.Barcode)
		if err != nil || product == nil {
			return nil, ErrBarcodeNotFound
		}

		// Price re-validation (deviation > 0.01)
		if math.Abs(item.UnitPrice-product.Price) > 0.01 {
			return nil, ErrPriceChanged
		}

		// Stock re-validation
		stock, err := s.cartRepo.GetStock(ctx, storeID, product.ID)
		if err != nil {
			return nil, err
		}
		if stock < 0 { 
			return nil, ErrInsufficientStock
		}
	}

	// 4. Get full cart for final totals
	cart, err := s.cartService.GetCart(ctx, userID, storeID)
	if err != nil {
		return nil, err
	}

	// 5. Snapshot to DB
	checkoutID, err := s.cartRepo.SnapshotCart(ctx, cart)
	if err != nil {
		return nil, err
	}

	return &model.CheckoutResponse{
		CheckoutID:    checkoutID,
		StoreID:       storeID,
		Items:         cart.Items,
		Subtotal:      cart.Subtotal,
		GSTTotal:      cart.GSTTotal,
		DiscountTotal: cart.DiscountTotal,
		TotalAmount:   cart.TotalAmount,
		ExpiresAt:     time.Now().Add(5 * time.Minute),
	}, nil
}
