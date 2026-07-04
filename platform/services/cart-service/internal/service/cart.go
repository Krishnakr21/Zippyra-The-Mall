package service

import (
	"context"
	"errors"
	"strconv"

	"github.com/zippyra/platform/services/cart-service/internal/model"
	"github.com/zippyra/platform/services/cart-service/internal/repository"
)

var (
	ErrBarcodeInvalid   = errors.New("invalid barcode format")
	ErrStoreNotFound     = errors.New("store session not found or mismatch")
	ErrBarcodeNotFound  = errors.New("product not found in catalog")
	ErrInsufficientStock = errors.New("insufficient stock")
	ErrCartEmpty        = errors.New("cart is empty")
)

type CartService interface {
	ScanItem(ctx context.Context, userID, storeID, barcode string, quantity int) (*model.CartItem, *model.Cart, error)
	GetCart(ctx context.Context, userID, storeID string) (*model.Cart, error)
	RemoveItem(ctx context.Context, userID, storeID, barcode string) (*model.Cart, error)
	ClearCart(ctx context.Context, userID, storeID string) error
}

type cartService struct {
	cartRepo    repository.CartRepository
	productRepo repository.ProductRepository
}

func NewCartService(cartRepo repository.CartRepository, productRepo repository.ProductRepository) CartService {
	return &cartService{
		cartRepo:    cartRepo,
		productRepo: productRepo,
	}
}

func (s *cartService) ScanItem(ctx context.Context, userID, storeID, barcode string, quantity int) (*model.CartItem, *model.Cart, error) {
	// 1. Validate barcode format (EAN-13)
	if !ValidateEAN13(barcode) {
		return nil, nil, ErrBarcodeInvalid
	}

	// 2. Validate store session
	sessionStoreID, err := s.productRepo.GetStoreSession(ctx, userID)
	if err != nil || sessionStoreID != storeID {
		return nil, nil, ErrStoreNotFound
	}

	// 3. Get product
	product, err := s.productRepo.GetProductByBarcode(ctx, storeID, barcode)
	if err != nil {
		return nil, nil, err
	}
	if product == nil {
		return nil, nil, ErrBarcodeNotFound
	}

	// 4. Check inventory
	stock, err := s.cartRepo.GetStock(ctx, storeID, product.ID)
	if err != nil {
		return nil, nil, err
	}
	if stock < int64(quantity) {
		return nil, nil, ErrInsufficientStock
	}

	// 5. Decrement inventory
	_, err = s.cartRepo.DecrementStock(ctx, storeID, product.ID, quantity)
	if err != nil {
		return nil, nil, err
	}

	// 6. Calculate GST
	// Assuming storeGSTIN is part of the product metadata for now or we get it from store
	// I'll use a placeholder for storeGSTIN if not in product
	storeGSTIN := "27AAAAAA0000A1Z5" // Placeholder for Maharashtra
	customerGSTIN := ""          // B2C default
	gst := CalculateGST(product.Price, product.GSTRate, storeGSTIN, customerGSTIN)

	item := model.CartItem{
		Barcode:     barcode,
		ProductID:   product.ID,
		ProductName: product.Name,
		Quantity:    quantity,
		UnitPrice:   product.Price,
		GSTAmount:   gst.TotalGST * float64(quantity),
		TotalPrice:  (product.Price * float64(quantity)) + (gst.TotalGST * float64(quantity)),
	}

	// 7. Add to cart
	if err := s.cartRepo.AddToCart(ctx, userID, storeID, item); err != nil {
		// Rollback stock? In high-concurrency Redis, maybe not if we want speed, 
		// but for correctness we should.
		_, _ = s.cartRepo.IncrementStock(ctx, storeID, product.ID, quantity)
		return nil, nil, err
	}

	// 8. Get updated cart
	cart, err := s.GetCart(ctx, userID, storeID)
	if err != nil {
		return nil, nil, err
	}

	return &item, cart, nil
}

func (s *cartService) GetCart(ctx context.Context, userID, storeID string) (*model.Cart, error) {
	items, err := s.cartRepo.GetCartItems(ctx, userID, storeID)
	if err != nil {
		return nil, err
	}

	cart := &model.Cart{
		UserID:  userID,
		StoreID: storeID,
		Items:   items,
	}

	var subtotal, gstTotal, cgstTotal, sgstTotal, igstTotal float64
	var itemCount int

	storeGSTIN := "27AAAAAA0000A1Z5" // Placeholder
	customerGSTIN := ""          // B2C

	for i := range cart.Items {
		item := &cart.Items[i]
		// Re-validate price and GST from SKU cache?
		// "Recalculate totals with current prices (detect price changes)"
		product, err := s.productRepo.GetProductByBarcode(ctx, storeID, item.Barcode)
		if err == nil && product != nil {
			item.UnitPrice = product.Price
			gst := CalculateGST(product.Price, product.GSTRate, storeGSTIN, customerGSTIN)
			item.GSTAmount = gst.TotalGST * float64(item.Quantity)
			item.TotalPrice = (item.UnitPrice * float64(item.Quantity)) + item.GSTAmount
			
			subtotal += item.UnitPrice * float64(item.Quantity)
			gstTotal += item.GSTAmount
			cgstTotal += gst.CGSTAmount * float64(item.Quantity)
			sgstTotal += gst.SGSTAmount * float64(item.Quantity)
			igstTotal += gst.IGSTAmount * float64(item.Quantity)
		} else {
			// If product not found in cache, use existing item data or handle error
			subtotal += item.UnitPrice * float64(item.Quantity)
			gstTotal += item.GSTAmount
			// GST breakdown might be inaccurate without product record
		}
		itemCount += item.Quantity
	}

	cart.Subtotal = subtotal
	cart.GSTTotal = gstTotal
	cart.CGSTTotal = cgstTotal
	cart.SGSTTotal = sgstTotal
	cart.IGSTTotal = igstTotal
	cart.ItemCount = itemCount
	cart.TotalAmount = subtotal + gstTotal

	// Apply offers
	rules, err := s.productRepo.GetOfferRules(ctx, storeID)
	if err == nil {
		offer := ApplyOffers(cart, rules)
		if offer != nil {
			cart.OfferApplied = offer
			cart.DiscountTotal = offer.DiscountValue
			cart.TotalAmount -= offer.DiscountValue
		}
	}

	return cart, nil
}

func (s *cartService) RemoveItem(ctx context.Context, userID, storeID, barcode string) (*model.Cart, error) {
	item, err := s.cartRepo.RemoveItem(ctx, userID, storeID, barcode)
	if err != nil {
		return nil, err
	}
	if item != nil {
		_, _ = s.cartRepo.IncrementStock(ctx, storeID, item.ProductID, item.Quantity)
	}

	return s.GetCart(ctx, userID, storeID)
}

func (s *cartService) ClearCart(ctx context.Context, userID, storeID string) error {
	items, err := s.cartRepo.ClearCart(ctx, userID, storeID)
	if err != nil {
		return err
	}

	for _, item := range items {
		_, _ = s.cartRepo.IncrementStock(ctx, storeID, item.ProductID, item.Quantity)
	}
	return nil
}

func ValidateEAN13(barcode string) bool {
	if len(barcode) != 13 {
		return false
	}
	sum := 0
	for i := 0; i < 12; i++ {
		digit, err := strconv.Atoi(string(barcode[i]))
		if err != nil {
			return false
		}
		if i%2 == 0 {
			sum += digit
		} else {
			sum += digit * 3
		}
	}
	checkDigit := (10 - (sum % 10)) % 10
	actualCheckDigit, err := strconv.Atoi(string(barcode[12]))
	if err != nil {
		return false
	}
	return checkDigit == actualCheckDigit
}
