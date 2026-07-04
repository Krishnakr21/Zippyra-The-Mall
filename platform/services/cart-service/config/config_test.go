package config

import (
	"os"
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	// 1. Test Defaults
	c := Load()
	if c.AppPort != ":8085" {
		t.Errorf("AppPort default = %s, want :8085", c.AppPort)
	}

	// 2. Test Overrides
	os.Setenv("APP_PORT", ":9000")
	os.Setenv("REDIS_DB", "5")
	os.Setenv("REDIS_TIMEOUT", "200ms")
	
	c2 := Load()
	if c2.AppPort != ":9000" {
		t.Errorf("AppPort override = %s, want :9000", c2.AppPort)
	}
	if c2.RedisDB != 5 {
		t.Errorf("RedisDB override = %d, want 5", c2.RedisDB)
	}
	if c2.RedisTimeout != 200*time.Millisecond {
		t.Errorf("RedisTimeout override = %v, want 200ms", c2.RedisTimeout)
	}

	// Clean up
	os.Unsetenv("APP_PORT")
	os.Unsetenv("REDIS_DB")
	os.Unsetenv("REDIS_TIMEOUT")
}
