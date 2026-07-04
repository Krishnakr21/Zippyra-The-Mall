package repository

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"errors"

	"github.com/go-redis/redismock/v9"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/zippyra/platform/services/cart-service/internal/model"
)

func TestProductRepo(t *testing.T) {
	rdb, mock := redismock.NewClientMock()
	
	// Mock Catalog Service
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/catalog/barcode/890" {
			p := model.Product{ID: "P890", Barcode: "890", Name: "Test"}
			json.NewEncoder(w).Encode(p)
			return
		}
		if r.URL.Path == "/v1/catalog/barcode/404" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.URL.Path == "/v1/catalog/barcode/invalid" {
			w.Write([]byte("invalid json"))
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	repo := NewProductRepository(rdb, ts.URL)
	ctx := context.Background()

	t.Run("NewProductRepository", func(t *testing.T) {
		r := NewProductRepository(rdb, "url")
		assert.NotNil(t, r)
	})

	t.Run("GetProductByBarcode Cache Hit", func(t *testing.T) {
		p := model.Product{ID: "P1", Barcode: "123", Name: "Cache Hit"}
		data, _ := json.Marshal(p)
		mock.ExpectGet("sku:store1:123").SetVal(string(data))

		res, err := repo.GetProductByBarcode(ctx, "store1", "123")
		assert.NoError(t, err)
		assert.Equal(t, "P1", res.ID)
	})

	t.Run("GetProductByBarcode Cache Hit Unmarshal Fail", func(t *testing.T) {
		mock.ExpectGet("sku:store1:123").SetVal("invalid")
		_, err := repo.GetProductByBarcode(ctx, "store1", "123")
		assert.Error(t, err)
	})
	
	t.Run("GetProductByBarcode Redis Error", func(t *testing.T) {
		mock.ExpectGet("sku:store1:123").SetErr(errors.New("redis error"))
		_, err := repo.GetProductByBarcode(ctx, "store1", "123")
		assert.Error(t, err)
	})

	t.Run("GetProductByBarcode Cache Miss Service Hit", func(t *testing.T) {
		mock.ExpectGet("sku:store1:890").SetErr(redis.Nil)
		
		res, err := repo.GetProductByBarcode(ctx, "store1", "890")
		assert.NoError(t, err)
		assert.Equal(t, "P890", res.ID)
	})

	t.Run("GetProductByBarcode 404", func(t *testing.T) {
		mock.ExpectGet("sku:store1:404").SetErr(redis.Nil)
		res, err := repo.GetProductByBarcode(ctx, "store1", "404")
		assert.NoError(t, err)
		assert.Nil(t, res)
	})
	
	t.Run("GetProductByBarcode Decode Fail", func(t *testing.T) {
		mock.ExpectGet("sku:store1:invalid").SetErr(redis.Nil)
		_, err := repo.GetProductByBarcode(ctx, "store1", "invalid")
		assert.Error(t, err)
	})

	t.Run("GetProductByBarcode HttpClient Error", func(t *testing.T) {
		// Use a closed server or malformed URL
		badRepo := NewProductRepository(rdb, "http://invalid-url-123")
		mock.ExpectGet("sku:store1:123").SetErr(redis.Nil)
		_, err := badRepo.GetProductByBarcode(ctx, "store1", "123")
		assert.Error(t, err)
	})

	t.Run("GetProductByBarcode 500", func(t *testing.T) {
		mock.ExpectGet("sku:store1:500").SetErr(redis.Nil)
		_, err := repo.GetProductByBarcode(ctx, "store1", "500")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "status: 500")
	})

	t.Run("GetStoreSession", func(t *testing.T) {
		mock.ExpectGet("store_session:user1").SetVal("store1")
		res, err := repo.GetStoreSession(ctx, "user1")
		assert.NoError(t, err)
		assert.Equal(t, "store1", res)
	})
	
	t.Run("GetStoreSession Redis Nil", func(t *testing.T) {
		mock.ExpectGet("store_session:user1").SetErr(redis.Nil)
		res, err := repo.GetStoreSession(ctx, "user1")
		assert.NoError(t, err)
		assert.Equal(t, "", res)
	})

	t.Run("GetOfferRules", func(t *testing.T) {
		rules := []model.OfferRule{{ID: "O1", Type: "FLAT"}}
		data, _ := json.Marshal(rules)
		mock.ExpectGet("offer_rules:store1").SetVal(string(data))

		res, err := repo.GetOfferRules(ctx, "store1")
		assert.NoError(t, err)
		assert.Len(t, res, 1)
	})

	t.Run("GetOfferRules Redis Nil", func(t *testing.T) {
		mock.ExpectGet("offer_rules:store1").SetErr(redis.Nil)
		res, err := repo.GetOfferRules(ctx, "store1")
		assert.NoError(t, err)
		assert.Len(t, res, 0)
	})

	t.Run("GetOfferRules RedisEmpty", func(t *testing.T) {
		mock.ExpectGet("offer_rules:store1").SetVal("")
		res, err := repo.GetOfferRules(ctx, "store1")
		assert.NoError(t, err)
		assert.Len(t, res, 0)
	})
	
	t.Run("GetOfferRules Redis Error", func(t *testing.T) {
		mock.ExpectGet("offer_rules:store1").SetErr(errors.New("redis error"))
		_, err := repo.GetOfferRules(ctx, "store1")
		assert.Error(t, err)
	})

	t.Run("GetOfferRules Unmarshal Fail", func(t *testing.T) {
		mock.ExpectGet("offer_rules:store1").SetVal("invalid")
		_, err := repo.GetOfferRules(ctx, "store1")
		assert.Error(t, err)
	})
}
