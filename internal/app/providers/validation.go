package providers

import (
	"fmt"

	"github.com/aikowocki/yandex-go-ext/internal/config"
)

func validateConfig(cfg *config.Config) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}
	if cfg.Worker.Concurrency < 1 {
		return fmt.Errorf("worker concurrency must be >= 1")
	}
	return nil
}
