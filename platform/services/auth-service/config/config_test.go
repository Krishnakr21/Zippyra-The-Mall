package config

import (
	"errors"
	"os"
	"testing"
)

func TestLoadFromEnv_Success(t *testing.T) {
	getenv := func(k string) string {
		switch k {
		case "APP_ENV":
			return "local"
		case "PORT":
			return "8080"
		case "VERSION":
			return "1.0.0"
		case "DATABASE_URL":
			return "postgres://user:pass@localhost:5432/db"
		case "REDIS_URL":
			return "redis://localhost:6379/0"
		case "KAFKA_BROKERS":
			return "localhost:9092"
		case "JWT_PRIVATE_KEY":
			return "priv"
		case "JWT_PUBLIC_KEY":
			return "pub"
		case "OTP_SALT":
			return "test-salt-that-is-at-least-32-characters-long"
		case "ALLOWED_ORIGINS":
			return "*"
		case "JAEGER_ENDPOINT":
			return "localhost:4317"
		case "LOG_LEVEL":
			return "info"
		case "SENTRY_DSN":
			return ""
		default:
			return ""
		}
	}

	cfg, err := LoadFromEnv(getenv)
	if err != nil {
		t.Fatalf("expected success, got err=%v", err)
	}
	if cfg.DatabaseURL == "" || cfg.RedisURL == "" {
		t.Fatal("expected required fields")
	}
	if len(cfg.KafkaBrokers) != 1 || cfg.KafkaBrokers[0] != "localhost:9092" {
		t.Fatalf("unexpected brokers: %#v", cfg.KafkaBrokers)
	}
}

func TestLoadFromEnv_MissingRequired(t *testing.T) {
	getenv := func(k string) string {
		return ""
	}
	_, err := LoadFromEnv(getenv)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLoadFromEnv_SaltTooShort(t *testing.T) {
	getenv := func(k string) string {
		switch k {
		case "DATABASE_URL":
			return "postgres://user:pass@localhost:5432/db"
		case "REDIS_URL":
			return "redis://localhost:6379/0"
		case "KAFKA_BROKERS":
			return "localhost:9092"
		case "JWT_PRIVATE_KEY":
			return "priv"
		case "JWT_PUBLIC_KEY":
			return "pub"
		case "OTP_SALT":
			return "too-short"
		default:
			return ""
		}
	}
	_, err := LoadFromEnv(getenv)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLoadFromEnv_MissingKafkaBrokers(t *testing.T) {
	getenv := func(k string) string {
		switch k {
		case "DATABASE_URL":
			return "postgres://user:pass@localhost:5432/db"
		case "REDIS_URL":
			return "redis://localhost:6379/0"
		case "KAFKA_BROKERS":
			return ""
		case "JWT_PRIVATE_KEY":
			return "priv"
		case "JWT_PUBLIC_KEY":
			return "pub"
		case "OTP_SALT":
			return "test-salt-that-is-at-least-32-characters-long"
		default:
			return ""
		}
	}
	_, err := LoadFromEnv(getenv)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGetEnvDefaultFrom(t *testing.T) {
	getenv := func(k string) string {
		if k == "A" {
			return ""
		}
		if k == "B" {
			return "x"
		}
		return ""
	}
	if got := getEnvDefaultFrom(getenv, "A", "def"); got != "def" {
		t.Fatalf("expected def, got %q", got)
	}
	if got := getEnvDefaultFrom(getenv, "B", "def"); got != "x" {
		t.Fatalf("expected x, got %q", got)
	}
}

func TestMustGetFrom(t *testing.T) {
	getenv := func(k string) string {
		if k == "A" {
			return ""
		}
		if k == "B" {
			return "y"
		}
		return ""
	}
	if got := mustGetFrom(getenv, "A"); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
	if got := mustGetFrom(getenv, "B"); got != "y" {
		t.Fatalf("expected y, got %q", got)
	}
}

func TestLoad_UsesFatalfOnError(t *testing.T) {
	oldFatal := fatalf
	defer func() { fatalf = oldFatal }()
	oldLoader := loadFromEnv
	defer func() { loadFromEnv = oldLoader }()

	called := false
	fatalf = func(v ...any) {
		called = true
	}

	loadFromEnv = func(getenv func(string) string) (*Config, error) {
		return nil, errors.New("boom")
	}

	_ = Load()
	if !called {
		t.Fatal("expected fatalf to be called")
	}
}

func TestGetEnvDefault(t *testing.T) {
	_ = os.Unsetenv("CFG_TEST_A")
	_ = os.Setenv("CFG_TEST_B", "x")
	defer func() {
		_ = os.Unsetenv("CFG_TEST_A")
		_ = os.Unsetenv("CFG_TEST_B")
	}()

	if got := getEnvDefault("CFG_TEST_A", "def"); got != "def" {
		t.Fatalf("expected def, got %q", got)
	}
	if got := getEnvDefault("CFG_TEST_B", "def"); got != "x" {
		t.Fatalf("expected x, got %q", got)
	}
}

func TestMustGet(t *testing.T) {
	_ = os.Unsetenv("CFG_TEST_C")
	_ = os.Setenv("CFG_TEST_D", "y")
	defer func() {
		_ = os.Unsetenv("CFG_TEST_C")
		_ = os.Unsetenv("CFG_TEST_D")
	}()

	if got := mustGet("CFG_TEST_C"); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
	if got := mustGet("CFG_TEST_D"); got != "y" {
		t.Fatalf("expected y, got %q", got)
	}
}
