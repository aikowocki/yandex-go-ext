package avatar

import (
	"context"
	"errors"
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func TestAvatarTracing(t *testing.T) {
	provider := sdktrace.NewTracerProvider()
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })
	ctx, span := startAvatarSpan(context.Background(), "create")
	if ctx == nil || span == nil {
		t.Fatal("avatar span was not created")
	}
	finishAvatarSpan(span, errors.New("failed"))
	_, span = startAvatarSpan(context.Background(), "update")
	finishAvatarSpan(span, nil)
	finishAvatarSpan(nil, nil)
}
