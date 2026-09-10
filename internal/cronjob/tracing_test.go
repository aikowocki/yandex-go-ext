package cronjob

import (
	"context"
	"errors"
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func TestCronjobTracing(t *testing.T) {
	provider := sdktrace.NewTracerProvider()
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })
	ctx, span := startCronjobSpan(context.Background(), "retention")
	if ctx == nil || span == nil {
		t.Fatal("cronjob span was not created")
	}
	finishCronjobSpan(span, errors.New("failed"))
	_, span = startCronjobSpan(context.Background(), "reconcile")
	finishCronjobSpan(span, nil)
}
