package broker

import (
	"context"
	"fmt"

	"github.com/aikowocki/yandex-go-ext/internal/config"
	"github.com/aikowocki/yandex-go-ext/internal/contracts"
	"github.com/aikowocki/yandex-go-ext/internal/infra/broker/kafka"
	"github.com/aikowocki/yandex-go-ext/internal/infra/broker/rabbitmq"
	"github.com/aikowocki/yandex-go-ext/internal/shared/logging"
	"github.com/aikowocki/yandex-go-ext/internal/shared/resilience"
)

// NewBroker создаёт брокер выбранного типа.
func NewBroker(ctx context.Context, cfg *config.BrokerConfig, logger logging.Logger, concurrency ...int) (contracts.MessageBroker, error) {
	if cfg == nil {
		return nil, fmt.Errorf("broker configuration is nil")
	}
	switch cfg.Type {
	case "rabbitmq":
		broker, err := rabbitmq.NewBroker(ctx, &cfg.RabbitMQ, logger, concurrency...)
		if err != nil {
			return nil, err
		}
		return withCircuitBreaker(broker), nil
	case "kafka":
		broker, err := kafka.NewBroker(ctx, &cfg.Kafka, logger)
		if err != nil {
			return nil, err
		}
		return withCircuitBreaker(broker), nil
	default:
		return nil, fmt.Errorf("unsupported broker type: %s", cfg.Type)
	}
}

type circuitBreakerBroker struct {
	contracts.MessageBroker
	breaker *resilience.Breaker
}

func withCircuitBreaker(delegate contracts.MessageBroker) contracts.MessageBroker {
	return &circuitBreakerBroker{MessageBroker: delegate, breaker: resilience.New(resilience.Config{})}
}

func (b *circuitBreakerBroker) Publish(ctx context.Context, topic string, msg *contracts.Message) error {
	return b.breaker.Do(ctx, func(err error) bool {
		return err != nil
	}, func() error {
		return b.MessageBroker.Publish(ctx, topic, msg)
	})
}

func (b *circuitBreakerBroker) HealthCheck(ctx context.Context) error {
	checker, ok := b.MessageBroker.(interface{ HealthCheck(context.Context) error })
	if !ok {
		return fmt.Errorf("broker health check is not supported")
	}
	return checker.HealthCheck(ctx)
}
