package app

import (
	"context"
	"fmt"

	"github.com/aikowocki/yandex-go-ext/internal/app/providers"
	"github.com/aikowocki/yandex-go-ext/internal/app/providers/components"
	"github.com/aikowocki/yandex-go-ext/internal/shared/logging"
)

// New собирает зависимости приложения.
func New(ctx context.Context) (*Container, error) {
	cfg, err := providers.NewConfig()
	if err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}

	logger, err := logging.New(cfg.Log)
	if err != nil {
		return nil, fmt.Errorf("logger: %w", err)
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
		restoreLogger: restoreLogger,
	}
	cleanupLogger = false
	return container, nil
}
