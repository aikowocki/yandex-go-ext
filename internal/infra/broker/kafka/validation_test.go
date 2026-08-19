package kafka

import (
	"testing"
	"time"

	"github.com/aikowocki/yandex-go-ext/internal/config"
)

func TestValidateConfig(t *testing.T) {
	valid := func() *config.KafkaConfig {
		return &config.KafkaConfig{
			Brokers:           []string{"localhost:9092"},
			GroupID:           "workers",
			MaxAttempts:       5,
			DLQSuffix:         ".dlq",
			SessionTimeout:    10 * time.Second,
			HeartbeatInterval: 3 * time.Second,
			FetchMinBytes:     1,
			FetchMaxWait:      250 * time.Millisecond,
		}
	}
	tests := []struct {
		name  string
		cfg   func() *config.KafkaConfig
		valid bool
	}{
		{name: "nil config", cfg: func() *config.KafkaConfig { return nil }},
		{name: "empty brokers", cfg: func() *config.KafkaConfig { cfg := valid(); cfg.Brokers = nil; return cfg }},
		{name: "blank broker", cfg: func() *config.KafkaConfig { cfg := valid(); cfg.Brokers = []string{" "}; return cfg }},
		{name: "empty group id", cfg: func() *config.KafkaConfig { cfg := valid(); cfg.GroupID = " "; return cfg }},
		{name: "invalid max attempts", cfg: func() *config.KafkaConfig { cfg := valid(); cfg.MaxAttempts = 0; return cfg }},
		{name: "empty dlq suffix", cfg: func() *config.KafkaConfig { cfg := valid(); cfg.DLQSuffix = " "; return cfg }},
		{name: "invalid session timeout", cfg: func() *config.KafkaConfig { cfg := valid(); cfg.SessionTimeout = 0; return cfg }},
		{name: "invalid heartbeat timeout", cfg: func() *config.KafkaConfig { cfg := valid(); cfg.HeartbeatInterval = 0; return cfg }},
		{name: "heartbeat not less than session", cfg: func() *config.KafkaConfig { cfg := valid(); cfg.HeartbeatInterval = cfg.SessionTimeout; return cfg }},
		{name: "invalid fetch min bytes", cfg: func() *config.KafkaConfig { cfg := valid(); cfg.FetchMinBytes = 0; return cfg }},
		{name: "invalid fetch max wait", cfg: func() *config.KafkaConfig { cfg := valid(); cfg.FetchMaxWait = 0; return cfg }},
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
	_, err := NewBroker(t.Context(), &config.KafkaConfig{Brokers: []string{"localhost:9092"}, GroupID: "workers"})
	if err == nil {
		t.Fatal("NewBroker accepted invalid Kafka config")
	}
}
