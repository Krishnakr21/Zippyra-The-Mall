package config

import (
	"strings"

	"github.com/zippyra/platform/shared/config"
)

type Config struct {
	AppEnv          string
	Port            string
	DatabaseURL     string   // required
	RedisURL        string   // required
	KafkaBrokers    []string // required
	JWTPublicKey    string   // required (verify exit tokens)
	MQTTBrokerURL   string   // optional: mqtt://localhost:1883
	MQTTUsername    string
	MQTTPassword    string
	JaegerEndpoint  string
	StoreServiceURL string
}

func Load() *Config {
	config.LoadEnv()

	kafkaBrokersStr := config.Get("KAFKA_BROKERS", "localhost:9092")
	kafkaBrokers := strings.Split(kafkaBrokersStr, ",")

	// Also support KAFKA_BROKER for single broker setups if split results in empty
	if len(kafkaBrokers) == 1 && kafkaBrokers[0] == "" {
		kafkaBrokers = []string{"localhost:9092"}
	}

	return &Config{
		AppEnv:          config.Get("APP_ENV", "local"),
		Port:            config.Get("PORT", ":8088"),
		DatabaseURL:     config.Get("DATABASE_URL", "postgres://zippyra:zippyra_local@localhost:5434/zippyra?sslmode=disable"),
		RedisURL:        config.Get("REDIS_URL", "redis://localhost:6379"),
		KafkaBrokers:    kafkaBrokers,
		JWTPublicKey:    config.Get("JWT_PUBLIC_KEY", ""),
		MQTTBrokerURL:   config.Get("MQTT_BROKER_URL", ""),
		MQTTUsername:    config.Get("MQTT_USERNAME", ""),
		MQTTPassword:    config.Get("MQTT_PASSWORD", ""),
		JaegerEndpoint:  config.Get("JAEGER_ENDPOINT", ""),
		StoreServiceURL: config.Get("STORE_SERVICE_URL", "http://localhost:8081"),
	}
}
