package main

import (
	"testing"
	"time"
)

func TestLoadServerConfig(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		t.Setenv("ADDR", "")
		t.Setenv("SHUTDOWN_TIMEOUT", "")
		config, err := loadServerConfig()
		if err != nil {
			t.Fatal(err)
		}
		if config.address != ":8080" || config.shutdownTimeout != 10*time.Second {
			t.Fatalf("config = %#v, want default address and timeout", config)
		}
	})

	t.Run("environment", func(t *testing.T) {
		t.Setenv("ADDR", ":3000")
		t.Setenv("SHUTDOWN_TIMEOUT", "7s")
		config, err := loadServerConfig()
		if err != nil {
			t.Fatal(err)
		}
		if config.address != ":3000" || config.shutdownTimeout != 7*time.Second {
			t.Fatalf("config = %#v, want environment values", config)
		}
	})

	for _, value := range []string{"nope", "0s", "-1s"} {
		t.Run("invalid_"+value, func(t *testing.T) {
			t.Setenv("SHUTDOWN_TIMEOUT", value)
			if _, err := loadServerConfig(); err == nil {
				t.Fatalf("SHUTDOWN_TIMEOUT=%q unexpectedly succeeded", value)
			}
		})
	}
}
