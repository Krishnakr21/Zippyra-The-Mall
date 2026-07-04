package config

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLoad(t *testing.T) {
	t.Run("default config", func(t *testing.T) {
		os.Unsetenv("PORT")
		os.Unsetenv("DATABASE_URL")
		os.Unsetenv("DB_MAX_CONNS")

		cfg, err := Load()
		assert.NoError(t, err)
		assert.Equal(t, "8084", cfg.Server.Port)
	})

	t.Run("with environment variables", func(t *testing.T) {
		os.Setenv("PORT", "9090")
		os.Setenv("DATABASE_URL", "postgres://localhost:5432/db")
		defer os.Unsetenv("PORT")
		defer os.Unsetenv("DATABASE_URL")

		cfg, err := Load()
		assert.NoError(t, err)
		assert.Equal(t, "9090", cfg.Server.Port)
		assert.Equal(t, "postgres://localhost:5432/db", cfg.Database.URL)
	})

	t.Run("invalid environment variables", func(t *testing.T) {
		os.Setenv("DB_MAX_CONNS", "invalid")
		defer os.Unsetenv("DB_MAX_CONNS")

		cfg, err := Load()
		assert.NoError(t, err)
		assert.Equal(t, int32(25), cfg.Database.MaxConns) // Default value is 25
	})
}

func TestConfig_NewRedisClient(t *testing.T) {
	cfg := &Config{
		Redis: RedisConfig{
			Addr: "localhost:6379",
		},
	}
	client := cfg.NewRedisClient()
	assert.NotNil(t, client)
	assert.Equal(t, "localhost:6379", client.Options().Addr)
}

func TestConfig_NewDBPool(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		cfg := &Config{
			Database: DatabaseConfig{
				URL:             "postgres://user:pass@nonexistent.host:5432/db?sslmode=disable",
				MaxConns:        10,
				MinConns:        2,
				ConnMaxIdleTime: 5 * time.Minute,
				ConnMaxLifetime: 1 * time.Hour,
			},
		}
		pool, err := cfg.NewDBPool(context.Background())
		// This will fail to connect, but should pass parsing
		assert.Error(t, err)
		assert.Nil(t, pool)
		assert.Contains(t, err.Error(), "failed to create database pool")
	})

	t.Run("invalid URL", func(t *testing.T) {
		cfg := &Config{
			Database: DatabaseConfig{
				URL: "invalid://URL",
			},
		}
		pool, err := cfg.NewDBPool(context.Background())
		assert.Error(t, err)
		assert.Nil(t, pool)
	})

	t.Run("invalid max conns", func(t *testing.T) {
		cfg := &Config{
			Database: DatabaseConfig{
				URL:      "postgres://localhost:5432/db?sslmode=verify-full&sslrootcert=nonexistent",
				MaxConns: -1,
			},
		}
		pool, err := cfg.NewDBPool(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse database URL")
		assert.Nil(t, pool)
	})
}

func TestGetEnvHelpers(t *testing.T) {
	t.Run("getEnvInt valid", func(t *testing.T) {
		os.Setenv("TEST_INT", "123")
		defer os.Unsetenv("TEST_INT")
		assert.Equal(t, 123, getEnvInt("TEST_INT", 10))
	})

	t.Run("getEnvInt32 valid", func(t *testing.T) {
		os.Setenv("TEST_INT32", "123")
		defer os.Unsetenv("TEST_INT32")
		assert.Equal(t, int32(123), getEnvInt32("TEST_INT32", 10))
	})

	t.Run("getEnvDuration valid", func(t *testing.T) {
		os.Setenv("TEST_DUR", "30s")
		defer os.Unsetenv("TEST_DUR")
		assert.Equal(t, 30*time.Second, getEnvDuration("TEST_DUR", time.Second))
	})

	t.Run("getEnvSlice valid", func(t *testing.T) {
		os.Setenv("TEST_SLICE", "v1")
		defer os.Unsetenv("TEST_SLICE")
		assert.Equal(t, []string{"v1"}, getEnvSlice("TEST_SLICE", []string{"default"}))
	})

	t.Run("getEnvInt invalid", func(t *testing.T) {
		os.Setenv("TEST_INT_FAIL", "abc")
		defer os.Unsetenv("TEST_INT_FAIL")
		assert.Equal(t, 10, getEnvInt("TEST_INT_FAIL", 10))
	})

	t.Run("getEnvInt32 invalid", func(t *testing.T) {
		os.Setenv("TEST_INT32_FAIL", "abc")
		defer os.Unsetenv("TEST_INT32_FAIL")
		assert.Equal(t, int32(10), getEnvInt32("TEST_INT32_FAIL", 10))
	})

	t.Run("getEnvDuration invalid", func(t *testing.T) {
		os.Setenv("TEST_DUR_FAIL", "abc")
		defer os.Unsetenv("TEST_DUR_FAIL")
		assert.Equal(t, time.Second, getEnvDuration("TEST_DUR_FAIL", time.Second))
	})
}
