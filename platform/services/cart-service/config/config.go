package config

import (
	"time"

	"github.com/zippyra/platform/shared/config"
)

type Config struct {
	AppPort            string
	LogLevel           string
	Environment        string
	RedisAddr          string
	RedisPassword      string
	RedisDB            int
	DatabaseURL        string
	CatalogServiceURL  string
	KafkaBrokers       []string
	JWTSecret          string
	RedisTimeout       time.Duration
	DatabaseTimeout    time.Duration
}

func Load() *Config {
	config.LoadEnv()

	return &Config{
		AppPort:           config.Get("APP_PORT", ":8085"),
		LogLevel:          config.Get("LOG_LEVEL", "info"),
		Environment:       config.Get("ENVIRONMENT", "development"),
		RedisAddr:         config.Get("REDIS_ADDR", "localhost:6379"),
		RedisPassword:     config.Get("REDIS_PASSWORD", ""),
		RedisDB:           config.GetInt("REDIS_DB", 0),
		DatabaseURL:       config.Get("DATABASE_URL", "postgres://user:pass@localhost:5432/zippyra_cart?sslmode=disable"),
		CatalogServiceURL: config.Get("CATALOG_SERVICE_URL", "http://catalog-service:8081"),
		KafkaBrokers:      []string{config.Get("KAFKA_BROKERS", "localhost:9092")},
		JWTSecret:         config.Get("JWT_SECRET", "secret"),
		RedisTimeout:      config.GetDuration("REDIS_TIMEOUT", 100*time.Millisecond),
		DatabaseTimeout:   config.GetDuration("DATABASE_TIMEOUT", 5*time.Second),
	}
}
