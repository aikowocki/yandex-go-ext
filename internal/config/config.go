package config

import (
	"time"
)

// Config содержит настройки всех компонентов приложения.
type Config struct {
	Server        ServerConfig        `yaml:"server"`
	Database      DatabaseConfig      `yaml:"database"`
	S3            S3Config            `yaml:"s3"`
	Broker        BrokerConfig        `yaml:"broker"`
	Worker        WorkerConfig        `yaml:"worker"`
	Log           LogConfig           `yaml:"log"`
	Observability ObservabilityConfig `yaml:"observability"`
}

// ServerConfig содержит настройки HTTP-сервера.
type ServerConfig struct {
	Host               string        `yaml:"host" env:"SERVER_HOST" env-default:"0.0.0.0"`
	Port               int           `yaml:"port" env:"SERVER_PORT" env-default:"8080"`
	ReadTimeout        time.Duration `yaml:"read_timeout" env:"SERVER_READ_TIMEOUT" env-default:"30s"`
	WriteTimeout       time.Duration `yaml:"write_timeout" env:"SERVER_WRITE_TIMEOUT" env-default:"30s"`
	PprofAddress       string        `yaml:"pprof_address" env:"SERVER_PPROF_ADDRESS" env-default:""`
	MaxUploadSize      int64         `yaml:"max_upload_size" env:"SERVER_MAX_UPLOAD_SIZE" env-default:"10485760"`
	RateLimitPerSecond int           `yaml:"rate_limit_per_second" env:"SERVER_RATE_LIMIT_PER_SECOND" env-default:"10"`
	RateLimitBurst     int           `yaml:"rate_limit_burst" env:"SERVER_RATE_LIMIT_BURST" env-default:"20"`
}

// DatabaseConfig содержит настройки пула PostgreSQL.
type DatabaseConfig struct {
	DSN             string        `yaml:"dsn" env:"DATABASE_URL"`
	MaxConns        int32         `yaml:"max_conns" env:"DB_MAX_CONNS" env-default:"25"`
	MinConns        int32         `yaml:"min_conns" env:"DB_MIN_CONNS" env-default:"5"`
	MaxConnLifetime time.Duration `yaml:"max_conn_lifetime" env:"DB_MAX_CONN_LIFETIME" env-default:"1h"`
	MaxConnIdleTime time.Duration `yaml:"max_conn_idle_time" env:"DB_MAX_CONN_IDLE_TIME" env-default:"30m"`
}

// S3Config содержит настройки S3-совместимого object storage.
type S3Config struct {
	Endpoint        string `yaml:"endpoint" env:"S3_ENDPOINT"`
	AccessKeyID     string `yaml:"access_key_id" env:"S3_ACCESS_KEY_ID"`
	SecretAccessKey string `yaml:"secret_access_key" env:"S3_SECRET_ACCESS_KEY"`
	Bucket          string `yaml:"bucket" env:"S3_BUCKET" env-default:"avatars"`
	UseSSL          bool   `yaml:"use_ssl" env:"S3_USE_SSL" env-default:"false"`
	Region          string `yaml:"region" env:"S3_REGION" env-default:"us-east-1"`
}

// BrokerConfig содержит настройки выбранного брокера сообщений.
type BrokerConfig struct {
	Type     string         `yaml:"type" env:"BROKER_TYPE" env-default:"rabbitmq"`
	RabbitMQ RabbitMQConfig `yaml:"rabbitmq"`
	Kafka    KafkaConfig    `yaml:"kafka"`
}

// RabbitMQConfig содержит настройки RabbitMQ.
type RabbitMQConfig struct {
	URL         string        `yaml:"url" env:"RABBITMQ_URL" env-default:"amqp://gophprofile:gophprofile-dev@localhost:5672/"`
	Exchange    string        `yaml:"exchange" env:"RABBITMQ_EXCHANGE" env-default:"avatars"`
	RetryDelay  time.Duration `yaml:"retry_delay" env:"RABBITMQ_RETRY_DELAY" env-default:"30s"`
	MaxAttempts int           `yaml:"max_attempts" env:"RABBITMQ_MAX_ATTEMPTS" env-default:"5"`
	QueueType   string        `yaml:"queue_type" env:"RABBITMQ_QUEUE_TYPE" env-default:"classic"`
}

// KafkaConfig содержит настройки Kafka.
type KafkaConfig struct {
	Brokers           []string      `yaml:"brokers" env:"KAFKA_BROKERS" env-separator:"," env-default:"localhost:9092"`
	GroupID           string        `yaml:"group_id" env:"KAFKA_GROUP_ID" env-default:"gophprofile-workers"`
	MaxAttempts       int           `yaml:"max_attempts" env:"KAFKA_MAX_ATTEMPTS" env-default:"5"`
	DLQSuffix         string        `yaml:"dlq_suffix" env:"KAFKA_DLQ_SUFFIX" env-default:".dlq"`
	SessionTimeout    time.Duration `yaml:"session_timeout" env:"KAFKA_SESSION_TIMEOUT" env-default:"10s"`
	HeartbeatInterval time.Duration `yaml:"heartbeat_interval" env:"KAFKA_HEARTBEAT_INTERVAL" env-default:"3s"`
	FetchMinBytes     int32         `yaml:"fetch_min_bytes" env:"KAFKA_FETCH_MIN_BYTES" env-default:"1"`
	FetchMaxWait      time.Duration `yaml:"fetch_max_wait" env:"KAFKA_FETCH_MAX_WAIT" env-default:"250ms"`
}

// WorkerConfig содержит настройки фоновой обработки и очистки данных.
type WorkerConfig struct {
	Concurrency       int           `yaml:"concurrency" env:"WORKER_CONCURRENCY" env-default:"4"`
	RetentionPeriod   time.Duration `yaml:"retention_period" env:"WORKER_RETENTION_PERIOD" env-default:"720h"`
	CleanupInterval   time.Duration `yaml:"cleanup_interval" env:"WORKER_CLEANUP_INTERVAL" env-default:"24h"`
	ReconcileInterval time.Duration `yaml:"reconcile_interval" env:"WORKER_RECONCILE_INTERVAL" env-default:"168h"`
	OrphanGracePeriod time.Duration `yaml:"orphan_grace_period" env:"WORKER_ORPHAN_GRACE_PERIOD" env-default:"24h"`
	CleanupBatchSize  int           `yaml:"cleanup_batch_size" env:"WORKER_CLEANUP_BATCH_SIZE" env-default:"100"`
}

// LogConfig содержит настройки логирования.
type LogConfig struct {
	Backend string `yaml:"backend" env:"LOG_BACKEND" env-default:"slog"`
	Level   string `yaml:"level" env:"LOG_LEVEL" env-default:"info"`
	Format  string `yaml:"format" env:"LOG_FORMAT" env-default:"json"`
}

// ObservabilityConfig содержит настройки экспорта телеметрии OpenTelemetry.
type ObservabilityConfig struct {
	Enabled                bool    `yaml:"enabled" env:"OTEL_ENABLED" env-default:"false"`
	OTLPEndpoint           string  `yaml:"otlp_endpoint" env:"OTEL_EXPORTER_OTLP_ENDPOINT" env-default:"http://localhost:4317"`
	OTLPInsecure           bool    `yaml:"otlp_insecure" env:"OTEL_EXPORTER_OTLP_INSECURE" env-default:"true"`
	TraceSampleRatio       float64 `yaml:"trace_sample_ratio" env:"OTEL_TRACES_SAMPLER_ARG" env-default:"1.0"`
	ServiceVersion         string  `yaml:"service_version" env:"OTEL_SERVICE_VERSION" env-default:"unknown"`
	Environment            string  `yaml:"environment" env:"OTEL_ENVIRONMENT" env-default:"development"`
	PyroscopeEnabled       bool    `yaml:"pyroscope_enabled" env:"PYROSCOPE_ENABLED" env-default:"false"`
	PyroscopeServerAddress string  `yaml:"pyroscope_server_address" env:"PYROSCOPE_SERVER_ADDRESS" env-default:"http://localhost:4040"`
	PyroscopeAuthToken     string  `yaml:"-" env:"PYROSCOPE_AUTH_TOKEN"`
}
