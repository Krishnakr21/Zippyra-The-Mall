package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// MustGet returns the context of the environment variable or panics if missing.
func MustGet(key string) string {
	val := os.Getenv(key)
	if val == "" {
		panic(fmt.Sprintf("config: required environment variable %q is missing", key))
	}
	return val
}

// Get returns the value of the environment variable or a fallback if missing.
func Get(key, fallback string) string {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}
	return val
}

// GetInt returns the parsed integer value of the environment variable or a fallback.
func GetInt(key string, fallback int) int {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}
	i, err := strconv.Atoi(val)
	if err != nil {
		return fallback
	}
	return i
}

// GetBool returns the parsed boolean value of the environment variable or a fallback.
func GetBool(key string, fallback bool) bool {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}
	b, err := strconv.ParseBool(val)
	if err != nil {
		return fallback
	}
	return b
}

// GetDuration returns the parsed duration value of the environment variable or a fallback.
func GetDuration(key string, fallback time.Duration) time.Duration {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}
	d, err := time.ParseDuration(val)
	if err != nil {
		return fallback
	}
	return d
}
// LoadEnv loads the .env file from the current directory or parent directories if it exists.
// It does not error if the file is missing, allowing environment variables to be provided normally.
func LoadEnv() {
	_ = godotenv.Load()
}
