package rest

import (
	"fmt"

	"github.com/aikowocki/yandex-go-ext/internal/config"
)

func validateConfig(cfg config.ServerConfig) error {
	if cfg.Port < 0 || cfg.Port > 65535 {
		return fmt.Errorf("server port must be between 0 and 65535")
	}
	if cfg.ReadTimeout < 0 || cfg.WriteTimeout < 0 {
		return fmt.Errorf("server timeouts must not be negative")
	}
	if cfg.MaxUploadSize < 0 {
		return fmt.Errorf("server max upload size must not be negative")
	}
	if cfg.RateLimitPerSecond < 1 || cfg.RateLimitBurst < 1 {
		return fmt.Errorf("server rate limit settings must be positive")
	}
	return nil
}
