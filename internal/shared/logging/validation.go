package logging

import (
	"fmt"
	"strings"

	"github.com/aikowocki/yandex-go-ext/internal/config"
)

func validateConfig(cfg config.LogConfig) error {
	format := strings.TrimSpace(cfg.Format)
	if format != "" && format != "json" && format != "console" {
		return fmt.Errorf("unknown log format %q: expected json or console", cfg.Format)
	}
	return nil
}
