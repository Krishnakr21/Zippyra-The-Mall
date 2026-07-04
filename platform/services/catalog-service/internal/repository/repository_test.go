package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/zippyra/platform/services/catalog-service/internal/model"
)

func TestProductRepo(t *testing.T) {
	// Test interface implementation
	repo := NewProductRepo(nil)
	assert.NotNil(t, repo)

	// Test that all methods exist (they will fail with nil pool, but we're testing interface compliance)
	ctx := context.Background()
	storeID := uuid.New()
	barcode := "123456789"

	// These calls will panic with nil pool, but they verify the methods exist
	assert.Panics(t, func() {
		repo.GetByStoreAndBarcode(ctx, storeID, barcode)
	})

	assert.Panics(t, func() {
		repo.Search(ctx, storeID, "test", nil, 1, 20)
	})

	assert.Panics(t, func() {
		repo.GetByCategory(ctx, storeID, "electronics", 1, 20)
	})

	assert.Panics(t, func() {
		repo.Sync(ctx, storeID, 0, 100)
	})

	assert.Panics(t, func() {
		product := &model.Product{ID: uuid.New()}
		repo.Upsert(ctx, product)
	})

	assert.Panics(t, func() {
		repo.GetByID(ctx, uuid.New())
	})
}

func TestOfferRepo(t *testing.T) {
	// Test interface implementation
	repo := NewOfferRepo(nil)
	assert.NotNil(t, repo)

	// Test that all methods exist
	ctx := context.Background()
	storeID := uuid.New()

	assert.Panics(t, func() {
		repo.GetActiveByStore(ctx, storeID)
	})
}
