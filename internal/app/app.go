package app

import (
	"context"
	"fmt"
	"time"

	"github.com/aikowocki/yandex-go-ext/internal/app/providers"
	"github.com/aikowocki/yandex-go-ext/internal/app/providers/components"
	"github.com/aikowocki/yandex-go-ext/internal/infra/observability"
	"github.com/aikowocki/yandex-go-ext/internal/shared/logging"
)

// New собирает зависимости приложения.
func New(ctx context.Context, serviceName string) (*Container, error) {
	cfg, err := providers.NewConfig()
	if err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}

	telemetry, err := observability.New(ctx, cfg.Observability, serviceName)
	if err != nil {
		return nil, fmt.Errorf("observability: %w", err)
	}
	telemetry.SetupGlobals()

	logger, err := logging.New(cfg.Log)
	if err != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = telemetry.Shutdown(shutdownCtx)
		cancel()
		return nil, fmt.Errorf("logger: %w", err)
	}
	// Добавляем в логи информацию о сервисе и окружении.
	logger = logger.With(
		logging.String("service.name", serviceName),
		logging.String("service.version", cfg.Observability.ServiceVersion),
		logging.String("deployment.environment.name", cfg.Observability.Environment),
	)
	if otelLogger := telemetry.Logger("github.com/aikowocki/yandex-go-ext/internal/app"); otelLogger != nil {
		logger = logging.WithOTelLogger(logger, otelLogger)
	}
	restoreLogger := logging.Install(logger)
	appLogger := logging.ComponentLogger(logger, "app")
	storageLogger := logging.ComponentLogger(logger, "storage")
	brokerLogger := logging.ComponentLogger(logger, "broker")
	avatarLogger := logging.ComponentLogger(logger, "avatar")
	workerLogger := logging.ComponentLogger(logger, "worker")
	restLogger := logging.ComponentLogger(logger, "rest")
	cleanupLogger := true
	defer func() {
		if cleanupLogger {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			_ = telemetry.Shutdown(shutdownCtx)
			cancel()
			restoreLogger()
			_ = logger.Sync()
		}
	}()

	db, err := providers.NewDatabase(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("database: %w", err)
	}

	objectStore, err := providers.NewObjectStore(ctx, cfg, storageLogger)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("object store: %w", err)
	}

	messageBroker, err := providers.NewBroker(ctx, cfg, brokerLogger)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("broker: %w", err)
	}

	avatarComponents, err := components.NewAvatar(db, objectStore, messageBroker, avatarLogger, workerLogger)
	if err != nil {
		_ = messageBroker.Close()
		db.Close()
		return nil, fmt.Errorf("avatar components: %w", err)
	}

	server, err := providers.NewREST(cfg, avatarComponents, restLogger)
	if err != nil {
		_ = messageBroker.Close()
		db.Close()
		return nil, fmt.Errorf("REST transport: %w", err)
	}

	container := &Container{
		Config:        cfg,
		DB:            db,
		Avatar:        avatarComponents,
		Broker:        messageBroker,
		Worker:        avatarComponents.Worker,
		Server:        server,
		logger:        appLogger,
		telemetry:     telemetry,
		restoreLogger: restoreLogger,
	}
	cleanupLogger = false
	return container, nil
}
