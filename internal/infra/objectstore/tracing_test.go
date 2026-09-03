package objectstore

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func TestStorageTracing(t *testing.T) {
	provider := sdktrace.NewTracerProvider()
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })
	ctx, span := startStorageSpan(context.Background(), "read")
	if ctx == nil || span == nil {
		t.Fatal("storage span was not created")
	}
	finishStorageSpan(span, errors.New("failed"))

	_, span = startStorageSpan(context.Background(), "write")
	finishStorageSpan(span, nil)
	reader := &tracedReadCloser{ReadCloser: io.NopCloser(strings.NewReader("data")), span: span}
	buffer := make([]byte, 4)
	if _, err := reader.Read(buffer); err != nil {
		t.Fatalf("Read: %v", err)
	}
	if err := reader.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}
