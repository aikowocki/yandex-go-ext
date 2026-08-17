package logging

import (
	"context"
	"testing"

	"github.com/aikowocki/yandex-go-ext/internal/config"
)

func TestNewSupportsSlogAndZapBackends(t *testing.T) {
	for _, backend := range []string{"slog", "zap"} {
		t.Run(backend, func(t *testing.T) {
			logger, err := New(config.LogConfig{Backend: backend, Level: "debug", Format: "json"})
			if err != nil {
				t.Fatalf("New(%q): %v", backend, err)
			}
			_ = logger.Sync()
		})
	}
}

func TestNewRejectsUnknownBackend(t *testing.T) {
	if _, err := New(config.LogConfig{Backend: "unknown", Level: "info", Format: "json"}); err == nil {
		t.Fatal("unknown backend accepted")
	}
}
func TestFacadeChildLoggerAndGlobalOperations(t *testing.T) {
	logger, err := New(config.LogConfig{Backend: "slog", Level: "debug", Format: "json"})
	if err != nil {
		t.Fatal(err)
	}
	restore := Install(logger)
	defer restore()

	ctx := context.Background()
	Debug(ctx, "debug", String("scope", "test"))
	Info(ctx, "info", Int("count", 1))
	Warn(ctx, "warn", Bool("enabled", true))
	Error(ctx, "error", Err(nil))
	Log(ctx, InfoLevel, "log", Any("value", "test"))
	if !Enabled(ctx, InfoLevel) {
		t.Fatal("info level is not enabled")
	}
	child := With(String("service", "avatar"))
	child.Info(ctx, "child.info")
	child = child.With(String("component", "worker"))
	child = child.WithGroup("request")
	child.Log(ctx, DebugLevel, "child.debug", Duration("elapsed", 0))
	if !child.Enabled(ctx, DebugLevel) {
		t.Fatal("child debug level is not enabled")
	}
	if err := child.Sync(); err != nil {
		t.Fatalf("child sync: %v", err)
	}
	if err := L().Sync(); err != nil {
		t.Fatalf("global sync: %v", err)
	}
}

func TestInstallNilUsesNopAndRestoresPreviousLogger(t *testing.T) {
	previous := L()
	restore := Install(nil)
	if L() == nil {
		t.Fatal("nil logger was installed")
	}
	restore()
	if L() != previous {
		t.Fatal("previous logger was not restored")
	}
}

type testSink struct{}

func (testSink) Record(Level, string, ...Attr) {}

func TestSinkContextHandlesNilAndValue(t *testing.T) {
	var nilContext context.Context
	if SinkFromContext(nilContext) != nil {
		t.Fatal("nil context returned a sink")
	}
	ctx := WithSink(nilContext, testSink{})
	if SinkFromContext(ctx) == nil {
		t.Fatal("sink was not stored in nil context")
	}
	if SinkFromContext(context.Background()) != nil {
		t.Fatal("background context unexpectedly has a sink")
	}
}
