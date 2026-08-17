package providers

import (
	"context"
	"fmt"

	"github.com/aikowocki/yandex-go-ext/internal/config"
	"github.com/aikowocki/yandex-go-ext/internal/infra/objectstore"
)

// NewObjectStore создаёт подключение к S3-совместимому хранилищу.
func NewObjectStore(ctx context.Context, cfg *config.Config) (*objectstore.MinIO, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}
	return objectstore.NewMinIO(ctx, cfg.S3)
}
