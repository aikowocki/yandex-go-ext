package rabbitmq

import (
	"testing"
	"time"

	"github.com/aikowocki/yandex-go-ext/internal/config"
)

func TestValidateConfig(t *testing.T) {
	valid := func() *config.RabbitMQConfig {
		return &config.RabbitMQConfig{URL: "amqp://localhost", Exchange: "avatars", RetryDelay: time.Second, MaxAttempts: 3, QueueType: "classic"}
	}
	tests := []struct {
		name  string
		cfg   func() *config.RabbitMQConfig
		valid bool
	}{
		{name: "nil config", cfg: func() *config.RabbitMQConfig { return nil }},
		{name: "empty url", cfg: func() *config.RabbitMQConfig { cfg := valid(); cfg.URL = " "; return cfg }},
		{name: "empty exchange", cfg: func() *config.RabbitMQConfig { cfg := valid(); cfg.Exchange = " "; return cfg }},
		{name: "negative retry delay", cfg: func() *config.RabbitMQConfig { cfg := valid(); cfg.RetryDelay = -time.Second; return cfg }},
		{name: "negative max attempts", cfg: func() *config.RabbitMQConfig { cfg := valid(); cfg.MaxAttempts = -1; return cfg }},
		{name: "invalid queue type", cfg: func() *config.RabbitMQConfig { cfg := valid(); cfg.QueueType = "stream"; return cfg }},
		{name: "valid", cfg: valid, valid: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConfig(tt.cfg())
			if tt.valid && err != nil {
				t.Fatalf("validateConfig() error = %v", err)
			}
			if !tt.valid && err == nil {
				t.Fatal("validateConfig() accepted invalid config")
			}
		})
	}
}

func TestNewBrokerRejectsInvalidConfig(t *testing.T) {
	_, err := NewBroker(t.Context(), &config.RabbitMQConfig{URL: "amqp://localhost", Exchange: "avatars", RetryDelay: -time.Second}, nil)
	if err == nil {
		t.Fatal("NewBroker accepted invalid RabbitMQ config")
	}
}
