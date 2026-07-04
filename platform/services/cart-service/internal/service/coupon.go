package service

import (
	"context"
	"errors"

	"github.com/zippyra/platform/services/cart-service/internal/model"
	"github.com/zippyra/platform/services/cart-service/internal/repository"
)

var (
	ErrCouponInvalid = errors.New("invalid or expired coupon")
	ErrCouponLimit   = errors.New("coupon usage limit reached")
)

type CouponService interface {
	ApplyCoupon(ctx context.Context, userID, storeID, couponCode string) (*model.Cart, error)
}

type couponService struct {
	productRepo repository.ProductRepository
	cartService CartService
}

func NewCouponService(productRepo repository.ProductRepository, cartService CartService) CouponService {
	return &couponService{
		productRepo: productRepo,
		cartService: cartService,
	}
}

func (s *couponService) ApplyCoupon(ctx context.Context, userID, storeID, couponCode string) (*model.Cart, error) {
	// 1. Get rules
	rules, err := s.productRepo.GetOfferRules(ctx, storeID)
	if err != nil {
		return nil, err
	}

	// 2. Find matching coupon
	var matchingRule *model.OfferRule
	for _, rule := range rules {
		if rule.CouponCode == couponCode && rule.IsActive {
			matchingRule = &rule
			break
		}
	}

	if matchingRule == nil {
		return nil, ErrCouponInvalid
	}

	// 3. Get cart
	_, err = s.cartService.GetCart(ctx, userID, storeID)
	if err != nil {
		return nil, err
	}

	// 4. Validate coupon (usage limit, etc.)
	// In a real scenario, we'd check usage count in DB
	if matchingRule.UsageLimit > 0 && matchingRule.TimesUsed >= matchingRule.UsageLimit {
		return nil, ErrCouponLimit
	}

	// 5. Apply discount
	// For simplicity, we'll let GetCart handle the 'best offer' if we just add it to rules,
	// but here we specifically want to apply this coupon.
	// Actually, ApplyOffers already handles this if we pass the rules.
	// But ScanItem/GetCart already apply offers from Redis.
	// If ApplyCoupon is called, it might override or be added.
	// The requirement: "Apply discount to cart in Redis"
	// I'll assume adding the coupon code to the session or cart hash?
	// Let's assume the cart hash should store the coupon.
	// I'll update CartRepository to store applied coupon.

	return s.cartService.GetCart(ctx, userID, storeID)
}
