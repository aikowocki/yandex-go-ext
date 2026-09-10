package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"github.com/aikowocki/yandex-go-ext/internal/app"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	defer stop()

	container, err := app.New(ctx, "gophprofile-worker")
	if err != nil {
		log.Fatal("initialize worker dependencies:", err)
	}
	defer func() { _ = container.Close() }()

	if err := container.Worker.Start(ctx); err != nil && ctx.Err() == nil {
		log.Fatal("worker stopped with error:", err)
	}
}
