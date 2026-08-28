package kafka

import (
	"fmt"
	"strings"

	"github.com/aikowocki/yandex-go-ext/internal/config"
)

func validateConfig(cfg *config.KafkaConfig) error {
	if cfg == nil || len(cfg.Brokers) == 0 || strings.TrimSpace(cfg.GroupID) == "" {
		return fmt.Errorf("kafka configuration is incomplete")
	}
	if strings.TrimSpace(cfg.Brokers[0]) == "" {
		return fmt.Errorf("kafka brokers are required")
	}
	if cfg.MaxAttempts < 1 {
		return fmt.Errorf("kafka max attempts must be >= 1")
	}
	if strings.TrimSpace(cfg.DLQSuffix) == "" {
		return fmt.Errorf("kafka dlq suffix must not be empty")
	}
	if cfg.SessionTimeout <= 0 || cfg.HeartbeatInterval <= 0 || cfg.HeartbeatInterval >= cfg.SessionTimeout {
		return fmt.Errorf("kafka heartbeat interval must be positive and less than session timeout")
	}
	if cfg.FetchMinBytes < 1 || cfg.FetchMaxWait <= 0 {
		return fmt.Errorf("kafka fetch settings must be positive")
	}
	return nil
}
