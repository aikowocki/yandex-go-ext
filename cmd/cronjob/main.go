package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/aikowocki/yandex-go-ext/internal/app/providers"
	"github.com/aikowocki/yandex-go-ext/internal/cronjob"
	"github.com/aikowocki/yandex-go-ext/internal/infra/postgres"
	"github.com/aikowocki/yandex-go-ext/internal/infra/postgres/gen"
	"github.com/aikowocki/yandex-go-ext/internal/shared/logging"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	if len(os.Args) != 2 {
		return fmt.Errorf("usage: cronjob <retention|reconcile>")
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := providers.NewConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	logger, err := logging.New(cfg.Log)
	if err != nil {
		return fmt.Errorf("create logger: %w", err)
	}
	restoreLogger := logging.Install(logger)
	defer func() {
		restoreLogger()
		_ = logger.Sync()
	}()

	db, err := providers.NewDatabase(ctx, cfg)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	storage, err := providers.NewObjectStore(ctx, cfg)
	if err != nil {
		return fmt.Errorf("open object storage: %w", err)
	}
	queries := gen.New(db)
	avatarRepo := postgres.NewAvatarRepository(queries, db)
	thumbnailRepo := postgres.NewThumbnailRepository(queries)
	blobRepo := postgres.NewBlobRepository(db)
	runner := cronjob.New(avatarRepo, thumbnailRepo, blobRepo, storage, storage, cfg.Worker)

	switch os.Args[1] {
	case "retention":
		return runner.RunRetention(ctx)
	case "reconcile":
		return runner.RunReconcile(ctx)
	default:
		return fmt.Errorf("unknown cronjob %q: use retention or reconcile", os.Args[1])
	}
}
