package main

import (
	"context"
	"log"

	"github.com/aikowocki/yandex-go-ext/internal/app"
)

func main() {
	container, err := app.New(context.Background(), "gophprofile-server")
	if err != nil {
		log.Fatal("initialize application:", err)
	}
	defer func() { _ = container.Close() }()

	if err := container.Run(); err != nil {
		log.Fatal("application stopped with error:", err)
	}
}
