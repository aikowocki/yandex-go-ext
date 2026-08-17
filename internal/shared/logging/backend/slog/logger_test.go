package slogbackend

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/aikowocki/yandex-go-ext/internal/shared/logging/core"
)

func TestLoggerImplementsCommonFeatures(t *testing.T) {
	output := new(bytes.Buffer)
	logger := &logger{logger: slog.New(slog.NewJSONHandler(output, &slog.HandlerOptions{Level: slog.LevelDebug}))}
	ctx := context.Background()

	if !logger.Enabled(ctx, core.DebugLevel) {
		t.Fatal("debug level should be enabled")
	}
	logger.With(core.String("service", "avatar")).WithGroup("request").Log(
		ctx,
		core.InfoLevel,
		"request.completed",
		core.String("id", "req-1"),
	)

	for _, fragment := range []string{`"service":"avatar"`, `"request"`, `"id":"req-1"`, `"request.completed"`} {
		if !strings.Contains(output.String(), fragment) {
			t.Errorf("output %q does not contain %q", output.String(), fragment)
		}
	}
}

func TestLoggerDisablesLevels(t *testing.T) {
	logger := &logger{logger: slog.New(slog.NewJSONHandler(
		new(bytes.Buffer),
		&slog.HandlerOptions{Level: slog.LevelInfo},
	))}
	ctx := context.Background()
	if logger.Enabled(ctx, core.DebugLevel) {
		t.Fatal("debug level should be disabled")
	}
	if !logger.Enabled(ctx, core.InfoLevel) {
		t.Fatal("info level should be enabled")
	}
}
