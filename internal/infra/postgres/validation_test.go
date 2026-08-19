package postgres

import (
	"testing"

	"github.com/aikowocki/yandex-go-ext/internal/config"
)

func TestValidateConfig(t *testing.T) {
	valid := config.DatabaseConfig{DSN: "postgres://user:pass@localhost/db", MaxConns: 5, MinConns: 1}
	tests := []struct {
		name  string
		cfg   config.DatabaseConfig
		valid bool
	}{
		{name: "empty dsn", cfg: config.DatabaseConfig{MaxConns: 1}},
		{name: "invalid max connections", cfg: config.DatabaseConfig{DSN: valid.DSN, MaxConns: 0}},
		{name: "negative min connections", cfg: config.DatabaseConfig{DSN: valid.DSN, MaxConns: 1, MinConns: -1}},
		{name: "min exceeds max", cfg: config.DatabaseConfig{DSN: valid.DSN, MaxConns: 1, MinConns: 2}},
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

func TestNewPoolRejectsInvalidPoolLimits(t *testing.T) {
	_, err := NewPool(nil, config.DatabaseConfig{DSN: "postgres://user:pass@localhost/db", MaxConns: 0})
	if err == nil {
		t.Fatal("NewPool accepted invalid pool limits")
	}
}
