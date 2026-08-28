package providers

import (
	"context"

	"github.com/aikowocki/yandex-go-ext/internal/config"
	"github.com/aikowocki/yandex-go-ext/internal/contracts"
	brokerinfra "github.com/aikowocki/yandex-go-ext/internal/infra/broker"
	"github.com/aikowocki/yandex-go-ext/internal/shared/logging"
)

// NewBroker создаёт и настраивает брокер сообщений из конфигурации приложения.
func NewBroker(ctx context.Context, cfg *config.Config, logger logging.Logger) (contracts.MessageBroker, error) {
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}
	return brokerinfra.NewBroker(ctx, &cfg.Broker, logger, cfg.Worker.Concurrency)
}
