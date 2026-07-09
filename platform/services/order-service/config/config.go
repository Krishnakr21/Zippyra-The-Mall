package config

import (
	"strings"
	"time"

	"github.com/zippyra/platform/shared/config"
)

type Config struct {
	AppPort           string
	LogLevel          string
	Environment       string
	DatabaseURL       string
	RedisAddr         string
	RedisPassword     string
	RedisDB           int
	KafkaBrokers      []string
	PaymentServiceURL string
	DatabaseTimeout   time.Duration
}

func Load() *Config {
	config.LoadEnv()

	kafkaBrokersStr := config.Get("KAFKA_BROKERS", "localhost:9092")
	kafkaBrokers := strings.Split(kafkaBrokersStr, ",")

	return &Config{
		AppPort:           config.Get("APP_PORT", ":8087"),
		LogLevel:          config.Get("LOG_LEVEL", "info"),
		Environment:       config.Get("ENVIRONMENT", "development"),
		DatabaseURL:       config.Get("DATABASE_URL", "postgres://zippyra:zippyra_local@localhost:5434/zippyra?sslmode=disable"),
		RedisAddr:         config.Get("REDIS_ADDR", "localhost:6379"),
		RedisPassword:     config.Get("REDIS_PASSWORD", "zippyra_local"),
		RedisDB:           config.GetInt("REDIS_DB", 0),
		KafkaBrokers:      kafkaBrokers,
		PaymentServiceURL: config.Get("PAYMENT_SERVICE_URL", "http://localhost:8086"),
		DatabaseTimeout:   config.GetDuration("DATABASE_TIMEOUT", 5*time.Second),
	}
}
