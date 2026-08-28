package core

import (
	"context"
	"time"
)

// Logger описывает общий интерфейс логгера приложения.
type Logger interface {
	Debug(ctx context.Context, message string, args ...any)
	Info(ctx context.Context, message string, args ...any)
	Warn(ctx context.Context, message string, args ...any)
	Error(ctx context.Context, message string, args ...any)
	Log(ctx context.Context, level Level, message string, args ...any)
	Enabled(ctx context.Context, level Level) bool
	With(attrs ...Attr) Logger
	WithGroup(name string) Logger
	Sync() error
}

// Event представляет структурированную запись лога.
type Event struct {
	Time    time.Time      `json:"time"`
	Level   Level          `json:"level"`
	Message string         `json:"message"`
	Fields  map[string]any `json:"fields,omitempty"`
}

// Sink принимает структурированные записи лога.
type Sink interface {
	Record(level Level, message string, attrs ...Attr)
}
