package logging

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/otel/log"
	sdklog "go.opentelemetry.io/otel/sdk/log"
)

type testStringer string

func (s testStringer) String() string { return string(s) }

func TestOTelSeverityAndAttributes(t *testing.T) {
	for _, level := range []Level{DebugLevel, InfoLevel, WarnLevel, ErrorLevel, Level(99)} {
		_ = otelSeverity(level)
	}
	attrs := []Attr{
		String("string", "value"),
		Int("int", 1),
		Int32("int32", 2),
		Int64("int64", 3),
		Bool("bool", true),
		Duration("duration", time.Second),
		Any("stringer", testStringer("value")),
		Any("nil", nil),
		Any("fallback", 1.5),
	}
	for _, attr := range attrs {
		if got := otelAttr(attr); got.Key != attr.Key {
			t.Fatalf("otelAttr key = %q, want %q", got.Key, attr.Key)
		}
	}
	if got := otelAttr(Attr{Key: "error", Value: errors.New("boom")}); got.Key != "error" {
		t.Fatalf("error attr key = %q", got.Key)
	}
}

func TestOTelLogBody(t *testing.T) {
	if got := otelLogBody("message", nil); got != "message" {
		t.Fatalf("body without trace = %q", got)
	}
	body := otelLogBody("message", []Attr{String("trace_id", "abc")})
	if !strings.Contains(body, `"trace_id":"abc"`) || !strings.Contains(body, `"message":"message"`) {
		t.Fatalf("correlated body = %q", body)
	}
	if got := otelLogBody("message", []Attr{String("other", "value")}); got != "message" {
		t.Fatalf("body with unrelated attrs = %q", got)
	}
	_ = log.SeverityInfo
}

func TestOTelLoggerBridgeDelegatesOperations(t *testing.T) {
	backend := &recordingLogger{}
	provider := sdklog.NewLoggerProvider()
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })
	bridge := WithOTelLogger(backend, provider.Logger("test"))
	if bridge == nil {
		t.Fatal("bridge is nil")
	}
	if WithOTelLogger(nil, provider.Logger("test")) != nil {
		t.Fatal("nil backend should be returned as nil")
	}
	if WithOTelLogger(backend, nil) != backend {
		t.Fatal("nil OTel logger should return backend")
	}
	ctx := context.Background()
	bridge.Debug(ctx, "debug")
	bridge.Info(ctx, "info", String("trace_id", "abc"))
	bridge.Warn(ctx, "warn")
	bridge.Error(ctx, "error", Err(errors.New("boom")))
	bridge.Log(ctx, InfoLevel, "log", Int("count", 1))
	child := bridge.With(String("component", "test"))
	child = child.WithGroup("group")
	_ = child.Enabled(ctx, InfoLevel)
	if err := child.Sync(); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if backend.calls != 5 {
		t.Fatalf("backend calls = %d, want 5", backend.calls)
	}
}

func (l *recordingLogger) WithGroup(string) Logger             { return l }
func (l *recordingLogger) Enabled(context.Context, Level) bool { return true }
func (l *recordingLogger) Sync() error                         { return nil }
