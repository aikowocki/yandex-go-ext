package providers

import (
	"context"

	"github.com/aikowocki/yandex-go-ext/internal/config"
	"github.com/aikowocki/yandex-go-ext/internal/contracts"
	brokerinfra "github.com/aikowocki/yandex-go-ext/internal/infra/broker"
)

// NewBroker создаёт и настраивает брокер сообщений из конфигурации приложения.
func NewBroker(ctx context.Context, cfg *config.Config) (contracts.MessageBroker, error) {
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}
	return brokerinfra.NewBroker(ctx, &cfg.Broker, cfg.Worker.Concurrency)
}
