package slogbackend

import (
	"context"
	"fmt"
	stdslog "log/slog"
	"os"
	"strings"

	"github.com/aikowocki/yandex-go-ext/internal/config"
	"github.com/aikowocki/yandex-go-ext/internal/shared/logging/core"
)

// New создаёт логгер на базе slog.
func New(cfg config.LogConfig) (core.Logger, error) {
	level, err := parseLevel(cfg.Level)
	if err != nil {
		return nil, err
	}
	opts := &stdslog.HandlerOptions{Level: level}
	var handler stdslog.Handler
	if cfg.Format == "console" {
		handler = stdslog.NewTextHandler(os.Stdout, opts)
	} else {
		handler = stdslog.NewJSONHandler(os.Stdout, opts)
	}
	return &logger{logger: stdslog.New(handler)}, nil
}

func parseLevel(raw string) (stdslog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "debug":
		return stdslog.LevelDebug, nil
	case "info":
		return stdslog.LevelInfo, nil
	case "warn", "warning":
		return stdslog.LevelWarn, nil
	case "error":
		return stdslog.LevelError, nil
	default:
		return 0, fmt.Errorf("parse log level %q: expected debug, info, warn or error", raw)
	}
}

type logger struct{ logger *stdslog.Logger }

func (l *logger) Debug(ctx context.Context, message string, args ...any) {
	l.Log(ctx, core.DebugLevel, message, args...)
}
func (l *logger) Info(ctx context.Context, message string, args ...any) {
	l.Log(ctx, core.InfoLevel, message, args...)
}
func (l *logger) Warn(ctx context.Context, message string, args ...any) {
	l.Log(ctx, core.WarnLevel, message, args...)
}
func (l *logger) Error(ctx context.Context, message string, args ...any) {
	l.Log(ctx, core.ErrorLevel, message, args...)
}
func (l *logger) Log(ctx context.Context, level core.Level, message string, args ...any) {
	l.logger.LogAttrs(ctx, slogLevel(level), message, slogAttrs(core.NormalizeArgs(args))...)
}
func (l *logger) Enabled(ctx context.Context, level core.Level) bool {
	return l.logger.Enabled(ctx, slogLevel(level))
}
func (l *logger) With(attrs ...core.Attr) core.Logger {
	return &logger{logger: l.logger.With(slogAttrArgs(attrs)...)}
}
func (l *logger) WithGroup(name string) core.Logger {
	if name == "" {
		return l
	}
	return &logger{logger: l.logger.WithGroup(name)}
}
func (l *logger) Sync() error { return nil }

func slogLevel(level core.Level) stdslog.Level {
	switch level {
	case core.DebugLevel:
		return stdslog.LevelDebug
	case core.WarnLevel:
		return stdslog.LevelWarn
	case core.ErrorLevel:
		return stdslog.LevelError
	default:
		return stdslog.LevelInfo
	}
}

func slogAttrs(attrs []core.Attr) []stdslog.Attr {
	result := make([]stdslog.Attr, 0, len(attrs))
	for _, attr := range attrs {
		result = append(result, stdslog.Any(attr.Key, attr.Value))
	}
	return result
}

func slogAttrArgs(attrs []core.Attr) []any {
	converted := slogAttrs(attrs)
	result := make([]any, len(converted))
	for index, attr := range converted {
		result[index] = attr
	}
	return result
}
