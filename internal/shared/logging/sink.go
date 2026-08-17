package logging

import (
	"context"

	"github.com/aikowocki/yandex-go-ext/internal/shared/logging/core"
)

// Event описывает событие для тестового sink-а.
type Event = core.Event

// Sink принимает структурированные события логирования.
type Sink = core.Sink

type sinkContextKey struct{}

// WithSink добавляет sink в контекст.
func WithSink(ctx context.Context, sink Sink) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, sinkContextKey{}, sink)
}

// SinkFromContext возвращает sink из контекста.
func SinkFromContext(ctx context.Context) Sink {
	if ctx == nil {
		return nil
	}
	sink, _ := ctx.Value(sinkContextKey{}).(Sink)
	return sink
}
