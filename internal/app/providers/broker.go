package providers

import (
	"context"
	"fmt"

	"github.com/aikowocki/yandex-go-ext/internal/config"
	"github.com/aikowocki/yandex-go-ext/internal/contracts"
	brokerinfra "github.com/aikowocki/yandex-go-ext/internal/infra/broker"
)

// NewBroker создаёт и настраивает брокер сообщений из конфигурации приложения.
func NewBroker(ctx context.Context, cfg *config.Config) (contracts.MessageBroker, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}
	return brokerinfra.NewBroker(ctx, &cfg.Broker, cfg.Worker.Concurrency)
}
