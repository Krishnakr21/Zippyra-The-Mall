package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NewTimescalePool creates a pool and verifies TimescaleDB extension is available.
func NewTimescalePool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	pool, err := NewPostgresPool(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("context: failed to initialize timescale pool: %w", err)
	}

	var extName string
	err = pool.QueryRow(ctx, "SELECT extname FROM pg_extension WHERE extname = 'timescaledb'").Scan(&extName)
	if err != nil {
		return nil, fmt.Errorf("context: timescaledb extension not found: %w", err)
	}

	return pool, nil
}

// InsertTimeseriesEvent inserts a row into a timeseries table.
func InsertTimeseriesEvent(ctx context.Context, pool *pgxpool.Pool, table string, data map[string]interface{}) error {
	if len(data) == 0 {
		return nil
	}
	
	cols := make([]string, 0, len(data))
	args := make([]interface{}, 0, len(data))
	
	i := 1
	var colsString string
	var pString string
	
	for k, v := range data {
		cols = append(cols, k)
		args = append(args, v)
		
		if i > 1 {
			colsString += ", "
			pString += ", "
		}
		colsString += fmt.Sprintf("%q", k)
		pString += fmt.Sprintf("$%d", i)
		i++
	}

	query := fmt.Sprintf("INSERT INTO %q (%s) VALUES (%s)", table, colsString, pString)
	_, err := pool.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("context: failed to insert timeseries event: %w", err)
	}
	return nil
}
