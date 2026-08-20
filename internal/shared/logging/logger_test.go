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

type recordingLogger struct {
	Logger
	attrs  []Attr
	logged []Attr
	calls  int
}

func (l *recordingLogger) With(attrs ...Attr) Logger {
	combined := append([]Attr(nil), l.attrs...)
	combined = append(combined, attrs...)
	return &recordingLogger{Logger: l.Logger, attrs: combined}
}

func (l *recordingLogger) Log(_ context.Context, _ Level, _ string, args ...any) {
	l.calls++
	l.logged = append([]Attr(nil), l.attrs...)
	l.logged = append(l.logged, normalizeArgs(args)...)
}

func TestChildLoggerPassesPersistentAttributesToBackend(t *testing.T) {
	backend := &recordingLogger{}
	child := newChildLogger(backend, String("component", "worker"))

	child.Info(context.Background(), "worker started")

	if len(backend.logged) != 0 {
		t.Fatalf("original backend unexpectedly recorded the event: %#v", backend.logged)
	}

	scopedBackend := child.(*childLogger).backend.(*recordingLogger)
	if len(scopedBackend.logged) != 1 || scopedBackend.logged[0] != String("component", "worker") {
		t.Fatalf("persistent attributes were not passed to backend: %#v", scopedBackend.logged)
	}
}

func TestPackageLoggingUsesContextLogger(t *testing.T) {
	backend := &recordingLogger{}
	ctx := WithLogger(context.Background(), backend)

	Info(ctx, "context scoped")

	if backend.calls != 1 {
		t.Fatalf("context logger did not receive the event: calls=%d attrs=%#v", backend.calls, backend.logged)
	}
}

type countingSink struct {
	calls int
}

func (s *countingSink) Record(Level, string, ...Attr) {
	s.calls++
}

func TestContextChildLoggerRecordsSinkOnce(t *testing.T) {
	child := newChildLogger(&recordingLogger{}, String("component", "worker"))
	sink := &countingSink{}
	ctx := WithSink(WithLogger(context.Background(), child), sink)

	Info(ctx, "worker started")

	if sink.calls != 1 {
		t.Fatalf("expected one sink event, got %d", sink.calls)
	}
}
