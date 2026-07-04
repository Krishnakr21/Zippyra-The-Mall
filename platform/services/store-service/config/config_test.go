package config

import (
	"os"
	"strings"
	"testing"
)

func TestLoadConfig_Defaults(t *testing.T) {
	// Set required vars for Load() to not panic/fail
	os.Setenv("DATABASE_URL", "postgres://localhost")
	os.Setenv("REDIS_URL", "redis://localhost:6379")
	os.Setenv("KAFKA_BROKERS", "localhost")
	os.Setenv("JWT_PUBLIC_KEY", "pub")
	os.Setenv("JWT_PRIVATE_KEY", "priv")
	defer func() {
		os.Unsetenv("DATABASE_URL")
		os.Unsetenv("REDIS_URL")
		os.Unsetenv("KAFKA_BROKERS")
		os.Unsetenv("JWT_PUBLIC_KEY")
		os.Unsetenv("JWT_PRIVATE_KEY")
	}()

	// Clear relevant env vars
	os.Unsetenv("PORT")
	os.Unsetenv("APP_ENV")
	os.Unsetenv("VERSION")
	os.Unsetenv("ALLOWED_ORIGINS")
	os.Unsetenv("JAEGER_ENDPOINT")
	os.Unsetenv("LOG_LEVEL")
	os.Unsetenv("SENTRY_DSN")

	cfg := Load()

	if cfg.Port != "8081" {
		t.Errorf("expected default port 8081, got %s", cfg.Port)
	}

	if cfg.AppEnv != "local" {
		t.Errorf("expected default env local, got %s", cfg.AppEnv)
	}

	if cfg.Version != "1.0.0" {
		t.Errorf("expected default version 1.0.0, got %s", cfg.Version)
	}

	if len(cfg.AllowedOrigins) != 1 || cfg.AllowedOrigins[0] != "*" {
		t.Errorf("expected default allowed origins [*], got %v", cfg.AllowedOrigins)
	}

	if cfg.JaegerEndpoint != "localhost:4317" {
		t.Errorf("expected default jaeger endpoint localhost:4317, got %s", cfg.JaegerEndpoint)
	}

	if cfg.LogLevel != "info" {
		t.Errorf("expected default log level info, got %s", cfg.LogLevel)
	}
}

func TestLoadConfig_EnvOverrides(t *testing.T) {
	os.Setenv("PORT", "9090")
	os.Setenv("APP_ENV", "production")
	os.Setenv("VERSION", "2.0.0")
	os.Setenv("DATABASE_URL", "postgres://user:pass@host:5432/db")
	os.Setenv("REDIS_URL", "redis://localhost:6379")
	os.Setenv("KAFKA_BROKERS", "localhost:9092,broker2:9092")
	os.Setenv("JWT_PUBLIC_KEY", "pub")
	os.Setenv("JWT_PRIVATE_KEY", "priv")
	os.Setenv("ALLOWED_ORIGINS", "https://example.com,https://app.example.com")
	os.Setenv("JAEGER_ENDPOINT", "jaeger:4317")
	os.Setenv("LOG_LEVEL", "debug")
	os.Setenv("SENTRY_DSN", "https://sentry.example.com")
	defer func() {
		os.Unsetenv("PORT")
		os.Unsetenv("APP_ENV")
		os.Unsetenv("VERSION")
		os.Unsetenv("DATABASE_URL")
		os.Unsetenv("REDIS_URL")
		os.Unsetenv("KAFKA_BROKERS")
		os.Unsetenv("JWT_PUBLIC_KEY")
		os.Unsetenv("JWT_PRIVATE_KEY")
		os.Unsetenv("ALLOWED_ORIGINS")
		os.Unsetenv("JAEGER_ENDPOINT")
		os.Unsetenv("LOG_LEVEL")
		os.Unsetenv("SENTRY_DSN")
	}()

	cfg := Load()

	if cfg.Port != "9090" {
		t.Errorf("expected port 9090, got %s", cfg.Port)
	}

	if cfg.AppEnv != "production" {
		t.Errorf("expected env production, got %s", cfg.AppEnv)
	}

	if cfg.Version != "2.0.0" {
		t.Errorf("expected version 2.0.0, got %s", cfg.Version)
	}

	if cfg.DatabaseURL != "postgres://user:pass@host:5432/db" {
		t.Errorf("wrong database url: %s", cfg.DatabaseURL)
	}

	if len(cfg.KafkaBrokers) != 2 || cfg.KafkaBrokers[0] != "localhost:9092" {
		t.Errorf("expected kafka brokers [localhost:9092 broker2:9092], got %v", cfg.KafkaBrokers)
	}

	if len(cfg.AllowedOrigins) != 2 || cfg.AllowedOrigins[0] != "https://example.com" {
		t.Errorf("expected allowed origins [https://example.com https://app.example.com], got %v", cfg.AllowedOrigins)
	}

	if cfg.JaegerEndpoint != "jaeger:4317" {
		t.Errorf("expected jaeger endpoint jaeger:4317, got %s", cfg.JaegerEndpoint)
	}

	if cfg.LogLevel != "debug" {
		t.Errorf("expected log level debug, got %s", cfg.LogLevel)
	}

	if cfg.SentryDSN != "https://sentry.example.com" {
		t.Errorf("expected sentry dsn https://sentry.example.com, got %s", cfg.SentryDSN)
	}
}

func TestLoadFromEnv_MissingDatabaseURL(t *testing.T) {
	os.Unsetenv("DATABASE_URL")
	os.Setenv("REDIS_URL", "redis://localhost:6379")
	os.Setenv("KAFKA_BROKERS", "localhost")
	os.Setenv("JWT_PUBLIC_KEY", "pub")
	os.Setenv("JWT_PRIVATE_KEY", "priv")
	defer func() {
		os.Unsetenv("REDIS_URL")
		os.Unsetenv("KAFKA_BROKERS")
		os.Unsetenv("JWT_PUBLIC_KEY")
		os.Unsetenv("JWT_PRIVATE_KEY")
	}()

	_, err := LoadFromEnv(os.Getenv)
	if err == nil {
		t.Error("expected error for missing DATABASE_URL")
	}
	if !strings.Contains(err.Error(), "missing required config") {
		t.Errorf("expected 'missing required config' error, got: %v", err)
	}
}

func TestLoadFromEnv_MissingRedisURL(t *testing.T) {
	os.Setenv("DATABASE_URL", "postgres://localhost")
	os.Unsetenv("REDIS_URL")
	os.Setenv("KAFKA_BROKERS", "localhost")
	os.Setenv("JWT_PUBLIC_KEY", "pub")
	os.Setenv("JWT_PRIVATE_KEY", "priv")
	defer func() {
		os.Unsetenv("DATABASE_URL")
		os.Unsetenv("KAFKA_BROKERS")
		os.Unsetenv("JWT_PUBLIC_KEY")
		os.Unsetenv("JWT_PRIVATE_KEY")
	}()

	_, err := LoadFromEnv(os.Getenv)
	if err == nil {
		t.Error("expected error for missing REDIS_URL")
	}
}

func TestLoadFromEnv_MissingKafkaBrokers(t *testing.T) {
	os.Setenv("DATABASE_URL", "postgres://localhost")
	os.Setenv("REDIS_URL", "redis://localhost:6379")
	os.Unsetenv("KAFKA_BROKERS")
	os.Setenv("JWT_PUBLIC_KEY", "pub")
	os.Setenv("JWT_PRIVATE_KEY", "priv")
	defer func() {
		os.Unsetenv("DATABASE_URL")
		os.Unsetenv("REDIS_URL")
		os.Unsetenv("JWT_PUBLIC_KEY")
		os.Unsetenv("JWT_PRIVATE_KEY")
	}()

	_, err := LoadFromEnv(os.Getenv)
	if err == nil {
		t.Error("expected error for missing KAFKA_BROKERS")
	}
}

func TestLoadFromEnv_EmptyKafkaBrokers(t *testing.T) {
	os.Setenv("DATABASE_URL", "postgres://localhost")
	os.Setenv("REDIS_URL", "redis://localhost:6379")
	os.Setenv("KAFKA_BROKERS", "")
	os.Setenv("JWT_PUBLIC_KEY", "pub")
	os.Setenv("JWT_PRIVATE_KEY", "priv")
	defer func() {
		os.Unsetenv("DATABASE_URL")
		os.Unsetenv("REDIS_URL")
		os.Unsetenv("KAFKA_BROKERS")
		os.Unsetenv("JWT_PUBLIC_KEY")
		os.Unsetenv("JWT_PRIVATE_KEY")
	}()

	_, err := LoadFromEnv(os.Getenv)
	if err == nil {
		t.Error("expected error for empty KAFKA_BROKERS")
	}
}

func TestLoadFromEnv_MissingJWTPublicKey(t *testing.T) {
	os.Setenv("DATABASE_URL", "postgres://localhost")
	os.Setenv("REDIS_URL", "redis://localhost:6379")
	os.Setenv("KAFKA_BROKERS", "localhost")
	os.Unsetenv("JWT_PUBLIC_KEY")
	os.Setenv("JWT_PRIVATE_KEY", "priv")
	defer func() {
		os.Unsetenv("DATABASE_URL")
		os.Unsetenv("REDIS_URL")
		os.Unsetenv("KAFKA_BROKERS")
		os.Unsetenv("JWT_PRIVATE_KEY")
	}()

	_, err := LoadFromEnv(os.Getenv)
	if err == nil {
		t.Error("expected error for missing JWT_PUBLIC_KEY")
	}
}

func TestLoadFromEnv_MissingJWTPrivateKey(t *testing.T) {
	os.Setenv("DATABASE_URL", "postgres://localhost")
	os.Setenv("REDIS_URL", "redis://localhost:6379")
	os.Setenv("KAFKA_BROKERS", "localhost")
	os.Setenv("JWT_PUBLIC_KEY", "pub")
	os.Unsetenv("JWT_PRIVATE_KEY")
	defer func() {
		os.Unsetenv("DATABASE_URL")
		os.Unsetenv("REDIS_URL")
		os.Unsetenv("KAFKA_BROKERS")
		os.Unsetenv("JWT_PUBLIC_KEY")
	}()

	_, err := LoadFromEnv(os.Getenv)
	if err == nil {
		t.Error("expected error for missing JWT_PRIVATE_KEY")
	}
}

func TestLoad_FatalOnError(t *testing.T) {
	// Save original fatalf
	originalFatalf := fatalf
	defer func() { fatalf = originalFatalf }()

	fatalCalled := false
	fatalMsg := ""
	fatalf = func(v ...any) {
		fatalCalled = true
		if len(v) > 0 {
			fatalMsg = v[0].(error).Error()
		}
	}

	// Set up env to cause error
	os.Setenv("DATABASE_URL", "")
	os.Setenv("REDIS_URL", "")
	os.Setenv("KAFKA_BROKERS", "")
	os.Setenv("JWT_PUBLIC_KEY", "")
	os.Setenv("JWT_PRIVATE_KEY", "")
	defer func() {
		os.Unsetenv("DATABASE_URL")
		os.Unsetenv("REDIS_URL")
		os.Unsetenv("KAFKA_BROKERS")
		os.Unsetenv("JWT_PUBLIC_KEY")
		os.Unsetenv("JWT_PRIVATE_KEY")
	}()

	Load()

	if !fatalCalled {
		t.Error("expected fatalf to be called on config error")
	}
	if !strings.Contains(fatalMsg, "missing required config") {
		t.Errorf("expected 'missing required config' in fatal message, got: %s", fatalMsg)
	}
}
