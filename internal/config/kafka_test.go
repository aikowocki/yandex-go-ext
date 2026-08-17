package config

import (
	"strings"
	"testing"
	"time"
)

func TestKafkaConfigValidation(t *testing.T) {
	cfg := validTestConfig()
	cfg.Broker.Type = "kafka"
	cfg.Broker.Kafka = KafkaConfig{
		Brokers:           []string{"localhost:9092"},
		GroupID:           "gophprofile-workers",
		MaxAttempts:       5,
		DLQSuffix:         ".dlq",
		SessionTimeout:    10 * time.Second,
		HeartbeatInterval: 3 * time.Second,
		FetchMinBytes:     1,
		FetchMaxWait:      250 * time.Millisecond,
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("valid Kafka config rejected: %v", err)
	}

	cfg.Broker.Kafka.HeartbeatInterval = cfg.Broker.Kafka.SessionTimeout
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "heartbeat interval") {
		t.Fatalf("invalid Kafka heartbeat settings accepted: %v", err)
	}
}
