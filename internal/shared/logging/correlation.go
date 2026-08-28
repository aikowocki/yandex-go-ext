package logging

import (
	"context"

	"github.com/aikowocki/yandex-go-ext/internal/shared/logging/core"
	"go.opentelemetry.io/otel/trace"
)

const (
	traceIDKey = "trace_id"
	spanIDKey  = "span_id"
)

// traceCorrelatingLogger добавляет ко всем бэкендам корреляцию с OpenTelemetry.
// Бэкенды отвечают только за форматирование и запись логов.
type traceCorrelatingLogger struct {
	backend Logger
}

func withTraceCorrelation(logger Logger) Logger {
	if logger == nil {
		return nil
	}
	if _, decorated := logger.(*traceCorrelatingLogger); decorated {
		return logger
	}
	return &traceCorrelatingLogger{backend: logger}
}

func (l *traceCorrelatingLogger) Debug(ctx context.Context, message string, args ...any) {
	l.Log(ctx, DebugLevel, message, args...)
}

func (l *traceCorrelatingLogger) Info(ctx context.Context, message string, args ...any) {
	l.Log(ctx, InfoLevel, message, args...)
}

func (l *traceCorrelatingLogger) Warn(ctx context.Context, message string, args ...any) {
	l.Log(ctx, WarnLevel, message, args...)
}

func (l *traceCorrelatingLogger) Error(ctx context.Context, message string, args ...any) {
	l.Log(ctx, ErrorLevel, message, args...)
}

func (l *traceCorrelatingLogger) Log(ctx context.Context, level Level, message string, args ...any) {
	attrs := withoutCorrelationAttrs(core.NormalizeArgs(args))
	spanContext := trace.SpanContextFromContext(ctx)
	if spanContext.IsValid() {
		attrs = append(attrs,
			core.String(traceIDKey, spanContext.TraceID().String()),
			core.String(spanIDKey, spanContext.SpanID().String()),
		)
	}
	l.backend.Log(ctx, level, message, attrArgs(attrs)...)
}

func (l *traceCorrelatingLogger) Enabled(ctx context.Context, level Level) bool {
	return l.backend.Enabled(ctx, level)
}

func (l *traceCorrelatingLogger) With(attrs ...Attr) Logger {
	return withTraceCorrelation(l.backend.With(withoutCorrelationAttrs(attrs)...))
}

func (l *traceCorrelatingLogger) WithGroup(name string) Logger {
	return withTraceCorrelation(l.backend.WithGroup(name))
}

func (l *traceCorrelatingLogger) Sync() error {
	return l.backend.Sync()
}

func withoutCorrelationAttrs(attrs []Attr) []Attr {
	filtered := make([]Attr, 0, len(attrs))
	for _, attr := range attrs {
		if attr.Key == traceIDKey || attr.Key == spanIDKey {
			continue
		}
		filtered = append(filtered, attr)
	}
	return filtered
}
