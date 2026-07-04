package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/zippyra/platform/services/cart-service/internal/model"
)

type mockCartRepo struct {
	items                 map[string]model.CartItem
	stock                 map[string]int64
	err                   error
	getCartItemsErr       error
	getCartItemsCallCount int
	getStockErr           error
	snapshotErr           error
	addToCartErr          error
	decrementStockErr     error
	isLocked              bool
	lockErr               error
}

func (m *mockCartRepo) AddToCart(ctx context.Context, userID, storeID string, item model.CartItem) error {
	if m.addToCartErr != nil {
		return m.addToCartErr
	}
	if m.err != nil {
		return m.err
	}
	m.items[item.Barcode] = item
	return nil
}
func (m *mockCartRepo) GetCartItems(ctx context.Context, userID, storeID string) ([]model.CartItem, error) {
	m.getCartItemsCallCount++
	if userID == "empty-user" {
		return nil, nil
	}
	if m.getCartItemsCallCount == 2 && m.getCartItemsErr != nil {
		return nil, m.getCartItemsErr
	}
	if m.getCartItemsCallCount == 1 && m.getCartItemsErr != nil && userID == "error-on-first" {
		return nil, m.getCartItemsErr
	}
	if m.err != nil {
		return nil, m.err
	}
	var items []model.CartItem
	for _, it := range m.items {
		items = append(items, it)
	}
	return items, nil
}
func (m *mockCartRepo) RemoveItem(ctx context.Context, userID, storeID, barcode string) (*model.CartItem, error) {
	if m.err != nil {
		return nil, m.err
	}
	it, ok := m.items[barcode]
	if ok {
		delete(m.items, barcode)
		return &it, nil
	}
	return nil, nil
}
func (m *mockCartRepo) ClearCart(ctx context.Context, userID, storeID string) ([]model.CartItem, error) {
	if m.err != nil {
		return nil, m.err
	}
	var items []model.CartItem
	for _, it := range m.items {
		items = append(items, it)
	}
	m.items = make(map[string]model.CartItem)
	return items, nil
}
func (m *mockCartRepo) AcquireCheckoutLock(ctx context.Context, userID string, ttl time.Duration) (bool, error) {
	if m.lockErr != nil {
		return false, m.lockErr
	}
	if m.isLocked {
		return false, nil
	}
	if m.err != nil {
		return false, m.err
	}
	return true, nil
}
func (m *mockCartRepo) ReleaseCheckoutLock(ctx context.Context, userID string) error {
	return m.err
}
func (m *mockCartRepo) DecrementStock(ctx context.Context, storeID, productID string, quantity int) (int64, error) {
	if m.decrementStockErr != nil {
		return 0, m.decrementStockErr
	}
	if m.err != nil {
		return 0, m.err
	}
	m.stock[productID] -= int64(quantity)
	return m.stock[productID], nil
}
func (m *mockCartRepo) IncrementStock(ctx context.Context, storeID, productID string, quantity int) (int64, error) {
	if m.err != nil {
		return 0, m.err
	}
	m.stock[productID] += int64(quantity)
	return m.stock[productID], nil
}
func (m *mockCartRepo) GetStock(ctx context.Context, storeID, productID string) (int64, error) {
	if m.getStockErr != nil {
		return 0, m.getStockErr
	}
	if m.err != nil {
		return 0, m.err
	}
	return m.stock[productID], nil
}
func (m *mockCartRepo) SnapshotCart(ctx context.Context, cart *model.Cart) (string, error) {
	if m.snapshotErr != nil {
		return "", m.snapshotErr
	}
	if m.err != nil {
		return "", m.err
	}
	return "uuid", nil
}

type mockProductRepo struct {
	products          map[string]*model.Product
	session           string
	err               error
	offerRules        []model.OfferRule
	getOfferRulesErr  error
	productLookupErr  map[string]error
}

func (m *mockProductRepo) GetProductByBarcode(ctx context.Context, storeID, barcode string) (*model.Product, error) {
	if err, ok := m.productLookupErr[barcode]; ok {
		return nil, err
	}
	if m.err != nil {
		return nil, m.err
	}
	return m.products[barcode], nil
}
func (m *mockProductRepo) GetStoreSession(ctx context.Context, userID string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.session, nil
}
func (m *mockProductRepo) GetOfferRules(ctx context.Context, storeID string) ([]model.OfferRule, error) {
	if m.getOfferRulesErr != nil {
		return nil, m.getOfferRulesErr
	}
	if m.err != nil {
		return nil, m.err
	}
	return m.offerRules, nil
}

func TestScanItem(t *testing.T) {
	cartRepo := &mockCartRepo{
		items: make(map[string]model.CartItem),
		stock: map[string]int64{"P001": 10},
	}
	productRepo := &mockProductRepo{
		products: map[string]*model.Product{
			"8901234567890": {ID: "P001", Barcode: "8901234567890", Name: "Test Product", Price: 100.0, GSTRate: 18.0},
		},
		session: "store-1",
	}
	s := NewCartService(cartRepo, productRepo)

	t.Run("Scan Success", func(t *testing.T) {
		item, cart, err := s.ScanItem(context.Background(), "user-1", "store-1", "8901234567890", 1)
		if err != nil {
			t.Fatalf("ScanItem() unexpected error: %v", err)
		}
		if item.Barcode != "8901234567890" {
			t.Errorf("item barcode = %v, want 8901234567890", item.Barcode)
		}
		if cartRepo.stock["P001"] != 9 {
			t.Errorf("stock = %v, want 9", cartRepo.stock["P001"])
		}
		if cart.ItemCount != 1 {
			t.Errorf("cart item count = %v, want 1", cart.ItemCount)
		}
	})

	t.Run("Scan Invalid Barcode", func(t *testing.T) {
		_, _, err := s.ScanItem(context.Background(), "user-1", "store-1", "invalid", 1)
		if err != ErrBarcodeInvalid {
			t.Errorf("got %v, want %v", err, ErrBarcodeInvalid)
		}
	})

	t.Run("Scan Without Store Session", func(t *testing.T) {
		_, _, err := s.ScanItem(context.Background(), "user-1", "different-store", "8901234567890", 1)
		if err != ErrStoreNotFound {
			t.Errorf("got %v, want %v", err, ErrStoreNotFound)
		}
	})

	t.Run("Scan Product Repo Error", func(t *testing.T) {
		productRepo.productLookupErr = map[string]error{"8901234567890": errors.New("repo error")}
		_, _, err := s.ScanItem(context.Background(), "user-1", "store-1", "8901234567890", 1)
		if err == nil {
			t.Error("expected error")
		}
		productRepo.productLookupErr = nil
	})

	t.Run("Scan Barcode Not Found", func(t *testing.T) {
		productRepo.products = make(map[string]*model.Product)
		_, _, err := s.ScanItem(context.Background(), "user-1", "store-1", "8901234567890", 1)
		if err != ErrBarcodeNotFound {
			t.Errorf("got %v, want %v", err, ErrBarcodeNotFound)
		}
		productRepo.products = map[string]*model.Product{
			"8901234567890": {ID: "P001", Barcode: "8901234567890", Name: "Test Product", Price: 100.0, GSTRate: 18.0},
		}
	})

	t.Run("Scan Stock Check Error", func(t *testing.T) {
		cartRepo.getStockErr = errors.New("stock error")
		_, _, err := s.ScanItem(context.Background(), "user-1", "store-1", "8901234567890", 1)
		if err == nil {
			t.Error("expected error")
		}
		cartRepo.getStockErr = nil
	})

	t.Run("Scan Out of Stock", func(t *testing.T) {
		cartRepo.stock["P001"] = 0
		_, _, err := s.ScanItem(context.Background(), "user-1", "store-1", "8901234567890", 1)
		if err != ErrInsufficientStock {
			t.Errorf("got %v, want %v", err, ErrInsufficientStock)
		}
	})

	t.Run("Scan Decrement Error", func(t *testing.T) {
		cartRepo.stock["P001"] = 10
		cartRepo.decrementStockErr = errors.New("decrement error")
		_, _, err := s.ScanItem(context.Background(), "user-1", "store-1", "8901234567890", 1)
		if err == nil {
			t.Error("expected error")
		}
		cartRepo.decrementStockErr = nil
	})

	t.Run("Scan AddToCart Error", func(t *testing.T) {
		cartRepo.addToCartErr = errors.New("add error")
		_, _, err := s.ScanItem(context.Background(), "user-1", "store-1", "8901234567890", 1)
		if err == nil {
			t.Error("expected error")
		}
		cartRepo.addToCartErr = nil
	})

	t.Run("Scan GetCart Error", func(t *testing.T) {
		cartRepo.getCartItemsErr = errors.New("getcart error")
		_, _, err := s.ScanItem(context.Background(), "user-1", "store-1", "8901234567890", 1)
		if err == nil {
			t.Error("expected error")
		}
		cartRepo.getCartItemsErr = nil
	})
}

func TestGetCartRecalculation(t *testing.T) {
	cartRepo := &mockCartRepo{
		items: map[string]model.CartItem{
			"8901234567890": {Barcode: "8901234567890", Quantity: 2, UnitPrice: 90.0},
		},
	}
	productRepo := &mockProductRepo{
		products: map[string]*model.Product{
			"8901234567890": {Barcode: "8901234567890", Price: 100.0, GSTRate: 18.0},
		},
	}
	s := NewCartService(cartRepo, productRepo)

	t.Run("Recalculate with New Price", func(t *testing.T) {
		cart, err := s.GetCart(context.Background(), "user-1", "store-1")
		if err != nil {
			t.Fatal(err)
		}
		if cart.Subtotal != 200.0 {
			t.Errorf("expected subtotal 200.0, got %f", cart.Subtotal)
		}
	})

	t.Run("Product Not Found during Recalculation", func(t *testing.T) {
		productRepo.products = make(map[string]*model.Product)
		cart, err := s.GetCart(context.Background(), "user-1", "store-1")
		if err != nil {
			t.Fatal(err)
		}
		if cart.Subtotal != 180.0 { // Use existing 90.0 * 2
			t.Errorf("expected subtotal 180.0, got %f", cart.Subtotal)
		}
	})
}

func TestValidateEAN13Full(t *testing.T) {
	if !ValidateEAN13("8901234567890") {
		t.Error("valid barcode failed")
	}
	if ValidateEAN13("123") {
		t.Error("short barcode passed")
	}
	if ValidateEAN13("890123456789A") {
		t.Error("non-numeric check digit passed")
	}
	if ValidateEAN13("89012345678A0") {
		t.Error("non-numeric digit passed")
	}
}

func TestCartOpsErrors(t *testing.T) {
	cartRepo := &mockCartRepo{err: errors.New("repo error")}
	productRepo := &mockProductRepo{}
	s := NewCartService(cartRepo, productRepo)

	t.Run("GetCart Error", func(t *testing.T) {
		_, err := s.GetCart(context.Background(), "u", "s")
		if err == nil {
			t.Error("expected error")
		}
	})
	t.Run("RemoveItem Error", func(t *testing.T) {
		_, err := s.RemoveItem(context.Background(), "u", "s", "b")
		if err == nil {
			t.Error("expected error")
		}
	})
	t.Run("ClearCart Error", func(t *testing.T) {
		err := s.ClearCart(context.Background(), "u", "s")
		if err == nil {
			t.Error("expected error")
		}
	})
}
