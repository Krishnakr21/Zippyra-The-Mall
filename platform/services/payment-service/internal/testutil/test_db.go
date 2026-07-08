package testutil

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

func loadEnv() {
	_ = godotenv.Load("../../../infra/docker/.env.local")
	_ = godotenv.Load("../../infra/docker/.env.local")
	_ = godotenv.Load("infra/docker/.env.local")
}

func SetupDB() (*pgxpool.Pool, func()) {
	loadEnv()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://zippyra:zippyra_local@localhost:5434/zippyra?sslmode=disable"
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		panic(fmt.Sprintf("failed to connect to test db: %v", err))
	}

	cleanup := func() {
		_, _ = pool.Exec(ctx, "TRUNCATE TABLE users, stores, orders, payments, payment_outbox, payment_webhook_events CASCADE")
		pool.Close()
	}

	// Initial cleanup
	_, _ = pool.Exec(ctx, "TRUNCATE TABLE users, stores, orders, payments, payment_outbox, payment_webhook_events CASCADE")

	return pool, cleanup
}

func SeedBaseData(ctx context.Context, pool *pgxpool.Pool) (uuid.UUID, uuid.UUID, uuid.UUID, error) {
	userID := uuid.New()
	storeID := uuid.New()
	orderID := uuid.New()
	sessionID := uuid.New()

	// 1. Insert User
	_, err := pool.Exec(ctx, `INSERT INTO users (id, phone, email, full_name) VALUES ($1, $2, $3, $4)`,
		userID, fmt.Sprintf("+91%010d", time.Now().UnixNano()%10000000000), "test@example.com", "Test User")
	if err != nil {
		return uuid.Nil, uuid.Nil, uuid.Nil, fmt.Errorf("seed user: %w", err)
	}

	// 2. Insert Store
	_, err = pool.Exec(ctx, `INSERT INTO stores (id, name, address, city, state, state_code, pincode, gstin) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		storeID, "Test Store", "123 Main St", "City", "State", "ST", "123456", fmt.Sprintf("12%010d1Z1", time.Now().UnixNano()%10000000000))
	if err != nil {
		return uuid.Nil, uuid.Nil, uuid.Nil, fmt.Errorf("seed store: %w", err)
	}

	// 3. Insert Order
	_, err = pool.Exec(ctx, `INSERT INTO orders (id, order_number, user_id, store_id, session_id, subtotal, total_amount) VALUES ($1, $2, $3, $4, $5, 100.0, 100.0)`,
		orderID, fmt.Sprintf("ORD-%d", time.Now().UnixNano()), userID, storeID, sessionID)
	if err != nil {
		return uuid.Nil, uuid.Nil, uuid.Nil, fmt.Errorf("seed order: %w", err)
	}

	return userID, storeID, orderID, nil
}
