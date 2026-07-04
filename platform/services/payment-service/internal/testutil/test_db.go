package testutil

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

func SetupDB() (*pgxpool.Pool, func()) {
	// Database URL for testing
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:postgres@localhost:5432/zippyra?sslmode=disable"
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		panic(fmt.Sprintf("failed to connect to test db: %v", err))
	}

	cleanup := func() {
		_, _ = pool.Exec(ctx, "TRUNCATE TABLE payment_outbox, payment_webhook_events, payments CASCADE")
		pool.Close()
	}

	// Initial cleanup
	_, _ = pool.Exec(ctx, "TRUNCATE TABLE payment_outbox, payment_webhook_events, payments CASCADE")

	return pool, cleanup
}
