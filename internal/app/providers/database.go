package providers

import (
	"context"
	"fmt"

	"github.com/aikowocki/yandex-go-ext/internal/config"
	"github.com/aikowocki/yandex-go-ext/internal/infra/postgres"
	"github.com/aikowocki/yandex-go-ext/internal/infra/postgres/migrations"
)

// NewDatabase открывает пул PostgreSQL и применяет ожидающие миграции.
func NewDatabase(ctx context.Context, cfg *config.Config) (*postgres.DB, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}

	db, err := postgres.NewPool(ctx, cfg.Database)
	if err != nil {
		return nil, err
	}
	if err := migrations.Up(cfg.Database.DSN); err != nil {
		db.Close()
		return nil, fmt.Errorf("run database migrations: %w", err)
	}
	return db, nil
}
