package providers

import (
	"context"
	"fmt"

	"github.com/aikowocki/yandex-go-ext/internal/app/providers/components"
	"github.com/aikowocki/yandex-go-ext/internal/config"
	"github.com/aikowocki/yandex-go-ext/internal/transport/rest"
)

// NewREST создаёт REST-сервер и подключает проверки его зависимостей.
func NewREST(cfg *config.Config, avatarComponents *components.Avatar) (*rest.Server, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}
	if avatarComponents == nil {
		return nil, fmt.Errorf("avatar components are nil")
	}

	checks := rest.DependencyChecks{
		Database: avatarComponents.DB.Ping,
		Storage:  dependencyHealthCheck(avatarComponents.Storage),
		Broker:   dependencyHealthCheck(avatarComponents.Broker),
	}
	server, err := rest.NewServer(
		cfg.Server,
		avatarComponents.Avatar,
		avatarComponents.ThumbnailRepo,
		avatarComponents.Storage,
		checks,
	)
	if err != nil {
		return nil, err
	}
	return server, nil
}

func dependencyHealthCheck(dependency any) func(context.Context) error {
	return func(ctx context.Context) error {
		checker, ok := dependency.(interface{ HealthCheck(context.Context) error })
		if !ok {
			return fmt.Errorf("dependency health check is not supported")
		}
		return checker.HealthCheck(ctx)
	}
}
