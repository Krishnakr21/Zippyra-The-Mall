package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/zippyra/platform/services/catalog-service/internal/model"
)

type OfferRepo interface {
	GetActiveByStore(ctx context.Context, storeID uuid.UUID) ([]model.OfferRule, error)
}

type offerRepo struct {
	db DB
}

func NewOfferRepo(db DB) OfferRepo {
	return &offerRepo{db: db}
}

func (r *offerRepo) GetActiveByStore(ctx context.Context, storeID uuid.UUID) ([]model.OfferRule, error) {
	query := `
		SELECT id, store_id, name, description, type, value, min_amount, max_discount,
		       category, product_ids, priority, is_active, valid_from, valid_until,
		       created_at, updated_at
		FROM offer_rules
		WHERE store_id = $1
		  AND is_active = true
		  AND valid_from <= NOW()
		  AND (valid_until IS NULL OR valid_until >= NOW())
		ORDER BY priority DESC`

	rows, err := r.db.Query(ctx, query, storeID)
	if err != nil {
		return nil, fmt.Errorf("failed to get active offers: %w", err)
	}
	defer rows.Close()

	var offers []model.OfferRule
	for rows.Next() {
		var offer model.OfferRule
		var productIDs []string

		err := rows.Scan(
			&offer.ID, &offer.StoreID, &offer.Name, &offer.Description, &offer.Type, &offer.Value,
			&offer.MinAmount, &offer.MaxDiscount, &offer.Category, &productIDs, &offer.Priority,
			&offer.IsActive, &offer.ValidFrom, &offer.ValidUntil, &offer.CreatedAt, &offer.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan offer row: %w", err)
		}

		for _, idStr := range productIDs {
			if id, err := uuid.Parse(idStr); err == nil {
				offer.ProductIDs = append(offer.ProductIDs, id)
			}
		}

		offers = append(offers, offer)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating offer rows: %w", err)
	}

	return offers, nil
}
