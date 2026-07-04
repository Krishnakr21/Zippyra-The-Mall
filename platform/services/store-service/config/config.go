package config

import (
	"fmt"
	"log"
	"os"
	"strings"
)

var fatalf = log.Fatal

var loadFromEnv = LoadFromEnv

// Config holds all store-service configuration loaded from environment variables.
type Config struct {
	AppEnv         string   // APP_ENV: local|pilot|production
	Port           string   // PORT: default 8081
	Version        string   // VERSION: default 1.0.0
	DatabaseURL    string   // DATABASE_URL: required
	RedisURL       string   // REDIS_URL: required
	KafkaBrokers   []string // KAFKA_BROKERS: comma-separated, required
	JWTPrivateKey  string   // JWT_PRIVATE_KEY: base64 Ed25519, required
	JWTPublicKey   string   // JWT_PUBLIC_KEY: base64 Ed25519, required
	AllowedOrigins []string // ALLOWED_ORIGINS: comma-separated
	JaegerEndpoint string   // JAEGER_ENDPOINT
	LogLevel       string   // LOG_LEVEL: debug|info|warn|error
	SentryDSN      string   // SENTRY_DSN: optional
}

// Load reads all configuration from environment variables.
// Panics with a clear message if any required field is missing.
func Load() *Config {
	cfg, err := loadFromEnv(os.Getenv)
	if err != nil {
		fatalf(err)
	}
	return cfg
}

func LoadFromEnv(getenv func(string) string) (*Config, error) {
	cfg := &Config{
		AppEnv:         getEnvDefaultFrom(getenv, "APP_ENV", "local"),
		Port:           getEnvDefaultFrom(getenv, "PORT", "8081"),
		Version:        getEnvDefaultFrom(getenv, "VERSION", "1.0.0"),
		DatabaseURL:    mustGetFrom(getenv, "DATABASE_URL"),
		RedisURL:       mustGetFrom(getenv, "REDIS_URL"),
		KafkaBrokers:   strings.Split(mustGetFrom(getenv, "KAFKA_BROKERS"), ","),
		JWTPrivateKey:  mustGetFrom(getenv, "JWT_PRIVATE_KEY"),
		JWTPublicKey:   mustGetFrom(getenv, "JWT_PUBLIC_KEY"),
		AllowedOrigins: strings.Split(getEnvDefaultFrom(getenv, "ALLOWED_ORIGINS", "*"), ","),
		JaegerEndpoint: getEnvDefaultFrom(getenv, "JAEGER_ENDPOINT", "localhost:4317"),
		LogLevel:       getEnvDefaultFrom(getenv, "LOG_LEVEL", "info"),
		SentryDSN:      getenv("SENTRY_DSN"),
	}

	if cfg.DatabaseURL == "" || cfg.RedisURL == "" || cfg.JWTPrivateKey == "" || cfg.JWTPublicKey == "" {
		return nil, fmt.Errorf("missing required config")
	}
	if len(cfg.KafkaBrokers) == 0 || cfg.KafkaBrokers[0] == "" {
		return nil, fmt.Errorf("missing required config: KAFKA_BROKERS")
	}

	return cfg, nil
}

func mustGetFrom(getenv func(string) string, key string) string {
	v := getenv(key)
	if v == "" {
		return ""
	}
	return v
}

func getEnvDefaultFrom(getenv func(string) string, key, def string) string {
	v := getenv(key)
	if v == "" {
		return def
	}
	return v
}
