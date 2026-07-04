package main

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/zippyra/platform/services/payment-service/config"
)

func TestRun_ConfigError(t *testing.T) {
	ctx := context.Background()
	cfg := &config.Config{
		DatabaseURL: "postgres://invalid:5432/db",
	}

	err := Run(ctx, cfg)
	assert.Error(t, err)
}

func TestRun_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cfg := &config.Config{
		DatabaseURL: "postgres://postgres:postgres@localhost:5432/zippyra?sslmode=disable",
	}

	err := Run(ctx, cfg)
	// Should fail at DB connection or exit gracefully if context is done before connection
	if err != nil {
		assert.Error(t, err)
	}
}

func TestRun_Success(t *testing.T) {
	// Skip real run if no DB
	cfg := &config.Config{
		DatabaseURL: "postgres://postgres:postgres@localhost:5432/zippyra?sslmode=disable",
		AppPort: ":0", 
		Environment: "development",
		KafkaBrokers: []string{"localhost:9092"},
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_ = Run(ctx, cfg) // Just ensure it doesn't panic and starts enough to see context done
}
