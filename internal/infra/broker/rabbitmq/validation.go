package rabbitmq

import (
	"fmt"
	"strings"

	"github.com/aikowocki/yandex-go-ext/internal/config"
)

func validateConfig(cfg *config.RabbitMQConfig) error {
	if cfg == nil || strings.TrimSpace(cfg.URL) == "" || strings.TrimSpace(cfg.Exchange) == "" {
		return fmt.Errorf("RabbitMQ configuration is incomplete")
	}
	if cfg.RetryDelay < 0 {
		return fmt.Errorf("RabbitMQ retry delay must not be negative")
	}
	if cfg.MaxAttempts < 0 {
		return fmt.Errorf("RabbitMQ max attempts must not be negative")
	}
	if cfg.QueueType != "" && cfg.QueueType != "classic" && cfg.QueueType != "quorum" {
		return fmt.Errorf("unsupported RabbitMQ queue type: %q", cfg.QueueType)
	}
	return nil
}
