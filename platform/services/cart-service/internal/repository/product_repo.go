package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/zippyra/platform/services/cart-service/internal/model"
)

type ProductRepository interface {
	GetProductByBarcode(ctx context.Context, storeID, barcode string) (*model.Product, error)
	GetStoreSession(ctx context.Context, userID string) (string, error)
	GetOfferRules(ctx context.Context, storeID string) ([]model.OfferRule, error)
}

type productRepo struct {
	rdb            *redis.Client
	catalogService string
	httpClient     *http.Client
}

func NewProductRepository(rdb *redis.Client, catalogServiceURL string) ProductRepository {
	return &productRepo{
		rdb:            rdb,
		catalogService: catalogServiceURL,
		httpClient:     &http.Client{Timeout: 2 * time.Second},
	}
}

func (r *productRepo) GetProductByBarcode(ctx context.Context, storeID, barcode string) (*model.Product, error) {
	key := fmt.Sprintf("sku:%s:%s", storeID, barcode)
	val, err := r.rdb.Get(ctx, key).Result()
	if err == nil {
		var p model.Product
		if err := json.Unmarshal([]byte(val), &p); err == nil {
			return &p, nil
		}
	}

	// Cache miss or unmarshal error - try catalog service
	url := fmt.Sprintf("%s/v1/catalog/barcode/%s?store_id=%s", r.catalogService, barcode, storeID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil // ErrBarcodeNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("catalog service returned status: %d", resp.StatusCode)
	}

	var p model.Product
	if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
		return nil, err
	}

	return &p, nil
}

func (r *productRepo) GetStoreSession(ctx context.Context, userID string) (string, error) {
	key := fmt.Sprintf("store_session:%s", userID)
	val, err := r.rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", nil
	}
	return val, err
}

func (r *productRepo) GetOfferRules(ctx context.Context, storeID string) ([]model.OfferRule, error) {
	key := fmt.Sprintf("offer_rules:%s", storeID)
	val, err := r.rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	if val == "" {
		return nil, nil
	}

	var rules []model.OfferRule
	if err := json.Unmarshal([]byte(val), &rules); err != nil {
		return nil, err
	}
	return rules, nil
}
