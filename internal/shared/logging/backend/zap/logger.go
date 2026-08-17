package zapbackend

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/aikowocki/yandex-go-ext/internal/config"
	"github.com/aikowocki/yandex-go-ext/internal/shared/logging/core"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// New создаёт логгер на базе zap.
func New(cfg config.LogConfig) (core.Logger, error) {
	level := zapcore.InfoLevel
	if err := level.Set(cfg.Level); err != nil {
		return nil, fmt.Errorf("parse log level %q: %w", cfg.Level, err)
	}

	var base zap.Config
	if cfg.Format == "console" {
		base = zap.NewDevelopmentConfig()
		base.Encoding = "console"
		base.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	} else {
		base = zap.NewProductionConfig()
		base.Encoding = "json"
	}
	base.Level = zap.NewAtomicLevelAt(level)
	base.OutputPaths = []string{os.Stdout.Name()}
	base.ErrorOutputPaths = []string{os.Stderr.Name()}
	logger, err := base.Build()
	if err != nil {
		return nil, err
	}
	return newLogger(logger), nil
}

// NewNop создаёт пустой zap-логгер.
func NewNop() core.Logger {
	return newLogger(zap.NewNop())
}

type logger struct{ logger *zap.Logger }

func newLogger(raw *zap.Logger) core.Logger { return &logger{logger: raw} }

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
func (l *logger) Log(_ context.Context, level core.Level, message string, args ...any) {
	fields := zapFields(core.NormalizeArgs(args))
	switch level {
	case core.DebugLevel:
		l.logger.Debug(message, fields...)
	case core.WarnLevel:
		l.logger.Warn(message, fields...)
	case core.ErrorLevel:
		l.logger.Error(message, fields...)
	default:
		l.logger.Info(message, fields...)
	}
}
func (l *logger) Enabled(_ context.Context, level core.Level) bool {
	return l.logger.Core().Enabled(zapLogLevel(level))
}
func (l *logger) With(attrs ...core.Attr) core.Logger {
	return &logger{logger: l.logger.With(zapFields(attrs)...)}
}
func (l *logger) WithGroup(name string) core.Logger {
	if name == "" {
		return l
	}
	return &logger{logger: l.logger.With(zap.Namespace(name))}
}
func (l *logger) Sync() error { return l.logger.Sync() }

func zapLogLevel(level core.Level) zapcore.Level {
	switch level {
	case core.DebugLevel:
		return zapcore.DebugLevel
	case core.WarnLevel:
		return zapcore.WarnLevel
	case core.ErrorLevel:
		return zapcore.ErrorLevel
	default:
		return zapcore.InfoLevel
	}
}

func zapFields(attrs []core.Attr) []zap.Field {
	fields := make([]zap.Field, 0, len(attrs))
	for _, attr := range attrs {
		switch value := attr.Value.(type) {
		case error:
			fields = append(fields, zap.NamedError(attr.Key, value))
		case string:
			fields = append(fields, zap.String(attr.Key, value))
		case int:
			fields = append(fields, zap.Int(attr.Key, value))
		case int32:
			fields = append(fields, zap.Int32(attr.Key, value))
		case int64:
			fields = append(fields, zap.Int64(attr.Key, value))
		case bool:
			fields = append(fields, zap.Bool(attr.Key, value))
		case time.Duration:
			fields = append(fields, zap.Duration(attr.Key, value))
		default:
			fields = append(fields, zap.Any(attr.Key, value))
		}
	}
	return fields
}
