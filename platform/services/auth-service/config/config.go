package config

import (
	"fmt"
	"log"
	"os"
	"strings"
)

var fatalf = log.Fatal

var loadFromEnv = LoadFromEnv

// Config holds all auth-service configuration loaded from environment variables.
type Config struct {
	AppEnv           string   // APP_ENV: local|pilot|production
	Port             string   // PORT: default 8080
	Version          string   // VERSION: default 1.0.0
	DatabaseURL      string   // DATABASE_URL: required
	RedisURL         string   // REDIS_URL: required
	KafkaBrokers     []string // KAFKA_BROKERS: comma-separated, required
	JWTPrivateKey    string   // JWT_PRIVATE_KEY: base64 Ed25519, required
	JWTPublicKey     string   // JWT_PUBLIC_KEY: base64 Ed25519, required
	TwilioAccountSID string   // TWILIO_ACCOUNT_SID
	TwilioAuthToken  string   // TWILIO_AUTH_TOKEN
	TwilioServiceSID string   // TWILIO_SERVICE_SID
	OTPSalt          string   // OTP_SALT: required, min 32 chars
	AllowedOrigins   []string // ALLOWED_ORIGINS: comma-separated
	JaegerEndpoint   string   // JAEGER_ENDPOINT
	LogLevel         string   // LOG_LEVEL: debug|info|warn|error
	SentryDSN        string   // SENTRY_DSN: optional
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
		AppEnv:           getEnvDefaultFrom(getenv, "APP_ENV", "local"),
		Port:             getEnvDefaultFrom(getenv, "PORT", "8080"),
		Version:          getEnvDefaultFrom(getenv, "VERSION", "1.0.0"),
		DatabaseURL:      mustGetFrom(getenv, "DATABASE_URL"),
		RedisURL:         mustGetFrom(getenv, "REDIS_URL"),
		KafkaBrokers:     strings.Split(mustGetFrom(getenv, "KAFKA_BROKERS"), ","),
		JWTPrivateKey:    mustGetFrom(getenv, "JWT_PRIVATE_KEY"),
		JWTPublicKey:     mustGetFrom(getenv, "JWT_PUBLIC_KEY"),
		TwilioAccountSID: getenv("TWILIO_ACCOUNT_SID"),
		TwilioAuthToken:  getenv("TWILIO_AUTH_TOKEN"),
		TwilioServiceSID: getenv("TWILIO_SERVICE_SID"),
		OTPSalt:          mustGetFrom(getenv, "OTP_SALT"),
		AllowedOrigins:   strings.Split(getEnvDefaultFrom(getenv, "ALLOWED_ORIGINS", "*"), ","),
		JaegerEndpoint:   getEnvDefaultFrom(getenv, "JAEGER_ENDPOINT", "localhost:4317"),
		LogLevel:         getEnvDefaultFrom(getenv, "LOG_LEVEL", "info"),
		SentryDSN:        getenv("SENTRY_DSN"),
	}

	if cfg.DatabaseURL == "" || cfg.RedisURL == "" || cfg.OTPSalt == "" || cfg.JWTPrivateKey == "" || cfg.JWTPublicKey == "" {
		return nil, fmt.Errorf("missing required config")
	}
	if len(cfg.KafkaBrokers) == 0 || cfg.KafkaBrokers[0] == "" {
		return nil, fmt.Errorf("missing required config: KAFKA_BROKERS")
	}
	if len(cfg.OTPSalt) < 32 {
		return nil, fmt.Errorf("OTP_SALT must be at least 32 characters, got %d", len(cfg.OTPSalt))
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

func mustGet(key string) string {
	return mustGetFrom(os.Getenv, key)
}

func getEnvDefault(key, def string) string {
	return getEnvDefaultFrom(os.Getenv, key, def)
}
