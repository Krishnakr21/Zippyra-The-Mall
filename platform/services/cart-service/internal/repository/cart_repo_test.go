package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/go-redis/redismock/v9"
	"github.com/pashagolub/pgxmock/v3"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zippyra/platform/services/cart-service/internal/model"
)

func TestCartRepoRedis(t *testing.T) {
	rdb, mock := redismock.NewClientMock()
	repo := &redisCartRepo{rdb: rdb, db: nil}

	ctx := context.Background()
	userID := "user1"
	storeID := "store1"
	key := fmt.Sprintf("cart:%s:%s", userID, storeID)

	t.Run("NewCartRepository", func(t *testing.T) {
		r := NewCartRepository(rdb, nil)
		assert.NotNil(t, r)
	})

	t.Run("AddToCart", func(t *testing.T) {
		item := model.CartItem{Barcode: "123", ProductID: "P1", Quantity: 1}
		data, _ := json.Marshal(item)
		mock.ExpectHSet(key, item.Barcode, data).SetVal(1)

		err := repo.AddToCart(ctx, userID, storeID, item)
		assert.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("AddToCart Error", func(t *testing.T) {
		item := model.CartItem{Barcode: "123"}
		data, _ := json.Marshal(item)
		mock.ExpectHSet(key, "123", data).SetErr(errors.New("redis error"))
		err := repo.AddToCart(ctx, userID, storeID, item)
		assert.Error(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("GetCartItems Success", func(t *testing.T) {
		item := model.CartItem{Barcode: "123", ProductID: "P1", Quantity: 1}
		data, _ := json.Marshal(item)
		mock.ExpectHGetAll(key).SetVal(map[string]string{"123": string(data)})

		items, err := repo.GetCartItems(ctx, userID, storeID)
		assert.NoError(t, err)
		require.Len(t, items, 1)
		assert.Equal(t, "P1", items[0].ProductID)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("GetCartItems Unmarshal Fail", func(t *testing.T) {
		mock.ExpectHGetAll(key).SetVal(map[string]string{"123": "invalid json"})
		items, err := repo.GetCartItems(ctx, userID, storeID)
		assert.NoError(t, err)
		assert.Len(t, items, 0)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("RemoveItem Success", func(t *testing.T) {
		item := model.CartItem{Barcode: "123", ProductID: "P1", Quantity: 1}
		data, _ := json.Marshal(item)
		mock.ExpectHGet(key, "123").SetVal(string(data))
		mock.ExpectHDel(key, "123").SetVal(1)

		res, err := repo.RemoveItem(ctx, userID, storeID, "123")
		assert.NoError(t, err)
		require.NotNil(t, res)
		assert.Equal(t, "P1", res.ProductID)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("RemoveItem HGet Generic Error", func(t *testing.T) {
		mock.ExpectHGet(key, "123").SetErr(errors.New("generic redis error"))
		_, err := repo.RemoveItem(ctx, userID, storeID, "123")
		assert.Error(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("RemoveItem Unmarshal Fail", func(t *testing.T) {
		mock.ExpectHGet(key, "123").SetVal("invalid json")
		_, err := repo.RemoveItem(ctx, userID, storeID, "123")
		assert.Error(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("RemoveItem HDel Fail", func(t *testing.T) {
		item := model.CartItem{Barcode: "123"}
		data, _ := json.Marshal(item)
		mock.ExpectHGet(key, "123").SetVal(string(data))
		mock.ExpectHDel(key, "123").SetErr(errors.New("hdel fail"))
		_, err := repo.RemoveItem(ctx, userID, storeID, "123")
		assert.Error(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("ClearCart Success", func(t *testing.T) {
		item := model.CartItem{Barcode: "123", ProductID: "P1", Quantity: 1}
		data, _ := json.Marshal(item)
		mock.ExpectHGetAll(key).SetVal(map[string]string{"123": string(data)})
		mock.ExpectDel(key).SetVal(1)

		items, err := repo.ClearCart(ctx, userID, storeID)
		assert.NoError(t, err)
		assert.Len(t, items, 1)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("ClearCart Partial Unmarshal Fail", func(t *testing.T) {
		mock.ExpectHGetAll(key).SetVal(map[string]string{"123": "invalid"})
		mock.ExpectDel(key).SetVal(1)
		items, err := repo.ClearCart(ctx, userID, storeID)
		assert.NoError(t, err)
		assert.Len(t, items, 0)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("ClearCart HGetAll Fail", func(t *testing.T) {
		mock.ExpectHGetAll(key).SetErr(errors.New("hgetall fail"))
		_, err := repo.ClearCart(ctx, userID, storeID)
		assert.Error(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("ClearCart Del Fail", func(t *testing.T) {
		mock.ExpectHGetAll(key).SetVal(map[string]string{"123": "{}"})
		mock.ExpectDel(key).SetErr(errors.New("del fail"))
		_, err := repo.ClearCart(ctx, userID, storeID)
		assert.Error(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("Locks", func(t *testing.T) {
		lockKey := "checkout:user1"
		mock.ExpectSetNX(lockKey, "1", 5*time.Minute).SetVal(true)
		ok, err := repo.AcquireCheckoutLock(ctx, userID, 5*time.Minute)
		assert.NoError(t, err)
		assert.True(t, ok)

		mock.ExpectDel(lockKey).SetVal(1)
		err = repo.ReleaseCheckoutLock(ctx, userID)
		assert.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("Stock", func(t *testing.T) {
		stockKey := "stock:store1:P1"
		mock.ExpectDecrBy(stockKey, 2).SetVal(8)
		val, err := repo.DecrementStock(ctx, storeID, "P1", 2)
		assert.NoError(t, err)
		assert.Equal(t, int64(8), val)

		mock.ExpectIncrBy(stockKey, 2).SetVal(10)
		val, err = repo.IncrementStock(ctx, storeID, "P1", 2)
		assert.NoError(t, err)
		assert.Equal(t, int64(10), val)

		mock.ExpectGet(stockKey).SetVal("10")
		val, err = repo.GetStock(ctx, storeID, "P1")
		assert.NoError(t, err)
		assert.Equal(t, int64(10), val)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("Redis Errors", func(t *testing.T) {
		mock.ExpectHGetAll(key).SetErr(errors.New("redis error"))
		_, err := repo.GetCartItems(ctx, userID, storeID)
		assert.Error(t, err)

		mock.ExpectHGet(key, "123").SetErr(redis.Nil)
		res, err := repo.RemoveItem(ctx, userID, storeID, "123")
		assert.NoError(t, err)
		assert.Nil(t, res)

		mock.ExpectGet("stock:store1:P1").SetErr(redis.Nil)
		val, err := repo.GetStock(ctx, storeID, "P1")
		assert.NoError(t, err)
		assert.Equal(t, int64(0), val)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestCartRepoPG(t *testing.T) {
	ctx := context.Background()

	t.Run("SnapshotCart Success", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()
		repo := &redisCartRepo{rdb: nil, db: mock}

		cart := &model.Cart{
			UserID:  "user1",
			StoreID: "store1",
			Items: []model.CartItem{
				{Barcode: "123", ProductID: "P1", ProductName: "Test", Quantity: 1, UnitPrice: 10, GSTAmount: 1, TotalPrice: 11},
			},
		}

		mock.ExpectBegin()
		mock.ExpectExec("INSERT INTO cart_items").
			WithArgs(pgxmock.AnyArg(), "user1", "store1", "123", "P1", "Test", 1, float64(10), float64(1), float64(11)).
			WillReturnResult(pgxmock.NewResult("INSERT", 1))
		mock.ExpectCommit()

		id, err := repo.SnapshotCart(ctx, cart)
		assert.NoError(t, err)
		assert.NotEmpty(t, id)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("SnapshotCart Empty Items", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()
		repo := &redisCartRepo{rdb: nil, db: mock}

		mock.ExpectBegin()
		mock.ExpectCommit()

		id, err := repo.SnapshotCart(ctx, &model.Cart{UserID: "u1", StoreID: "s1"})
		assert.NoError(t, err)
		assert.NotEmpty(t, id)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("SnapshotCart Begin Fail", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()
		repo := &redisCartRepo{rdb: nil, db: mock}

		mock.ExpectBegin().WillReturnError(errors.New("begin fail"))
		_, err = repo.SnapshotCart(ctx, &model.Cart{})
		assert.Error(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("SnapshotCart Exec Fail", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()
		repo := &redisCartRepo{rdb: nil, db: mock}

		mock.ExpectBegin()
		mock.ExpectExec("INSERT INTO cart_items").WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).WillReturnError(errors.New("exec fail"))
		mock.ExpectRollback()

		_, err = repo.SnapshotCart(ctx, &model.Cart{Items: []model.CartItem{{Barcode: "1"}}})
		assert.Error(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("SnapshotCart Commit Fail", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()
		repo := &redisCartRepo{rdb: nil, db: mock}

		mock.ExpectBegin()
		mock.ExpectExec("INSERT INTO cart_items").WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).WillReturnResult(pgxmock.NewResult("INSERT", 1))
		mock.ExpectCommit().WillReturnError(errors.New("commit fail"))

		_, err = repo.SnapshotCart(ctx, &model.Cart{Items: []model.CartItem{{Barcode: "1"}}})
		assert.Error(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}
