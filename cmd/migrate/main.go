package main

import (
	"context"
	"fmt"
	"log"
	"os/signal"
	"syscall"
	"time"

	"github.com/aikowocki/yandex-go-ext/internal/app/providers"
	"github.com/aikowocki/yandex-go-ext/internal/infra/postgres/migrations"
)

const (
	migrationAttempts   = 5
	migrationRetryDelay = 2 * time.Second
	migrationTimeout    = 5 * time.Minute
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	defer stop()

	cfg, err := providers.NewConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	migrationCtx, cancel := context.WithTimeout(ctx, migrationTimeout)
	defer cancel()

	if err := runWithRetry(migrationCtx, func() error {
		return migrations.UpContext(migrationCtx, cfg.Database.DSN)
	}, migrationAttempts, migrationRetryDelay); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}
	return nil
}

func runWithRetry(ctx context.Context, operation func() error, attempts int, delay time.Duration) error {
	if attempts < 1 {
		return fmt.Errorf("migration attempts must be positive")
	}

	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := operation(); err == nil {
			return nil
		} else {
			lastErr = err
		}
		if attempt == attempts {
			break
		}

		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return ctx.Err()
		case <-timer.C:
		}
	}
	return fmt.Errorf("migration failed after %d attempts: %w", attempts, lastErr)
}
