package db

import (
	"context"
	"fmt"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

// NewClickHouseConn creates a new clickhouse connection wrapper.
func NewClickHouseConn(addr, database, username, password string) (driver.Conn, error) {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{addr},
		Auth: clickhouse.Auth{
			Database: database,
			Username: username,
			Password: password,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("context: failed to connect to clickhouse: %w", err)
	}
	return conn, nil
}

// BatchInsert allows batch inserting rows efficiently into Clickhouse.
func BatchInsert(ctx context.Context, conn driver.Conn, query string, rows [][]interface{}) error {
	batch, err := conn.PrepareBatch(ctx, query)
	if err != nil {
		return fmt.Errorf("context: failed to prepare batch: %w", err)
	}
	for _, row := range rows {
		if err := batch.Append(row...); err != nil {
			return fmt.Errorf("context: failed to append to batch: %w", err)
		}
	}
	if err := batch.Send(); err != nil {
		return fmt.Errorf("context: failed to send batch: %w", err)
	}
	return nil
}
