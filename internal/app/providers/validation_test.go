package providers

import (
	"testing"

	"github.com/aikowocki/yandex-go-ext/internal/config"
)

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name  string
		cfg   *config.Config
		valid bool
	}{
		{name: "nil config"},
		{name: "invalid worker concurrency", cfg: &config.Config{Worker: config.WorkerConfig{Concurrency: 0}}},
		{name: "valid", cfg: &config.Config{Worker: config.WorkerConfig{Concurrency: 1}}, valid: true},
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
