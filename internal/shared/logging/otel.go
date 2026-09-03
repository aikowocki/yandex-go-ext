package logging

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aikowocki/yandex-go-ext/internal/shared/logging/core"
	otellog "go.opentelemetry.io/otel/log"
)

// WithOTelLogger добавляет экспорт логов в OpenTelemetry.
func WithOTelLogger(logger Logger, otelLogger otellog.Logger) Logger {
	if logger == nil || otelLogger == nil {
		return logger
	}
	return &otelLoggerBridge{backend: logger, otel: otelLogger}
}

type otelLoggerBridge struct {
	backend Logger
	otel    otellog.Logger
	attrs   []Attr
}

func (l *otelLoggerBridge) Debug(ctx context.Context, message string, args ...any) {
	l.Log(ctx, DebugLevel, message, args...)
}

func (l *otelLoggerBridge) Info(ctx context.Context, message string, args ...any) {
	l.Log(ctx, InfoLevel, message, args...)
}

func (l *otelLoggerBridge) Warn(ctx context.Context, message string, args ...any) {
	l.Log(ctx, WarnLevel, message, args...)
}

func (l *otelLoggerBridge) Error(ctx context.Context, message string, args ...any) {
	l.Log(ctx, ErrorLevel, message, args...)
}

func (l *otelLoggerBridge) Log(ctx context.Context, level Level, message string, args ...any) {
	l.backend.Log(ctx, level, message, args...)

	attrs := make([]Attr, 0, len(l.attrs)+len(args))
	attrs = append(attrs, l.attrs...)
	attrs = append(attrs, core.NormalizeArgs(args)...)
	l.emit(ctx, level, message, attrs)
}

func (l *otelLoggerBridge) emit(ctx context.Context, level Level, message string, attrs []Attr) {
	record := otellog.Record{}
	record.SetTimestamp(time.Now())
	record.SetBody(otellog.StringValue(otelLogBody(message, attrs)))
	record.SetSeverity(otelSeverity(level))
	record.SetSeverityText(level.String())

	for _, attr := range attrs {
		if attr.Key == "" {
			continue
		}
		if err, ok := attr.Value.(error); ok && err != nil {
			record.SetErr(err)
			record.AddAttributes(otellog.String(attr.Key, err.Error()))
			continue
		}
		record.AddAttributes(otelAttr(attr))
	}
	l.otel.Emit(ctx, record)
}

func (l *otelLoggerBridge) Enabled(ctx context.Context, level Level) bool {
	return l.backend.Enabled(ctx, level) || l.otel.Enabled(ctx, otellog.EnabledParameters{Severity: otelSeverity(level)})
}

func (l *otelLoggerBridge) With(attrs ...Attr) Logger {
	combined := make([]Attr, 0, len(l.attrs)+len(attrs))
	combined = append(combined, l.attrs...)
	combined = append(combined, attrs...)
	return &otelLoggerBridge{backend: l.backend.With(attrs...), otel: l.otel, attrs: combined}
}

func (l *otelLoggerBridge) WithGroup(name string) Logger {
	return &otelLoggerBridge{backend: l.backend.WithGroup(name), otel: l.otel, attrs: append([]Attr(nil), l.attrs...)}
}

func (l *otelLoggerBridge) Sync() error { return l.backend.Sync() }

func otelLogBody(message string, attrs []Attr) string {
	for _, attr := range attrs {
		if attr.Key != "trace_id" || attr.Value == nil {
			continue
		}
		body, err := json.Marshal(struct {
			Message string `json:"message"`
			TraceID string `json:"trace_id"`
		}{
			Message: message,
			TraceID: fmt.Sprint(attr.Value),
		})
		if err == nil {
			return string(body)
		}
	}
	return message
}

func otelSeverity(level Level) otellog.Severity {
	switch level {
	case DebugLevel:
		return otellog.SeverityDebug
	case WarnLevel:
		return otellog.SeverityWarn
	case ErrorLevel:
		return otellog.SeverityError
	default:
		return otellog.SeverityInfo
	}
}

func otelAttr(attr Attr) otellog.KeyValue {
	switch value := attr.Value.(type) {
	case string:
		return otellog.String(attr.Key, value)
	case int:
		return otellog.Int(attr.Key, value)
	case int32:
		return otellog.Int64(attr.Key, int64(value))
	case int64:
		return otellog.Int64(attr.Key, value)
	case bool:
		return otellog.Bool(attr.Key, value)
	case time.Duration:
		return otellog.Int64(attr.Key, int64(value))
	case fmt.Stringer:
		return otellog.String(attr.Key, value.String())
	case nil:
		return otellog.Empty(attr.Key)
	default:
		return otellog.String(attr.Key, fmt.Sprint(value))
	}
}
