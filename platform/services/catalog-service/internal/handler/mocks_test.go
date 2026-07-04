package handler

import (
	"context"

	"github.com/google/uuid"
	"github.com/zippyra/platform/services/catalog-service/internal/model"
)

type mockBarcodeService struct {
	lookup func(ctx context.Context, storeID uuid.UUID, barcode string) (*model.ProductResponse, error)
	upsert func(ctx context.Context, req *model.UpsertProductRequest) (*model.Product, error)
}

func (m *mockBarcodeService) Lookup(ctx context.Context, storeID uuid.UUID, barcode string) (*model.ProductResponse, error) {
	return m.lookup(ctx, storeID, barcode)
}

func (m *mockBarcodeService) UpsertProduct(ctx context.Context, req *model.UpsertProductRequest) (*model.Product, error) {
	return m.upsert(ctx, req)
}

type mockSearchService struct {
	search     func(ctx context.Context, req *model.ProductSearchRequest) (*model.ProductListResponse, error)
	byCategory func(ctx context.Context, storeID uuid.UUID, category string, page, limit int) (*model.ProductListResponse, error)
	index      func(ctx context.Context, p model.Product) error
}

func (m *mockSearchService) Search(ctx context.Context, req *model.ProductSearchRequest) (*model.ProductListResponse, error) {
	return m.search(ctx, req)
}

func (m *mockSearchService) ByCategory(ctx context.Context, storeID uuid.UUID, category string, page, limit int) (*model.ProductListResponse, error) {
	return m.byCategory(ctx, storeID, category, page, limit)
}

func (m *mockSearchService) IndexProduct(ctx context.Context, p model.Product) error {
	return m.index(ctx, p)
}

type mockSyncService struct {
	sync func(ctx context.Context, req *model.SyncRequest) (*model.SyncResponse, error)
}

func (m *mockSyncService) Sync(ctx context.Context, req *model.SyncRequest) (*model.SyncResponse, error) {
	return m.sync(ctx, req)
}

type mockOfferService struct {
	getOffers func(ctx context.Context, storeID uuid.UUID) (*model.OfferResponse, error)
}

func (m *mockOfferService) GetOffers(ctx context.Context, storeID uuid.UUID) (*model.OfferResponse, error) {
	return m.getOffers(ctx, storeID)
}
