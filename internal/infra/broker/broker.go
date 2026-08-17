package broker

import (
	"context"
	"fmt"

	"github.com/aikowocki/yandex-go-ext/internal/config"
	"github.com/aikowocki/yandex-go-ext/internal/contracts"
	"github.com/aikowocki/yandex-go-ext/internal/infra/broker/kafka"
	"github.com/aikowocki/yandex-go-ext/internal/infra/broker/rabbitmq"
)

// NewBroker создаёт брокер выбранного типа.
func NewBroker(ctx context.Context, cfg *config.BrokerConfig, concurrency ...int) (contracts.MessageBroker, error) {
	if cfg == nil {
		return nil, fmt.Errorf("broker configuration is nil")
	}
	switch cfg.Type {
	case "rabbitmq":
		return rabbitmq.NewBroker(ctx, &cfg.RabbitMQ, concurrency...)
	case "kafka":
		return kafka.NewBroker(ctx, &cfg.Kafka)
	default:
		return nil, fmt.Errorf("unsupported broker type: %s", cfg.Type)
	}
}
