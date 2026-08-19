package logging

import (
	"testing"

	"github.com/aikowocki/yandex-go-ext/internal/config"
)

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name  string
		cfg   config.LogConfig
		valid bool
	}{
		{name: "invalid format", cfg: config.LogConfig{Format: "text"}},
		{name: "empty format", cfg: config.LogConfig{}, valid: true},
		{name: "json format", cfg: config.LogConfig{Format: "json"}, valid: true},
		{name: "console format", cfg: config.LogConfig{Format: "console"}, valid: true},
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

func TestNewRejectsInvalidFormat(t *testing.T) {
	if _, err := New(config.LogConfig{Format: "text"}); err == nil {
		t.Fatal("New accepted invalid log format")
	}
}
