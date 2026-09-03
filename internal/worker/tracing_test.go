package worker

import (
	"context"
	"errors"
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func TestWorkerTracing(t *testing.T) {
	provider := sdktrace.NewTracerProvider()
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })
	ctx, span := startWorkerSpan(context.Background(), "process")
	if ctx == nil || span == nil {
		t.Fatal("worker span was not created")
	}
	finishWorkerSpan(span, errors.New("failed"))
	_, span = startWorkerSpan(context.Background(), "cleanup")
	finishWorkerSpan(span, nil)
}
