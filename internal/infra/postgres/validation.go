package postgres

import (
	"fmt"
	"strings"

	"github.com/aikowocki/yandex-go-ext/internal/config"
)

func validateConfig(cfg config.DatabaseConfig) error {
	if strings.TrimSpace(cfg.DSN) == "" {
		return fmt.Errorf("database dsn is required")
	}
	if cfg.MaxConns < 1 || cfg.MinConns < 0 || cfg.MinConns > cfg.MaxConns {
		return fmt.Errorf("database pool limits are invalid")
	}
	return nil
}
