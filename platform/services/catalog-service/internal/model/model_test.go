package model

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestProductModel(t *testing.T) {
	// Test Product struct creation and validation
	product := Product{
		ID:            uuid.New(),
		StoreID:       uuid.New(),
		Barcode:       "123456789",
		Name:          "Test Product",
		Description:   func() *string { s := "Test Description"; return &s }(),
		Brand:         "Test Brand",
		Category:      "Test Category",
		HSNCode:       "123456",
		MRP:           100.0,
		SellingPrice:  90.0,
		GSTRate:       18.0,
		Unit:          "pcs",
		ImageURL:      func() *string { s := "http://example.com/image.jpg"; return &s }(),
		ThumbnailURL:  func() *string { s := "http://example.com/thumb.jpg"; return &s }(),
		IsActive:      true,
		IsReturnable:  true,
		StockQuantity: 100,
		ReorderPoint:  10,
		SyncSeq:       1,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	// Test that all fields are set correctly
	assert.NotEqual(t, uuid.Nil, product.ID)
	assert.NotEqual(t, uuid.Nil, product.StoreID)
	assert.Equal(t, "123456789", product.Barcode)
	assert.Equal(t, "Test Product", product.Name)
	assert.Equal(t, "Test Brand", product.Brand)
	assert.Equal(t, "Test Category", product.Category)
	assert.Equal(t, "123456", product.HSNCode)
	assert.Equal(t, 100.0, product.MRP)
	assert.Equal(t, 90.0, product.SellingPrice)
	assert.Equal(t, 18.0, product.GSTRate)
	assert.Equal(t, "pcs", product.Unit)
	assert.True(t, product.IsActive)
	assert.True(t, product.IsReturnable)
	assert.Equal(t, 100, product.StockQuantity)
	assert.Equal(t, int64(1), product.SyncSeq)
	assert.NotNil(t, product.Description)
	assert.NotNil(t, product.ImageURL)
	assert.NotNil(t, product.ThumbnailURL)
	assert.NotNil(t, product.ReorderPoint)
}

func TestProductResponse(t *testing.T) {
	product := Product{
		ID:       uuid.New(),
		StoreID:  uuid.New(),
		Barcode:  "123456789",
		Name:     "Test Product",
		IsActive: true,
	}

	response := ProductResponse{
		Product:  product,
		CacheHit: true,
	}

	assert.Equal(t, product.ID, response.Product.ID)
	assert.Equal(t, product.Barcode, response.Product.Barcode)
	assert.True(t, response.CacheHit)
}

func TestProductSearchRequest(t *testing.T) {
	req := ProductSearchRequest{
		StoreID:  uuid.New(),
		Query:    "test query",
		Category: func() *string { s := "electronics"; return &s }(),
		Page:     1,
		Limit:    20,
	}

	assert.NotEqual(t, uuid.Nil, req.StoreID)
	assert.Equal(t, "test query", req.Query)
	assert.NotNil(t, req.Category)
	assert.Equal(t, "electronics", *req.Category)
	assert.Equal(t, 1, req.Page)
	assert.Equal(t, 20, req.Limit)
}

func TestProductListResponse(t *testing.T) {
	products := []Product{
		{
			ID:      uuid.New(),
			Barcode: "123456789",
			Name:    "Product 1",
		},
		{
			ID:      uuid.New(),
			Barcode: "987654321",
			Name:    "Product 2",
		},
	}

	response := ProductListResponse{
		Products: products,
		Total:    2,
		Page:     1,
		Limit:    20,
	}

	assert.Equal(t, 2, len(response.Products))
	assert.Equal(t, 2, response.Total)
	assert.Equal(t, 1, response.Page)
	assert.Equal(t, 20, response.Limit)
}

func TestUpsertProductRequest(t *testing.T) {
	req := UpsertProductRequest{
		StoreID:       uuid.New(),
		Barcode:       "123456789",
		Name:          "Test Product",
		Brand:         "Test Brand",
		Category:      "Test Category",
		HSNCode:       "123456",
		MRP:           100.0,
		SellingPrice:  90.0,
		GSTRate:       18.0,
		Unit:          "pcs",
		StockQuantity: 100,
		Description:   func() *string { s := "Test Description"; return &s }(),
		ImageURL:      func() *string { s := "http://example.com/image.jpg"; return &s }(),
		ThumbnailURL:  func() *string { s := "http://example.com/thumb.jpg"; return &s }(),
		IsReturnable:  func() *bool { b := true; return &b }(),
		ReorderPoint:  func() *int { i := 10; return &i }(),
	}

	assert.NotEqual(t, uuid.Nil, req.StoreID)
	assert.Equal(t, "123456789", req.Barcode)
	assert.Equal(t, "Test Product", req.Name)
	assert.Equal(t, "Test Brand", req.Brand)
	assert.Equal(t, "Test Category", req.Category)
	assert.Equal(t, "123456", req.HSNCode)
	assert.Equal(t, 100.0, req.MRP)
	assert.Equal(t, 90.0, req.SellingPrice)
	assert.Equal(t, 18.0, req.GSTRate)
	assert.Equal(t, "pcs", req.Unit)
	assert.Equal(t, 100, req.StockQuantity)
	assert.NotNil(t, req.Description)
	assert.NotNil(t, req.ImageURL)
	assert.NotNil(t, req.ThumbnailURL)
	assert.NotNil(t, req.IsReturnable)
	assert.NotNil(t, req.ReorderPoint)
}

func TestSyncRequest(t *testing.T) {
	req := SyncRequest{
		StoreID:     uuid.New(),
		LastSyncSeq: 100,
		Limit:       50,
	}

	assert.NotEqual(t, uuid.Nil, req.StoreID)
	assert.Equal(t, int64(100), req.LastSyncSeq)
	assert.Equal(t, 50, req.Limit)
}

func TestSyncResponse(t *testing.T) {
	products := []Product{
		{
			ID:       uuid.New(),
			Barcode:  "123456789",
			Name:     "Test Product",
			SyncSeq:  101,
			IsActive: true,
		},
	}

	response := SyncResponse{
		Products:     products,
		NextSyncSeq:  102,
		HasMore:      false,
		TotalChanges: 1,
	}

	assert.Equal(t, 1, len(response.Products))
	assert.Equal(t, int64(102), response.NextSyncSeq)
	assert.False(t, response.HasMore)
	assert.Equal(t, 1, response.TotalChanges)
}

func TestOfferRule(t *testing.T) {
	now := time.Now()
	validUntil := now.Add(30 * 24 * time.Hour)

	offer := OfferRule{
		ID:          uuid.New(),
		StoreID:     uuid.New(),
		Name:        "Test Offer",
		Description: func() *string { s := "Test Description"; return &s }(),
		Type:        "discount",
		Value:       10.0,
		MinAmount:   func() *float64 { v := 50.0; return &v }(),
		MaxDiscount: func() *float64 { v := 5.0; return &v }(),
		Category:    func() *string { s := "electronics"; return &s }(),
		ProductIDs:  []uuid.UUID{uuid.New(), uuid.New()},
		Priority:    1,
		IsActive:    true,
		ValidFrom:   now,
		ValidUntil:  &validUntil,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	assert.NotEqual(t, uuid.Nil, offer.ID)
	assert.NotEqual(t, uuid.Nil, offer.StoreID)
	assert.Equal(t, "Test Offer", offer.Name)
	assert.Equal(t, "discount", offer.Type)
	assert.Equal(t, 10.0, offer.Value)
	assert.True(t, offer.IsActive)
	assert.Equal(t, 2, len(offer.ProductIDs))
	assert.NotNil(t, offer.Description)
	assert.NotNil(t, offer.MinAmount)
	assert.NotNil(t, offer.MaxDiscount)
	assert.NotNil(t, offer.Category)
	assert.NotNil(t, offer.ValidUntil)
}

func TestOfferResponse(t *testing.T) {
	offers := []OfferRule{
		{
			ID:        uuid.New(),
			Name:      "Offer 1",
			Type:      "discount",
			Value:     10.0,
			IsActive:  true,
			ValidFrom: time.Now(),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:        uuid.New(),
			Name:      "Offer 2",
			Type:      "bogo",
			Value:     0.0,
			IsActive:  true,
			ValidFrom: time.Now(),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}

	response := OfferResponse{
		Offers: offers,
	}

	assert.Equal(t, 2, len(response.Offers))
	assert.Equal(t, "Offer 1", response.Offers[0].Name)
	assert.Equal(t, "Offer 2", response.Offers[1].Name)
}
