package config

import (
	"testing"
	"time"
)

func TestLoadParsesKafkaSettings(t *testing.T) {
	path := writeConfigFile(t, `
broker:
  type: kafka
  kafka:
    brokers: [localhost:9092]
    group_id: workers
    max_attempts: 5
    dlq_suffix: .dlq
    session_timeout: 10s
    heartbeat_interval: 3s
    fetch_min_bytes: 1
    fetch_max_wait: 250ms
`)
	t.Setenv("CONFIG_FILE", path)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Broker.Type != "kafka" || len(cfg.Broker.Kafka.Brokers) != 1 {
		t.Fatalf("Kafka settings were not parsed: %+v", cfg.Broker)
	}
	if cfg.Broker.Kafka.SessionTimeout != 10*time.Second || cfg.Broker.Kafka.FetchMaxWait != 250*time.Millisecond {
		t.Fatalf("Kafka durations were not parsed: %+v", cfg.Broker.Kafka)
	}
}
