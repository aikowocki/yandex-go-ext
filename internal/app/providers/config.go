package providers

import "github.com/aikowocki/yandex-go-ext/internal/config"

// NewConfig загружает конфигурацию приложения.
func NewConfig() (*config.Config, error) {
	return config.Load()
}
