package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/zippyra/platform/services/catalog-service/internal/model"
	sharedErrors "github.com/zippyra/platform/shared/errors"
)

type mockProductRepo struct {
	getByStoreAndBarcode func(ctx context.Context, storeID uuid.UUID, barcode string) (*model.Product, error)
	search               func(ctx context.Context, storeID uuid.UUID, query string, category *string, page, limit int) ([]model.Product, int, error)
	getByCategory        func(ctx context.Context, storeID uuid.UUID, category string, page, limit int) ([]model.Product, int, error)
	sync                 func(ctx context.Context, storeID uuid.UUID, lastSyncSeq int64, limit int) ([]model.Product, error)
	upsert               func(ctx context.Context, p *model.Product) error
	getByID              func(ctx context.Context, id uuid.UUID) (*model.Product, error)
}

func (m *mockProductRepo) GetByStoreAndBarcode(ctx context.Context, storeID uuid.UUID, barcode string) (*model.Product, error) {
	return m.getByStoreAndBarcode(ctx, storeID, barcode)
}

func (m *mockProductRepo) Search(ctx context.Context, storeID uuid.UUID, query string, category *string, page, limit int) ([]model.Product, int, error) {
	return m.search(ctx, storeID, query, category, page, limit)
}

func (m *mockProductRepo) GetByCategory(ctx context.Context, storeID uuid.UUID, category string, page, limit int) ([]model.Product, int, error) {
	return m.getByCategory(ctx, storeID, category, page, limit)
}

func (m *mockProductRepo) Sync(ctx context.Context, storeID uuid.UUID, lastSyncSeq int64, limit int) ([]model.Product, error) {
	return m.sync(ctx, storeID, lastSyncSeq, limit)
}

func (m *mockProductRepo) Upsert(ctx context.Context, p *model.Product) error {
	return m.upsert(ctx, p)
}

func (m *mockProductRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Product, error) {
	return m.getByID(ctx, id)
}

type mockCache struct {
	get        func(ctx context.Context, storeID, barcode string) (*model.Product, error)
	set        func(ctx context.Context, storeID, barcode string, product *model.Product) error
	invalidate func(ctx context.Context, storeID, barcode string) error
}

func (m *mockCache) Get(ctx context.Context, storeID, barcode string) (*model.Product, error) {
	return m.get(ctx, storeID, barcode)
}

func (m *mockCache) Set(ctx context.Context, storeID, barcode string, product *model.Product) error {
	return m.set(ctx, storeID, barcode, product)
}

func (m *mockCache) Invalidate(ctx context.Context, storeID, barcode string) error {
	return m.invalidate(ctx, storeID, barcode)
}

type mockSearchIndexService struct {
	indexProduct func(ctx context.Context, p model.Product) error
}

func (m *mockSearchIndexService) IndexProduct(ctx context.Context, p model.Product) error {
	return m.indexProduct(ctx, p)
}

type mockSearchClient struct {
	searchFunc       func(ctx context.Context, storeID, query, category string, page, limit int) ([]model.Product, int, error)
	indexProductFunc func(ctx context.Context, p model.Product) error
}

func (m *mockSearchClient) Search(ctx context.Context, storeID, query, category string, page, limit int) ([]model.Product, int, error) {
	return m.searchFunc(ctx, storeID, query, category, page, limit)
}

func (m *mockSearchClient) IndexProduct(ctx context.Context, p model.Product) error {
	return m.indexProductFunc(ctx, p)
}

type mockOfferCache struct {
	getOffers func(ctx context.Context, key string) (*model.OfferResponse, error)
	setOffers func(ctx context.Context, key string, response *model.OfferResponse) error
}

func (m *mockOfferCache) GetOffers(ctx context.Context, key string) (*model.OfferResponse, error) {
	return m.getOffers(ctx, key)
}

func (m *mockOfferCache) SetOffers(ctx context.Context, key string, response *model.OfferResponse) error {
	return m.setOffers(ctx, key, response)
}

func TestService_Constructors(t *testing.T) {
	repo := &mockProductRepo{}
	cache := &mockCache{}
	search := &mockSearchIndexService{}
	offerRepo := &mockOfferRepo{}
	offerCache := &mockOfferCache{}
	searchClient := &mockSearchClient{}

	assert.NotNil(t, NewBarcodeService(repo, cache, search))
	assert.NotNil(t, NewSearchService(repo, searchClient))
	assert.NotNil(t, NewOfferService(offerRepo, offerCache))
	assert.NotNil(t, NewSyncService(repo))
}

func TestBarcodeService_Lookup(t *testing.T) {
	storeID := uuid.New()
	barcode := "123456789"
	ctx := context.Background()

	t.Run("Empty Barcode", func(t *testing.T) {
		s := &barcodeService{}
		res, err := s.Lookup(ctx, storeID, "")
		assert.Error(t, err)
		assert.Nil(t, res)
	})

	t.Run("Cache Hit", func(t *testing.T) {
		s := &barcodeService{
			cache: &mockCache{
				get: func(ctx context.Context, storeID, barcode string) (*model.Product, error) {
					return &model.Product{Name: "P"}, nil
				},
			},
		}
		res, err := s.Lookup(ctx, storeID, barcode)
		assert.NoError(t, err)
		assert.True(t, res.CacheHit)
	})

	t.Run("Cache Error Loopup Fallback", func(t *testing.T) {
		s := &barcodeService{
			cache: &mockCache{
				get: func(ctx context.Context, storeID, barcode string) (*model.Product, error) {
					return nil, errors.New("cache error")
				},
				set: func(ctx context.Context, storeID, barcode string, product *model.Product) error {
					return errors.New("cache set error") // To cover line 72
				},
			},
			productRepo: &mockProductRepo{
				getByStoreAndBarcode: func(ctx context.Context, storeID uuid.UUID, barcode string) (*model.Product, error) {
					return &model.Product{Name: "P"}, nil
				},
			},
		}
		res, err := s.Lookup(ctx, storeID, barcode)
		assert.NoError(t, err)
		assert.False(t, res.CacheHit)
	})

	t.Run("Product Not Found", func(t *testing.T) {
		s := &barcodeService{
			cache: &mockCache{
				get: func(ctx context.Context, storeID, barcode string) (*model.Product, error) {
					return nil, nil
				},
			},
			productRepo: &mockProductRepo{
				getByStoreAndBarcode: func(ctx context.Context, storeID uuid.UUID, barcode string) (*model.Product, error) {
					return nil, errors.New("failed to get product by store and barcode: no rows in result set")
				},
			},
		}
		_, err := s.Lookup(ctx, storeID, barcode)
		assert.Error(t, err)
		assert.Equal(t, sharedErrors.ErrBarcodeNotFound, err.(*sharedErrors.AppError).Code)
	})

	t.Run("Database Error", func(t *testing.T) {
		s := &barcodeService{
			cache: &mockCache{
				get: func(ctx context.Context, storeID, barcode string) (*model.Product, error) {
					return nil, nil
				},
			},
			productRepo: &mockProductRepo{
				getByStoreAndBarcode: func(ctx context.Context, storeID uuid.UUID, barcode string) (*model.Product, error) {
					return nil, errors.New("db error")
				},
			},
		}
		_, err := s.Lookup(ctx, storeID, barcode)
		assert.Error(t, err)
		assert.Equal(t, sharedErrors.ErrInternal, err.(*sharedErrors.AppError).Code)
	})
}

func TestBarcodeService_UpsertProduct(t *testing.T) {
	ctx := context.Background()
	req := &model.UpsertProductRequest{
		StoreID:      uuid.New(),
		Barcode:      "123",
		Name:         "P",
		Brand:        "B",
		Category:     "C",
		HSNCode:      "H",
		MRP:          10,
		SellingPrice: 8,
		Unit:         "U",
	}

	t.Run("Validation Failed", func(t *testing.T) {
		s := &barcodeService{}
		_, err := s.UpsertProduct(ctx, &model.UpsertProductRequest{})
		assert.Error(t, err)

		// Detailed validation paths
		reqs := []*model.UpsertProductRequest{
			{StoreID: uuid.Nil},
			{StoreID: uuid.New(), Barcode: ""},
			{StoreID: uuid.New(), Barcode: "123", Name: ""},
			{StoreID: uuid.New(), Barcode: "123", Name: "P", Brand: ""},
			{StoreID: uuid.New(), Barcode: "123", Name: "P", Brand: "B", Category: ""},
			{StoreID: uuid.New(), Barcode: "123", Name: "P", Brand: "B", Category: "C", HSNCode: ""},
			{StoreID: uuid.New(), Barcode: "123", Name: "P", Brand: "B", Category: "C", HSNCode: "H", MRP: 0},
			{StoreID: uuid.New(), Barcode: "123", Name: "P", Brand: "B", Category: "C", HSNCode: "H", MRP: 10, SellingPrice: 0},
			{StoreID: uuid.New(), Barcode: "123", Name: "P", Brand: "B", Category: "C", HSNCode: "H", MRP: 10, SellingPrice: 8, GSTRate: -1},
			{StoreID: uuid.New(), Barcode: "123", Name: "P", Brand: "B", Category: "C", HSNCode: "H", MRP: 10, SellingPrice: 8, GSTRate: 101},
			{StoreID: uuid.New(), Barcode: "123", Name: "P", Brand: "B", Category: "C", HSNCode: "H", MRP: 10, SellingPrice: 8, GSTRate: 18, Unit: ""},
			{StoreID: uuid.New(), Barcode: "123", Name: "P", Brand: "B", Category: "C", HSNCode: "H", MRP: 10, SellingPrice: 8, GSTRate: 18, Unit: "U", StockQuantity: -1},
			{StoreID: uuid.New(), Barcode: "123", Name: "P", Brand: "B", Category: "C", HSNCode: "H", MRP: 10, SellingPrice: 8, GSTRate: 18, Unit: "U", StockQuantity: 0, ReorderPoint: intPtr(-1)},
			{StoreID: uuid.New(), Barcode: "123", Name: "P", Brand: "B", Category: "C", HSNCode: "H", MRP: 10, SellingPrice: 8, GSTRate: 18, Unit: "U", StockQuantity: 0, ReorderQuantity: intPtr(-1)},
		}
		for _, r := range reqs {
			_, err := s.UpsertProduct(ctx, r)
			assert.Error(t, err)
		}
	})

	t.Run("Success", func(t *testing.T) {
		s := &barcodeService{
			productRepo: &mockProductRepo{
				upsert: func(ctx context.Context, p *model.Product) error { return nil },
			},
			cache: &mockCache{
				invalidate: func(ctx context.Context, storeID, barcode string) error { return nil },
			},
			search: &mockSearchIndexService{
				indexProduct: func(ctx context.Context, p model.Product) error { return nil },
			},
		}
		_, err := s.UpsertProduct(ctx, req)
		assert.NoError(t, err)
	})

	t.Run("Repository Error", func(t *testing.T) {
		s := &barcodeService{
			productRepo: &mockProductRepo{
				upsert: func(ctx context.Context, p *model.Product) error { return errors.New("err") },
			},
		}
		_, err := s.UpsertProduct(ctx, req)
		assert.Error(t, err)
	})

	t.Run("Cache and Search Errors", func(t *testing.T) {
		s := &barcodeService{
			productRepo: &mockProductRepo{
				upsert: func(ctx context.Context, p *model.Product) error { return nil },
			},
			cache: &mockCache{
				invalidate: func(ctx context.Context, storeID, barcode string) error { return errors.New("err") }, // Cover line 122
			},
			search: &mockSearchIndexService{
				indexProduct: func(ctx context.Context, p model.Product) error { return errors.New("err") }, // Cover line 126
			},
		}
		_, err := s.UpsertProduct(ctx, req)
		assert.NoError(t, err)
	})
	
	t.Run("Optional Fields", func(t *testing.T) {
		r := *req
		valid := true
		r.IsReturnable = &valid
		rp := 5
		r.ReorderPoint = &rp
		s := &barcodeService{
			productRepo: &mockProductRepo{
				upsert: func(ctx context.Context, p *model.Product) error { return nil },
			},
			cache: &mockCache{
				invalidate: func(ctx context.Context, storeID, barcode string) error { return nil },
			},
			search: &mockSearchIndexService{
				indexProduct: func(ctx context.Context, p model.Product) error { return nil },
			},
		}
		_, err := s.UpsertProduct(ctx, &r)
		assert.NoError(t, err)
	})
}

func TestSearchService_Search(t *testing.T) {
	ctx := context.Background()
	cat := "C"
	req := &model.ProductSearchRequest{
		StoreID:  uuid.New(),
		Query:    "Q",
		Category: &cat,
		Page:     1,
		Limit:    10,
	}

	t.Run("Success", func(t *testing.T) {
		s := &searchService{
			search: &mockSearchClient{
				searchFunc: func(ctx context.Context, storeID, query, category string, page, limit int) ([]model.Product, int, error) {
					return []model.Product{{Name: "P"}}, 1, nil
				},
			},
		}
		res, err := s.Search(ctx, req)
		assert.NoError(t, err)
		assert.Len(t, res.Products, 1)
	})

	t.Run("Search Client Error Fallback", func(t *testing.T) {
		s := &searchService{
			search: &mockSearchClient{
				searchFunc: func(ctx context.Context, storeID, query, category string, page, limit int) ([]model.Product, int, error) {
					return nil, 0, errors.New("err")
				},
			},
			productRepo: &mockProductRepo{
				search: func(ctx context.Context, storeID uuid.UUID, query string, category *string, page, limit int) ([]model.Product, int, error) {
					return []model.Product{{Name: "P"}}, 1, nil
				},
			},
		}
		res, err := s.Search(ctx, req)
		assert.NoError(t, err)
		assert.Len(t, res.Products, 1)
	})

	t.Run("Fallback Error", func(t *testing.T) {
		s := &searchService{
			search: &mockSearchClient{
				searchFunc: func(ctx context.Context, storeID, query, category string, page, limit int) ([]model.Product, int, error) {
					return nil, 0, errors.New("err")
				},
			},
			productRepo: &mockProductRepo{
				search: func(ctx context.Context, storeID uuid.UUID, query string, category *string, page, limit int) ([]model.Product, int, error) {
					return nil, 0, errors.New("err")
				},
			},
		}
		_, err := s.Search(ctx, req)
		assert.Error(t, err)
	})
}

func TestSearchService_ByCategory(t *testing.T) {
	ctx := context.Background()
	storeID := uuid.New()
	
	t.Run("Validation Errors", func(t *testing.T) {
		s := &searchService{}
		_, err := s.ByCategory(ctx, uuid.Nil, "", 0, 0)
		assert.Error(t, err)
	})

	t.Run("Success", func(t *testing.T) {
		s := &searchService{
			productRepo: &mockProductRepo{
				getByCategory: func(ctx context.Context, storeID uuid.UUID, category string, page, limit int) ([]model.Product, int, error) {
					return []model.Product{{Name: "P"}}, 1, nil
				},
			},
		}
		res, err := s.ByCategory(ctx, storeID, "C", 1, 10)
		assert.NoError(t, err)
		assert.Len(t, res.Products, 1)
	})

	t.Run("Repository Error", func(t *testing.T) {
		s := &searchService{
			productRepo: &mockProductRepo{
				getByCategory: func(ctx context.Context, storeID uuid.UUID, category string, page, limit int) ([]model.Product, int, error) {
					return nil, 0, errors.New("err")
				},
			},
		}
		_, err := s.ByCategory(ctx, storeID, "C", 1, 10)
		assert.Error(t, err)
	})
}

func TestSearchService_IndexProduct(t *testing.T) {
	ctx := context.Background()
	p := model.Product{ID: uuid.New()}

	t.Run("Success", func(t *testing.T) {
		s := &searchService{
			search: &mockSearchClient{
				indexProductFunc: func(ctx context.Context, p model.Product) error { return nil },
			},
		}
		err := s.IndexProduct(ctx, p)
		assert.NoError(t, err)
	})

	t.Run("Error", func(t *testing.T) {
		s := &searchService{
			search: &mockSearchClient{
				indexProductFunc: func(ctx context.Context, p model.Product) error { return errors.New("err") },
			},
		}
		err := s.IndexProduct(ctx, p)
		assert.Error(t, err)
	})

	t.Run("Nil Client", func(t *testing.T) {
		s := &searchService{}
		err := s.IndexProduct(ctx, p)
		assert.NoError(t, err)
	})
}

func TestOfferService_GetOffers(t *testing.T) {
	ctx := context.Background()
	storeID := uuid.New()

	t.Run("Empty Store ID", func(t *testing.T) {
		s := &offerService{}
		_, err := s.GetOffers(ctx, uuid.Nil)
		assert.Error(t, err)
	})

	t.Run("Cache Hit", func(t *testing.T) {
		s := &offerService{
			cache: &mockOfferCache{
				getOffers: func(ctx context.Context, key string) (*model.OfferResponse, error) {
					return &model.OfferResponse{Offers: []model.OfferRule{{Name: "O"}}}, nil
				},
			},
		}
		res, err := s.GetOffers(ctx, storeID)
		assert.NoError(t, err)
		assert.Len(t, res.Offers, 1)
	})

	t.Run("Cache Error Fallback", func(t *testing.T) {
		s := &offerService{
			cache: &mockOfferCache{
				getOffers: func(ctx context.Context, key string) (*model.OfferResponse, error) {
					return nil, errors.New("err")
				},
				setOffers: func(ctx context.Context, key string, response *model.OfferResponse) error {
					return errors.New("err") // Cover line 63
				},
			},
			offerRepo: &mockOfferRepo{
				getActiveByStore: func(ctx context.Context, storeID uuid.UUID) ([]model.OfferRule, error) {
					return []model.OfferRule{{Name: "O"}}, nil
				},
			},
		}
		res, err := s.GetOffers(ctx, storeID)
		assert.NoError(t, err)
		assert.Len(t, res.Offers, 1)
	})

	t.Run("Repository Error", func(t *testing.T) {
		s := &offerService{
			cache: &mockOfferCache{
				getOffers: func(ctx context.Context, key string) (*model.OfferResponse, error) {
					return nil, nil
				},
			},
			offerRepo: &mockOfferRepo{
				getActiveByStore: func(ctx context.Context, storeID uuid.UUID) ([]model.OfferRule, error) {
					return nil, errors.New("err")
				},
			},
		}
		_, err := s.GetOffers(ctx, storeID)
		assert.Error(t, err)
	})
}

func TestSyncService_Sync(t *testing.T) {
	ctx := context.Background()
	req := &model.SyncRequest{
		StoreID:     uuid.New(),
		LastSyncSeq: 0,
		Limit:       10,
	}

	t.Run("Validation Failed", func(t *testing.T) {
		s := &syncService{}
		_, err := s.Sync(ctx, &model.SyncRequest{})
		assert.Error(t, err)
	})

	t.Run("Repository Error", func(t *testing.T) {
		s := &syncService{
			productRepo: &mockProductRepo{
				sync: func(ctx context.Context, storeID uuid.UUID, lastSyncSeq int64, limit int) ([]model.Product, error) {
					return nil, errors.New("err")
				},
			},
		}
		_, err := s.Sync(ctx, req)
		assert.Error(t, err)
	})

	t.Run("Has More Products", func(t *testing.T) {
		s := &syncService{
			productRepo: &mockProductRepo{
				sync: func(ctx context.Context, storeID uuid.UUID, lastSyncSeq int64, limit int) ([]model.Product, error) {
					if limit == 10 {
						return []model.Product{{SyncSeq: 1}}, nil
					}
					return []model.Product{{SyncSeq: 2}}, nil // next batch
				},
			},
		}
		res, err := s.Sync(ctx, req)
		assert.NoError(t, err)
		assert.True(t, res.HasMore)
	})

	t.Run("No More Products", func(t *testing.T) {
		s := &syncService{
			productRepo: &mockProductRepo{
				sync: func(ctx context.Context, storeID uuid.UUID, lastSyncSeq int64, limit int) ([]model.Product, error) {
					if limit == 10 {
						return []model.Product{{SyncSeq: 1}}, nil
					}
					return []model.Product{}, nil
				},
			},
		}
		res, err := s.Sync(ctx, req)
		assert.NoError(t, err)
		assert.False(t, res.HasMore)
	})

	t.Run("More Products Check Error", func(t *testing.T) {
		s := &syncService{
			productRepo: &mockProductRepo{
				sync: func(ctx context.Context, storeID uuid.UUID, lastSyncSeq int64, limit int) ([]model.Product, error) {
					if limit == 10 {
						return []model.Product{{SyncSeq: 1}}, nil
					}
					return nil, errors.New("err") // Cover line 53
				},
			},
		}
		res, err := s.Sync(ctx, req)
		assert.NoError(t, err)
		assert.False(t, res.HasMore)
	})
}

type mockOfferRepo struct {
	getActiveByStore func(ctx context.Context, storeID uuid.UUID) ([]model.OfferRule, error)
	upsert           func(ctx context.Context, offer *model.OfferRule) error
	getByID          func(ctx context.Context, id uuid.UUID) (*model.OfferRule, error)
}

func (m *mockOfferRepo) GetActiveByStore(ctx context.Context, storeID uuid.UUID) ([]model.OfferRule, error) {
	return m.getActiveByStore(ctx, storeID)
}

func (m *mockOfferRepo) Upsert(ctx context.Context, offer *model.OfferRule) error {
	return m.upsert(ctx, offer)
}

func (m *mockOfferRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.OfferRule, error) {
	return m.getByID(ctx, id)
}

func strPtr(s string) *string { return &s }
func floatPtr(f float64) *float64 { return &f }
func timePtr(t time.Time) *time.Time { return &t }
func intPtr(i int) *int { return &i }
