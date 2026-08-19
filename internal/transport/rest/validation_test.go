package rest

import (
	"testing"
	"time"

	"github.com/aikowocki/yandex-go-ext/internal/config"
)

func TestValidateConfig(t *testing.T) {
	valid := config.ServerConfig{Port: 8080, ReadTimeout: time.Second, WriteTimeout: time.Second, MaxUploadSize: 1024, RateLimitPerSecond: 10, RateLimitBurst: 20}
	tests := []struct {
		name  string
		cfg   config.ServerConfig
		valid bool
	}{
		{name: "negative port", cfg: config.ServerConfig{Port: -1}},
		{name: "port above range", cfg: config.ServerConfig{Port: 65536}},
		{name: "negative read timeout", cfg: config.ServerConfig{ReadTimeout: -time.Second}},
		{name: "negative write timeout", cfg: config.ServerConfig{WriteTimeout: -time.Second}},
		{name: "negative upload size", cfg: config.ServerConfig{MaxUploadSize: -1}},
		{name: "invalid rate per second", cfg: config.ServerConfig{RateLimitPerSecond: 0, RateLimitBurst: 1}},
		{name: "invalid rate burst", cfg: config.ServerConfig{RateLimitPerSecond: 1, RateLimitBurst: 0}},
		{name: "valid", cfg: valid, valid: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConfig(tt.cfg)
			if tt.valid && err != nil {
				t.Fatalf("validateConfig() error = %v", err)
			}
			if !tt.valid && err == nil {
				t.Fatal("validateConfig() accepted invalid config")
			}
		})
	}
}

func TestNewServerRejectsInvalidConfig(t *testing.T) {
	if _, err := NewServer(config.ServerConfig{RateLimitPerSecond: 0, RateLimitBurst: 1}, nil, nil, nil); err == nil {
		t.Fatal("NewServer accepted invalid rate limit config")
	}
}
