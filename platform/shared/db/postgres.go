package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/zippyra/platform/shared/logger"
)

type queryData struct {
	Query string
	Args  []interface{}
	Start time.Time
}

type queryTracer struct{}

func (t *queryTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	return context.WithValue(ctx, "query_data", &queryData{
		Query: data.SQL,
		Args:  data.Args,
		Start: time.Now(),
	})
}

func (t *queryTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	qData, ok := ctx.Value("query_data").(*queryData)
	if !ok {
		return
	}
	duration := time.Since(qData.Start)
	if duration > 500*time.Millisecond {
		logger.Ctx(ctx).Warn().
			Dur("duration", duration).
			Str("query", qData.Query).
			Interface("args", qData.Args).
			Msg("slow query detected")
	}
}

// NewPostgresPool creates a new PostgreSQL connection pool with standard config.
func NewPostgresPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	return NewPostgresPoolWithRetry(ctx, dsn, 3)
}

// NewPostgresPoolWithRetry creates a pool with exponential backoff connection retries.
func NewPostgresPoolWithRetry(ctx context.Context, dsn string, maxRetries int) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("context: failed to parse pgxpool config: %w", err)
	}

	poolConfig.MaxConns = 25
	poolConfig.MinConns = 5
	poolConfig.MaxConnLifetime = 1 * time.Hour
	poolConfig.HealthCheckPeriod = 30 * time.Second
	poolConfig.ConnConfig.Tracer = &queryTracer{}

	var pool *pgxpool.Pool
	var lastErr error

	for i := 0; i < maxRetries; i++ {
		pool, lastErr = pgxpool.NewWithConfig(ctx, poolConfig)
		if lastErr == nil {
			lastErr = pool.Ping(ctx)
			if lastErr == nil {
				return pool, nil
			}
		}

		logger.Ctx(ctx).Warn().Err(lastErr).Int("attempt", i+1).Msg("postgres connection failed, retrying...")
		
		// Exponential backoff
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("context: context cancelled while retrying postgres connection: %w", ctx.Err())
		case <-time.After(time.Duration(1<<i) * time.Second):
		}
	}

	return nil, fmt.Errorf("context: failed to connect to postgres after %d retries: %w", maxRetries, lastErr)
}
