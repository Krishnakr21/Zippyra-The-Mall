package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/zippyra/platform/services/cart-service/internal/model"
)

type pgxDB interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

type CartRepository interface {
	AddToCart(ctx context.Context, userID, storeID string, item model.CartItem) error
	GetCartItems(ctx context.Context, userID, storeID string) ([]model.CartItem, error)
	RemoveItem(ctx context.Context, userID, storeID, barcode string) (*model.CartItem, error)
	ClearCart(ctx context.Context, userID, storeID string) ([]model.CartItem, error)
	AcquireCheckoutLock(ctx context.Context, userID string, ttl time.Duration) (bool, error)
	ReleaseCheckoutLock(ctx context.Context, userID string) error
	DecrementStock(ctx context.Context, storeID, productID string, quantity int) (int64, error)
	IncrementStock(ctx context.Context, storeID, productID string, quantity int) (int64, error)
	GetStock(ctx context.Context, storeID, productID string) (int64, error)
	SnapshotCart(ctx context.Context, cart *model.Cart) (string, error)
}

type redisCartRepo struct {
	rdb *redis.Client
	db  pgxDB
}

func NewCartRepository(rdb *redis.Client, db *pgxpool.Pool) CartRepository {
	return &redisCartRepo{rdb: rdb, db: db}
}

func (r *redisCartRepo) AddToCart(ctx context.Context, userID, storeID string, item model.CartItem) error {
	key := fmt.Sprintf("cart:%s:%s", userID, storeID)
	data, err := json.Marshal(item)
	if err != nil {
		return err
	}
	return r.rdb.HSet(ctx, key, item.Barcode, data).Err()
}

func (r *redisCartRepo) GetCartItems(ctx context.Context, userID, storeID string) ([]model.CartItem, error) {
	key := fmt.Sprintf("cart:%s:%s", userID, storeID)
	res, err := r.rdb.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, err
	}

	items := make([]model.CartItem, 0, len(res))
	for _, val := range res {
		var item model.CartItem
		if err := json.Unmarshal([]byte(val), &item); err != nil {
			continue
		}
		items = append(items, item)
	}
	return items, nil
}

func (r *redisCartRepo) RemoveItem(ctx context.Context, userID, storeID, barcode string) (*model.CartItem, error) {
	key := fmt.Sprintf("cart:%s:%s", userID, storeID)
	
	val, err := r.rdb.HGet(ctx, key, barcode).Result()
	if err == redis.Nil {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	var item model.CartItem
	if err := json.Unmarshal([]byte(val), &item); err != nil {
		return nil, err
	}

	if err := r.rdb.HDel(ctx, key, barcode).Err(); err != nil {
		return nil, err
	}

	return &item, nil
}

func (r *redisCartRepo) ClearCart(ctx context.Context, userID, storeID string) ([]model.CartItem, error) {
	items, err := r.rdb.HGetAll(ctx, fmt.Sprintf("cart:%s:%s", userID, storeID)).Result()
	if err != nil {
		return nil, err
	}

	cartItems := make([]model.CartItem, 0, len(items))
	for _, val := range items {
		var item model.CartItem
		if err := json.Unmarshal([]byte(val), &item); err == nil {
			cartItems = append(cartItems, item)
		}
	}

	err = r.rdb.Del(ctx, fmt.Sprintf("cart:%s:%s", userID, storeID)).Err()
	return cartItems, err
}

func (r *redisCartRepo) AcquireCheckoutLock(ctx context.Context, userID string, ttl time.Duration) (bool, error) {
	key := fmt.Sprintf("checkout:%s", userID)
	return r.rdb.SetNX(ctx, key, "1", ttl).Result()
}

func (r *redisCartRepo) ReleaseCheckoutLock(ctx context.Context, userID string) error {
	key := fmt.Sprintf("checkout:%s", userID)
	return r.rdb.Del(ctx, key).Err()
}

func (r *redisCartRepo) DecrementStock(ctx context.Context, storeID, productID string, quantity int) (int64, error) {
	key := fmt.Sprintf("stock:%s:%s", storeID, productID)
	return r.rdb.DecrBy(ctx, key, int64(quantity)).Result()
}

func (r *redisCartRepo) IncrementStock(ctx context.Context, storeID, productID string, quantity int) (int64, error) {
	key := fmt.Sprintf("stock:%s:%s", storeID, productID)
	return r.rdb.IncrBy(ctx, key, int64(quantity)).Result()
}

func (r *redisCartRepo) GetStock(ctx context.Context, storeID, productID string) (int64, error) {
	key := fmt.Sprintf("stock:%s:%s", storeID, productID)
	val, err := r.rdb.Get(ctx, key).Int64()
	if err == redis.Nil {
		return 0, nil
	}
	return val, err
}

func (r *redisCartRepo) SnapshotCart(ctx context.Context, cart *model.Cart) (string, error) {
	checkoutID := uuid.New().String()

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)

	// In a real scenario, we'd have a cart_snapshots table
	// The requirement mentions 'snapshot cart to PostgreSQL cart_items table'
	for _, item := range cart.Items {
		_, err = tx.Exec(ctx, `
			INSERT INTO cart_items (checkout_id, user_id, store_id, barcode, product_id, product_name, quantity, unit_price, gst_amount, total_price)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		`, checkoutID, cart.UserID, cart.StoreID, item.Barcode, item.ProductID, item.ProductName, item.Quantity, item.UnitPrice, item.GSTAmount, item.TotalPrice)
		if err != nil {
			return "", err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return "", err
	}

	return checkoutID, nil
}
