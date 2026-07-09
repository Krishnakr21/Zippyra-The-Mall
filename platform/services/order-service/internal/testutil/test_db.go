package testutil

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func LoadEnvLocal() {
	// Look up directories to find .env.local
	path := "../../../../infra/docker/.env.local"
	absPath, err := filepath.Abs(path)
	if err != nil {
		return
	}
	file, err := os.Open(absPath)
	if err != nil {
		// Fallback to searching closer
		path = "../../infra/docker/.env.local"
		absPath, _ = filepath.Abs(path)
		file, err = os.Open(absPath)
		if err != nil {
			return
		}
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "#") || !strings.Contains(line, "=") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			os.Setenv(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
		}
	}
}

func SetupDB() (*pgxpool.Pool, func()) {
	LoadEnvLocal()
	ctx := context.Background()

	// Connect to test database zippyra mapped on 5434
	dbURL := "postgres://zippyra:zippyra_local@localhost:5434/zippyra?sslmode=disable"
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		panic(fmt.Sprintf("failed to connect to test db: %v", err))
	}

	cleanup := func() {
		// Truncate tables to clean up
		_, _ = pool.Exec(ctx, "TRUNCATE TABLE users, stores, products, orders, order_items, exit_tokens, return_requests CASCADE")
		pool.Close()
	}

	// Initial clean
	_, _ = pool.Exec(ctx, "TRUNCATE TABLE users, stores, products, orders, order_items, exit_tokens, return_requests CASCADE")

	return pool, cleanup
}

func SeedBaseData(ctx context.Context, pool *pgxpool.Pool) (uuid.UUID, uuid.UUID, uuid.UUID, error) {
	userID := uuid.New()
	storeID := uuid.New()
	productID := uuid.New()

	// Seed User
	_, err := pool.Exec(ctx, `
		INSERT INTO users (id, phone, email, full_name)
		VALUES ($1, $2, $3, $4)`,
		userID, fmt.Sprintf("+91%010d", time.Now().UnixNano()%10000000000), "test@user.com", "Test User")
	if err != nil {
		return uuid.Nil, uuid.Nil, uuid.Nil, fmt.Errorf("seed user: %w", err)
	}

	// Seed Store
	_, err = pool.Exec(ctx, `
		INSERT INTO stores (id, name, address, city, state, state_code, pincode, gstin)
		VALUES ($1, 'Test Store', '123 Main St', 'City', 'State', 'ST', '123456', $2)`,
		storeID, fmt.Sprintf("12%010d1Z1", time.Now().UnixNano()%10000000000))
	if err != nil {
		return uuid.Nil, uuid.Nil, uuid.Nil, fmt.Errorf("seed store: %w", err)
	}

	// Seed Product
	_, err = pool.Exec(ctx, `
		INSERT INTO products (id, store_id, barcode, name, hsn_code, mrp, selling_price, gst_rate, is_active, is_returnable, stock_quantity)
		VALUES ($1, $2, '123456789012', 'Test Product', '123456', 100.00, 100.00, 18.00, true, true, 10)`,
		productID, storeID)
	if err != nil {
		return uuid.Nil, uuid.Nil, uuid.Nil, fmt.Errorf("seed product: %w", err)
	}

	return userID, storeID, productID, nil
}
