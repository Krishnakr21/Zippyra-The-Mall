package service

import (
	"github.com/zippyra/platform/services/cart-service/internal/model"
)

// ApplyOffers evaluates all active offer rules against the cart and returns the best one.
func ApplyOffers(cart *model.Cart, rules []model.OfferRule) *model.AppliedOffer {
	if len(rules) == 0 {
		return nil
	}

	var bestOffer *model.AppliedOffer
	var maxDiscount float64

	for _, rule := range rules {
		if !rule.IsActive {
			continue
		}

		var discount float64
		switch rule.Type {
		case "PERCENTAGE":
			discount = cart.Subtotal * (rule.Value / 100)
		case "FLAT":
			discount = rule.Value
		case "BOGO":
			// Buy X Get Y Free logic for a specific product
			for _, item := range cart.Items {
				if rule.ProductID != "" && item.ProductID != rule.ProductID {
					continue
				}
				if rule.BuyQuantity+rule.FreeQuantity == 0 {
					continue
				}
				sets := int(item.Quantity / (rule.BuyQuantity + rule.FreeQuantity))
				discount += float64(sets*rule.FreeQuantity) * item.UnitPrice
			}
		case "MIN_QUANTITY":
			if cart.ItemCount >= rule.MinQuantity {
				discount = rule.Value
			}
		default:
			continue
		}

		// Apply MaxDiscount cap if set
		if rule.MaxDiscount > 0 && discount > rule.MaxDiscount {
			discount = rule.MaxDiscount
		}

		if discount > maxDiscount {
			maxDiscount = discount
			bestOffer = &model.AppliedOffer{
				OfferID:       rule.ID,
				DiscountValue: discount,
			}
		}
	}

	if bestOffer == nil || bestOffer.DiscountValue <= 0 {
		return nil
	}

	return bestOffer
}
