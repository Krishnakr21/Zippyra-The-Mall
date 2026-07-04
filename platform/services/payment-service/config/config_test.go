package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoad(t *testing.T) {
	os.Setenv("APP_PORT", ":8086")
	os.Setenv("DATABASE_URL", "postgres://localhost:5432/test")
	os.Setenv("KAFKA_BROKERS", "localhost:9092")
	os.Setenv("RAZORPAY_KEY_ID", "k")
	os.Setenv("RAZORPAY_KEY_SECRET", "s")
	os.Setenv("RAZORPAY_WEBHOOK_SECRET", "w")
	os.Setenv("CASHFREE_APP_ID", "a")
	os.Setenv("CASHFREE_SECRET_KEY", "s")

	cfg := Load()
	assert.NotNil(t, cfg)
	assert.Equal(t, ":8086", cfg.AppPort)
	assert.Equal(t, "k", cfg.RazorpayKeyID)
}
