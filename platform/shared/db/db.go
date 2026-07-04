package db

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"time"
)

// Factory creates a configured pgxpool
// Mandated: RDS Proxy compat, 20 max_conns, 3 retry
func NewPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}

	config.MaxConns = 20
	config.MaxConnIdleTime = 5 * time.Minute
	config.ConnConfig.ConnectTimeout = 5 * time.Second

	return pgxpool.NewWithConfig(ctx, config)
}
