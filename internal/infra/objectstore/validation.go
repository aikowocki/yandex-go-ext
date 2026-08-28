package objectstore

import (
	"fmt"
	"strings"

	"github.com/aikowocki/yandex-go-ext/internal/config"
)

func validateConfig(cfg config.S3Config) error {
	if strings.TrimSpace(cfg.Endpoint) == "" {
		return fmt.Errorf("storage endpoint is required")
	}
	if strings.TrimSpace(cfg.AccessKeyID) == "" || strings.TrimSpace(cfg.SecretAccessKey) == "" {
		return fmt.Errorf("storage credentials are required")
	}
	if strings.TrimSpace(cfg.Bucket) == "" {
		return fmt.Errorf("storage bucket must not be empty")
	}
	return nil
}
