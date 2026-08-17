package logging

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/aikowocki/yandex-go-ext/internal/config"
	slogbackend "github.com/aikowocki/yandex-go-ext/internal/shared/logging/backend/slog"
	zapbackend "github.com/aikowocki/yandex-go-ext/internal/shared/logging/backend/zap"
	"github.com/aikowocki/yandex-go-ext/internal/shared/logging/core"
)

// Logger — общий интерфейс логгера.
type Logger = core.Logger

type dispatcher struct {
	base atomic.Value
}

// Level задаёт уровень логирования.
type Level = core.Level

// Attr содержит структурированное поле лога.
type Attr = core.Attr

const (
	// DebugLevel — отладочный уровень логирования.
	DebugLevel = core.DebugLevel
	// InfoLevel — информационный уровень логирования.
	InfoLevel = core.InfoLevel
	// WarnLevel — уровень предупреждений.
	WarnLevel = core.WarnLevel
	// ErrorLevel — уровень ошибок.
	ErrorLevel = core.ErrorLevel
)

// String создаёт строковый атрибут лога.
func String(key, value string) Attr { return core.String(key, value) }

// Int создаёт целочисленный атрибут лога.
func Int(key string, value int) Attr { return core.Int(key, value) }

// Int32 создаёт атрибут лога со значением int32.
func Int32(key string, value int32) Attr { return core.Int32(key, value) }

// Int64 создаёт атрибут лога со значением int64.
func Int64(key string, value int64) Attr { return core.Int64(key, value) }

// Bool создаёт логический атрибут лога.
func Bool(key string, value bool) Attr { return core.Bool(key, value) }

// Duration создаёт атрибут лога с длительностью.
func Duration(key string, value time.Duration) Attr {
	return core.Duration(key, value)
}

// Any добавляет произвольное значение в лог.
func Any(key string, value any) Attr { return core.Any(key, value) }

// Err добавляет ошибку в лог.
func Err(err error) Attr { return core.Err(err) }

// UUID добавляет идентификатор в лог.
func UUID(key string, value fmt.Stringer) Attr {
	return core.UUID(key, value)
}

func normalizeArgs(args []any) []Attr {
	return core.NormalizeArgs(args)
}

func attrArgs(attrs []Attr) []any {
	args := make([]any, len(attrs))
	for index, attr := range attrs {
		args[index] = attr
	}
	return args
}

type loggerState struct {
	logger Logger
}

func newDispatcher() *dispatcher {
	d := &dispatcher{}
	d.base.Store(loggerState{logger: zapbackend.NewNop()})
	return d
}

func (d *dispatcher) current() Logger {
	return d.base.Load().(loggerState).logger
}

func (d *dispatcher) set(logger Logger) (previous Logger) {
	previous = d.current()
	d.base.Store(loggerState{logger: logger})
	return previous
}

func (d *dispatcher) emit(ctx context.Context, level Level, message string, args []any, write func(Logger, []any)) {
	attrs := normalizeArgs(args)
	if sink := SinkFromContext(ctx); sink != nil {
		sink.Record(level, message, attrs...)
	}
	write(d.current(), attrArgs(attrs))
}

func (d *dispatcher) Debug(ctx context.Context, message string, args ...any) {
	d.Log(ctx, DebugLevel, message, args...)
}
func (d *dispatcher) Info(ctx context.Context, message string, args ...any) {
	d.Log(ctx, InfoLevel, message, args...)
}
func (d *dispatcher) Warn(ctx context.Context, message string, args ...any) {
	d.Log(ctx, WarnLevel, message, args...)
}
func (d *dispatcher) Error(ctx context.Context, message string, args ...any) {
	d.Log(ctx, ErrorLevel, message, args...)
}
func (d *dispatcher) Log(ctx context.Context, level Level, message string, args ...any) {
	d.emit(ctx, level, message, args, func(logger Logger, attrs []any) {
		logger.Log(ctx, level, message, attrs...)
	})
}
func (d *dispatcher) Enabled(ctx context.Context, level Level) bool {
	return d.current().Enabled(ctx, level)
}
func (d *dispatcher) With(attrs ...Attr) Logger {
	return newChildLogger(d.current(), attrs...)
}
func (d *dispatcher) WithGroup(name string) Logger {
	return newChildLogger(d.current().WithGroup(name))
}
func (d *dispatcher) Sync() error { return d.current().Sync() }

type childLogger struct {
	backend Logger
	attrs   []Attr
}

func newChildLogger(backend Logger, attrs ...Attr) Logger {
	copied := make([]Attr, len(attrs))
	copy(copied, attrs)
	return &childLogger{backend: backend, attrs: copied}
}

func (l *childLogger) Debug(ctx context.Context, message string, args ...any) {
	l.Log(ctx, DebugLevel, message, args...)
}
func (l *childLogger) Info(ctx context.Context, message string, args ...any) {
	l.Log(ctx, InfoLevel, message, args...)
}
func (l *childLogger) Warn(ctx context.Context, message string, args ...any) {
	l.Log(ctx, WarnLevel, message, args...)
}
func (l *childLogger) Error(ctx context.Context, message string, args ...any) {
	l.Log(ctx, ErrorLevel, message, args...)
}
func (l *childLogger) Log(ctx context.Context, level Level, message string, args ...any) {
	callAttrs := normalizeArgs(args)
	allAttrs := make([]Attr, 0, len(l.attrs)+len(callAttrs))
	allAttrs = append(allAttrs, l.attrs...)
	allAttrs = append(allAttrs, callAttrs...)
	if sink := SinkFromContext(ctx); sink != nil {
		sink.Record(level, message, allAttrs...)
	}
	l.backend.Log(ctx, level, message, args...)
}
func (l *childLogger) Enabled(ctx context.Context, level Level) bool {
	return l.backend.Enabled(ctx, level)
}
func (l *childLogger) With(attrs ...Attr) Logger {
	combined := make([]Attr, 0, len(l.attrs)+len(attrs))
	combined = append(combined, l.attrs...)
	combined = append(combined, attrs...)
	return &childLogger{backend: l.backend.With(attrs...), attrs: combined}
}
func (l *childLogger) WithGroup(name string) Logger {
	return &childLogger{backend: l.backend.WithGroup(name), attrs: append([]Attr(nil), l.attrs...)}
}
func (l *childLogger) Sync() error { return l.backend.Sync() }

var global = newDispatcher()

// New создаёт логгер из конфигурации.
func New(cfg config.LogConfig) (Logger, error) {
	backend := strings.ToLower(strings.TrimSpace(cfg.Backend))
	if backend == "" {
		backend = "slog"
	}
	switch backend {
	case "slog":
		return slogbackend.New(cfg)
	case "zap":
		return zapbackend.New(cfg)
	default:
		return nil, fmt.Errorf("unknown log backend %q: expected slog or zap", cfg.Backend)
	}
}

// Install устанавливает глобальный логгер и возвращает функцию восстановления.
func Install(logger Logger) func() {
	if logger == nil {
		logger = zapbackend.NewNop()
	}
	previous := global.set(logger)
	return func() { global.set(previous) }
}

// L возвращает глобальный логгер.
func L() Logger { return global }

// Debug записывает отладочное сообщение.
func Debug(ctx context.Context, message string, args ...any) {
	global.Debug(ctx, message, args...)
}

// Info записывает информационное сообщение.
func Info(ctx context.Context, message string, args ...any) {
	global.Info(ctx, message, args...)
}

// Warn записывает предупреждение.
func Warn(ctx context.Context, message string, args ...any) {
	global.Warn(ctx, message, args...)
}

// Error записывает ошибку.
func Error(ctx context.Context, message string, args ...any) {
	global.Error(ctx, message, args...)
}

// Log записывает сообщение с указанным уровнем.
func Log(ctx context.Context, level Level, message string, args ...any) {
	global.Log(ctx, level, message, args...)
}

// Enabled проверяет, включён ли уровень логирования.
func Enabled(ctx context.Context, level Level) bool {
	return global.Enabled(ctx, level)
}

// With добавляет атрибуты к глобальному логгеру.
func With(attrs ...Attr) Logger {
	return global.With(attrs...)
}

// WithGroup добавляет группу полей к глобальному логгеру.
func WithGroup(name string) Logger {
	return global.WithGroup(name)
}
