package config

import (
	"strings"
	"time"

	"github.com/zippyra/platform/shared/config"
)

type Config struct {
	AppPort               string
	LogLevel              string
	Environment           string
	DatabaseURL           string
	KafkaBrokers          []string
	JWTSecret             string
	RazorpayKeyID         string
	RazorpayKeySecret     string
	RazorpayWebhookSecret string
	CashfreeAppID         string
	CashfreeSecretKey     string
	DatabaseTimeout       time.Duration
}

func Load() *Config {
	config.LoadEnv()

	kafkaBrokersStr := config.Get("KAFKA_BROKERS", "localhost:9092")
	kafkaBrokers := strings.Split(kafkaBrokersStr, ",")

	return &Config{
		AppPort:               config.Get("APP_PORT", ":8086"),
		LogLevel:              config.Get("LOG_LEVEL", "info"),
		Environment:           config.Get("ENVIRONMENT", "development"),
		DatabaseURL:           config.Get("DATABASE_URL", "postgres://user:pass@localhost:5432/zippyra_payment?sslmode=disable"),
		KafkaBrokers:          kafkaBrokers,
		JWTSecret:             config.Get("JWT_SECRET", "secret"),
		RazorpayKeyID:         config.Get("RAZORPAY_KEY_ID", "rzp_test_123"),
		RazorpayKeySecret:     config.Get("RAZORPAY_KEY_SECRET", "secret"),
		RazorpayWebhookSecret: config.Get("RAZORPAY_WEBHOOK_SECRET", "webhook_secret"),
		CashfreeAppID:         config.Get("CASHFREE_APP_ID", "cf_test_123"),
		CashfreeSecretKey:     config.Get("CASHFREE_SECRET_KEY", "secret"),
		DatabaseTimeout:       config.GetDuration("DATABASE_TIMEOUT", 5*time.Second),
	}
}