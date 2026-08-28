package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ilyakaznacheev/cleanenv"
)

// Load загружает конфигурацию из файла и переменных окружения.
func Load() (*Config, error) {
	path := os.Getenv("CONFIG_FILE")
	if path == "" {
		env := os.Getenv("ENV")
		if env == "" {
			env = "dev"
		}
		path = filepath.Join("configs", "config."+env+".yaml")
	}

	var cfg Config
	if err := cleanenv.ReadConfig(path, &cfg); err != nil {
		if !errors.Is(err, os.ErrNotExist) && !os.IsNotExist(err) {
			return nil, fmt.Errorf("read config %q: %w", path, err)
		}
	}

	if err := readEnvironment(&cfg); err != nil {
		return nil, fmt.Errorf("read environment: %w", err)
	}
	return &cfg, nil
}

func readEnvironment(cfg *Config) error {
	aliases := map[string]string{
		"GOPHPROFILE_SERVER_HOST":                     "SERVER_HOST",
		"GOPHPROFILE_SERVER_PORT":                     "SERVER_PORT",
		"GOPHPROFILE_SERVER_READ_TIMEOUT":             "SERVER_READ_TIMEOUT",
		"GOPHPROFILE_SERVER_WRITE_TIMEOUT":            "SERVER_WRITE_TIMEOUT",
		"GOPHPROFILE_SERVER_MAX_UPLOAD_SIZE":          "SERVER_MAX_UPLOAD_SIZE",
		"GOPHPROFILE_SERVER_RATE_LIMIT_PER_SECOND":    "SERVER_RATE_LIMIT_PER_SECOND",
		"GOPHPROFILE_SERVER_RATE_LIMIT_BURST":         "SERVER_RATE_LIMIT_BURST",
		"GOPHPROFILE_DATABASE_DSN":                    "DATABASE_URL",
		"GOPHPROFILE_DATABASE_MAX_CONNS":              "DB_MAX_CONNS",
		"GOPHPROFILE_DATABASE_MIN_CONNS":              "DB_MIN_CONNS",
		"GOPHPROFILE_DATABASE_MAX_CONN_LIFETIME":      "DB_MAX_CONN_LIFETIME",
		"GOPHPROFILE_DATABASE_MAX_CONN_IDLE_TIME":     "DB_MAX_CONN_IDLE_TIME",
		"GOPHPROFILE_S3_ENDPOINT":                     "S3_ENDPOINT",
		"GOPHPROFILE_S3_ACCESS_KEY_ID":                "S3_ACCESS_KEY_ID",
		"GOPHPROFILE_S3_SECRET_ACCESS_KEY":            "S3_SECRET_ACCESS_KEY",
		"GOPHPROFILE_S3_BUCKET":                       "S3_BUCKET",
		"GOPHPROFILE_S3_USE_SSL":                      "S3_USE_SSL",
		"GOPHPROFILE_S3_REGION":                       "S3_REGION",
		"GOPHPROFILE_BROKER_TYPE":                     "BROKER_TYPE",
		"GOPHPROFILE_BROKER_RABBITMQ_URL":             "RABBITMQ_URL",
		"GOPHPROFILE_BROKER_RABBITMQ_EXCHANGE":        "RABBITMQ_EXCHANGE",
		"GOPHPROFILE_BROKER_RABBITMQ_RETRY_DELAY":     "RABBITMQ_RETRY_DELAY",
		"GOPHPROFILE_BROKER_RABBITMQ_MAX_ATTEMPTS":    "RABBITMQ_MAX_ATTEMPTS",
		"GOPHPROFILE_BROKER_RABBITMQ_QUEUE_TYPE":      "RABBITMQ_QUEUE_TYPE",
		"GOPHPROFILE_BROKER_KAFKA_BROKERS":            "KAFKA_BROKERS",
		"GOPHPROFILE_BROKER_KAFKA_GROUP_ID":           "KAFKA_GROUP_ID",
		"GOPHPROFILE_BROKER_KAFKA_MAX_ATTEMPTS":       "KAFKA_MAX_ATTEMPTS",
		"GOPHPROFILE_BROKER_KAFKA_DLQ_SUFFIX":         "KAFKA_DLQ_SUFFIX",
		"GOPHPROFILE_BROKER_KAFKA_SESSION_TIMEOUT":    "KAFKA_SESSION_TIMEOUT",
		"GOPHPROFILE_BROKER_KAFKA_HEARTBEAT_INTERVAL": "KAFKA_HEARTBEAT_INTERVAL",
		"GOPHPROFILE_BROKER_KAFKA_FETCH_MIN_BYTES":    "KAFKA_FETCH_MIN_BYTES",
		"GOPHPROFILE_BROKER_KAFKA_FETCH_MAX_WAIT":     "KAFKA_FETCH_MAX_WAIT",
		"GOPHPROFILE_WORKER_CONCURRENCY":              "WORKER_CONCURRENCY",
		"GOPHPROFILE_LOG_BACKEND":                     "LOG_BACKEND",
		"GOPHPROFILE_LOG_LEVEL":                       "LOG_LEVEL",
		"GOPHPROFILE_LOG_FORMAT":                      "LOG_FORMAT",
		"GOPHPROFILE_OBSERVABILITY_ENABLED":           "OTEL_ENABLED",
		"GOPHPROFILE_OTLP_ENDPOINT":                   "OTEL_EXPORTER_OTLP_ENDPOINT",
		"GOPHPROFILE_OTLP_INSECURE":                   "OTEL_EXPORTER_OTLP_INSECURE",
		"GOPHPROFILE_TRACE_SAMPLE_RATIO":              "OTEL_TRACES_SAMPLER_ARG",
		"GOPHPROFILE_SERVICE_VERSION":                 "OTEL_SERVICE_VERSION",
		"GOPHPROFILE_ENVIRONMENT":                     "OTEL_ENVIRONMENT",
	}

	restore := make([]func(), 0, len(aliases))
	for alias, canonical := range aliases {
		if _, exists := os.LookupEnv(canonical); exists {
			continue
		}
		value, exists := os.LookupEnv(alias)
		if !exists {
			continue
		}
		restore = append(restore, setTemporaryEnv(canonical, value))
	}
	defer func() {
		for i := len(restore) - 1; i >= 0; i-- {
			restore[i]()
		}
	}()

	return cleanenv.ReadEnv(cfg)
}

func setTemporaryEnv(key, value string) func() {
	previous, wasSet := os.LookupEnv(key)
	_ = os.Setenv(key, value)
	return func() {
		if wasSet {
			_ = os.Setenv(key, previous)
			return
		}
		_ = os.Unsetenv(key)
	}
}
