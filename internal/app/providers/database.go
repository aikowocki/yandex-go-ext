package providers

import (
	"context"
	"fmt"

	"github.com/aikowocki/yandex-go-ext/internal/config"
	"github.com/aikowocki/yandex-go-ext/internal/infra/postgres"
)

// NewDatabase открывает пул PostgreSQL.
func NewDatabase(ctx context.Context, cfg *config.Config) (*postgres.DB, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}

	return postgres.NewPool(ctx, cfg.Database)
}
