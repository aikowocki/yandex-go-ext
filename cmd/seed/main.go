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

	container, err := app.New(ctx)
	if err != nil {
		log.Fatal("initialize seed dependencies:", err)
	}
	defer func() { _ = container.Close() }()
}
