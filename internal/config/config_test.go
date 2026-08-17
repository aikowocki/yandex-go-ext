package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func validTestConfig() Config {
	return Config{
		Server:   ServerConfig{Host: "127.0.0.1", Port: 8080, ReadTimeout: 30e9, WriteTimeout: 30e9, MaxUploadSize: 1024, RateLimitPerSecond: 10, RateLimitBurst: 20},
		Database: DatabaseConfig{DSN: "postgres://user:pass@localhost/db", MaxConns: 5, MinConns: 1},
		S3:       S3Config{Endpoint: "localhost:9000", AccessKeyID: "key", SecretAccessKey: "secret", Bucket: "avatars"},
		Broker:   BrokerConfig{Type: "rabbitmq", RabbitMQ: RabbitMQConfig{URL: "amqp://guest:guest@localhost:5672/", Exchange: "avatars"}},
		Worker:   WorkerConfig{Concurrency: 1},
		Log:      LogConfig{Backend: "slog", Level: "info", Format: "json"},
	}
}

func TestConfigValidate(t *testing.T) {
	cfg := validTestConfig()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}

	cfg.Broker.Type = "unsupported"
	if err := cfg.Validate(); err == nil {
		t.Fatal("unsupported broker accepted")
	}
}

func TestConfigValidateRequiresInfrastructureSettings(t *testing.T) {
	cfg := validTestConfig()
	cfg.Database.DSN = ""
	if err := cfg.Validate(); err == nil {
		t.Fatal("empty database DSN accepted")
	}
}
func validKafkaTestConfig() Config {
	cfg := validTestConfig()
	cfg.Broker = BrokerConfig{
		Type: "kafka",
		Kafka: KafkaConfig{
			Brokers:           []string{"localhost:9092"},
			GroupID:           "workers",
			MaxAttempts:       3,
			DLQSuffix:         ".dlq",
			SessionTimeout:    10 * time.Second,
			HeartbeatInterval: 3 * time.Second,
			FetchMinBytes:     1,
			FetchMaxWait:      250 * time.Millisecond,
		},
	}
	return cfg
}

func TestConfigValidateRejectsInvalidFields(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*Config)
	}{
		{"nil config", func(_ *Config) {}},
		{"empty s3 endpoint", func(c *Config) { c.S3.Endpoint = "" }},
		{"empty access key", func(c *Config) { c.S3.AccessKeyID = "" }},
		{"empty secret key", func(c *Config) { c.S3.SecretAccessKey = "" }},
		{"empty bucket", func(c *Config) { c.S3.Bucket = "" }},
		{"invalid port below range", func(c *Config) { c.Server.Port = 0 }},
		{"invalid port above range", func(c *Config) { c.Server.Port = 65536 }},
		{"invalid read timeout", func(c *Config) { c.Server.ReadTimeout = 0 }},
		{"invalid write timeout", func(c *Config) { c.Server.WriteTimeout = 0 }},
		{"invalid upload size", func(c *Config) { c.Server.MaxUploadSize = 0 }},
		{"invalid rate per second", func(c *Config) { c.Server.RateLimitPerSecond = 0 }},
		{"invalid rate burst", func(c *Config) { c.Server.RateLimitBurst = 0 }},
		{"invalid database max connections", func(c *Config) { c.Database.MaxConns = 0 }},
		{"invalid database min connections", func(c *Config) { c.Database.MinConns = -1 }},
		{"database min exceeds max", func(c *Config) { c.Database.MinConns = 6 }},
		{"unsupported broker", func(c *Config) { c.Broker.Type = "nats" }},
		{"empty rabbit url", func(c *Config) { c.Broker.RabbitMQ.URL = "" }},
		{"empty rabbit exchange", func(c *Config) { c.Broker.RabbitMQ.Exchange = "" }},
		{"negative rabbit retry delay", func(c *Config) { c.Broker.RabbitMQ.RetryDelay = -time.Second }},
		{"negative rabbit attempts", func(c *Config) { c.Broker.RabbitMQ.MaxAttempts = -1 }},
		{"invalid rabbit queue type", func(c *Config) { c.Broker.RabbitMQ.QueueType = "stream" }},
		{"invalid worker concurrency", func(c *Config) { c.Worker.Concurrency = 0 }},
		{"invalid log backend", func(c *Config) { c.Log.Backend = "zerolog" }},
		{"invalid log level", func(c *Config) { c.Log.Level = "trace" }},
		{"invalid log format", func(c *Config) { c.Log.Format = "text" }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validTestConfig()
			if tt.name == "nil config" {
				var nilConfig *Config
				if err := nilConfig.Validate(); err == nil {
					t.Fatal("nil config accepted")
				}
				return
			}
			tt.setup(&cfg)
			if err := cfg.Validate(); err == nil {
				t.Fatal("invalid config accepted")
			}
		})
	}
}

func TestKafkaConfigValidateRejectsInvalidFields(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*KafkaConfig)
	}{
		{"empty brokers", func(c *KafkaConfig) { c.Brokers = nil }},
		{"blank first broker", func(c *KafkaConfig) { c.Brokers = []string{"  "} }},
		{"empty group id", func(c *KafkaConfig) { c.GroupID = "" }},
		{"invalid max attempts", func(c *KafkaConfig) { c.MaxAttempts = 0 }},
		{"empty dlq suffix", func(c *KafkaConfig) { c.DLQSuffix = "" }},
		{"invalid session timeout", func(c *KafkaConfig) { c.SessionTimeout = 0 }},
		{"invalid heartbeat timeout", func(c *KafkaConfig) { c.HeartbeatInterval = 0 }},
		{"heartbeat not less than session", func(c *KafkaConfig) { c.HeartbeatInterval = c.SessionTimeout }},
		{"invalid fetch min bytes", func(c *KafkaConfig) { c.FetchMinBytes = 0 }},
		{"invalid fetch max wait", func(c *KafkaConfig) { c.FetchMaxWait = 0 }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validKafkaTestConfig()
			tt.setup(&cfg.Broker.Kafka)
			if err := cfg.Validate(); err == nil {
				t.Fatal("invalid Kafka config accepted")
			}
		})
	}
}

func TestConfigValidateAcceptsKafkaAndRabbitQueueVariants(t *testing.T) {
	for _, queueType := range []string{"", "classic", "quorum"} {
		cfg := validTestConfig()
		cfg.Broker.RabbitMQ.QueueType = queueType
		if err := cfg.Validate(); err != nil {
			t.Fatalf("rabbit queue type %q rejected: %v", queueType, err)
		}
	}
	cfg := validKafkaTestConfig()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("valid Kafka config rejected: %v", err)
	}
}

func TestIsValidLogLevel(t *testing.T) {
	for _, level := range []string{"debug", "info", "warn", "error"} {
		if !isValidLogLevel(level) {
			t.Errorf("log level %q rejected", level)
		}
	}
	if isValidLogLevel("trace") {
		t.Error("unsupported log level accepted")
	}
}
func validConfigYAML() string {
	return `
server:
  host: 127.0.0.1
  port: 8080
database:
  dsn: postgres://user:pass@localhost/db
s3:
  endpoint: localhost:9000
  access_key_id: key
  secret_access_key: secret
  bucket: avatars
broker:
  type: rabbitmq
  rabbitmq:
    url: amqp://guest:guest@localhost:5672/
    exchange: avatars
worker:
  concurrency: 1
log:
  backend: slog
  level: info
  format: json
`
}

func writeConfigFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func unsetEnv(t *testing.T, key string) {
	t.Helper()
	old, wasSet := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if wasSet {
			_ = os.Setenv(key, old)
		} else {
			_ = os.Unsetenv(key)
		}
	})
}

func TestLoadUsesExplicitFileAndEnvironmentOverrides(t *testing.T) {
	for _, key := range []string{
		"CONFIG_FILE", "ENV", "SERVER_PORT", "DATABASE_URL", "GOPHPROFILE_SERVER_PORT", "GOPHPROFILE_DATABASE_DSN",
	} {
		unsetEnv(t, key)
	}
	path := writeConfigFile(t, validConfigYAML())
	t.Setenv("CONFIG_FILE", path)
	t.Setenv("GOPHPROFILE_SERVER_PORT", "9090")
	t.Setenv("GOPHPROFILE_DATABASE_DSN", "postgres://alias/db")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Server.Port != 9090 || cfg.Database.DSN != "postgres://alias/db" {
		t.Fatalf("environment aliases were not applied: %+v", cfg)
	}
	if _, exists := os.LookupEnv("SERVER_PORT"); exists {
		t.Fatal("temporary canonical SERVER_PORT leaked")
	}
	if _, exists := os.LookupEnv("DATABASE_URL"); exists {
		t.Fatal("temporary canonical DATABASE_URL leaked")
	}
}

func TestLoadMissingFileCanUseEnvironment(t *testing.T) {
	unsetEnv(t, "ENV")
	unsetEnv(t, "CONFIG_FILE")
	for _, key := range []string{"DATABASE_URL", "S3_ENDPOINT", "S3_ACCESS_KEY_ID", "S3_SECRET_ACCESS_KEY"} {
		unsetEnv(t, key)
	}
	t.Setenv("CONFIG_FILE", filepath.Join(t.TempDir(), "missing.yaml"))
	t.Setenv("DATABASE_URL", "postgres://env/db")
	t.Setenv("S3_ENDPOINT", "localhost:9000")
	t.Setenv("S3_ACCESS_KEY_ID", "key")
	t.Setenv("S3_SECRET_ACCESS_KEY", "secret")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() with missing optional file error = %v", err)
	}
	if cfg.Database.DSN != "postgres://env/db" || cfg.S3.Endpoint != "localhost:9000" {
		t.Fatalf("environment config not loaded: %+v", cfg)
	}
}

func TestLoadReportsConfigAndEnvironmentErrors(t *testing.T) {
	t.Run("invalid yaml", func(t *testing.T) {
		unsetEnv(t, "CONFIG_FILE")
		path := writeConfigFile(t, "server: [")
		t.Setenv("CONFIG_FILE", path)
		_, err := Load()
		if err == nil || !strings.Contains(err.Error(), "read config") {
			t.Fatalf("Load() error = %v", err)
		}
	})
	t.Run("invalid environment", func(t *testing.T) {
		unsetEnv(t, "CONFIG_FILE")
		path := writeConfigFile(t, validConfigYAML())
		t.Setenv("CONFIG_FILE", path)
		t.Setenv("SERVER_PORT", "not-a-port")
		_, err := Load()
		if err == nil || !strings.Contains(err.Error(), "read config") {
			t.Fatalf("Load() error = %v", err)
		}
	})
	t.Run("validation error", func(t *testing.T) {
		unsetEnv(t, "CONFIG_FILE")
		path := writeConfigFile(t, strings.Replace(validConfigYAML(), "dsn: postgres://user:pass@localhost/db", "dsn: ''", 1))
		t.Setenv("CONFIG_FILE", path)
		_, err := Load()
		if err == nil || !strings.Contains(err.Error(), "validate config") {
			t.Fatalf("Load() error = %v", err)
		}
	})
}

func TestSetTemporaryEnvRestoresSetAndUnsetValues(t *testing.T) {
	t.Setenv("TEMP_CONFIG_TEST", "old")
	restore := setTemporaryEnv("TEMP_CONFIG_TEST", "new")
	if os.Getenv("TEMP_CONFIG_TEST") != "new" {
		t.Fatal("temporary value was not set")
	}
	restore()
	if os.Getenv("TEMP_CONFIG_TEST") != "old" {
		t.Fatal("previous value was not restored")
	}

	unsetEnv(t, "TEMP_CONFIG_TEST_UNSET")
	restore = setTemporaryEnv("TEMP_CONFIG_TEST_UNSET", "new")
	restore()
	if _, exists := os.LookupEnv("TEMP_CONFIG_TEST_UNSET"); exists {
		t.Fatal("previously unset value was not unset")
	}
}

func TestMustLoadPanicsOnInvalidConfig(t *testing.T) {
	t.Setenv("CONFIG_FILE", filepath.Join(t.TempDir(), "missing.yaml"))
	defer func() {
		if recover() == nil {
			t.Fatal("MustLoad did not panic")
		}
	}()

	MustLoad()
}
