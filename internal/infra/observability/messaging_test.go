package observability

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

func TestMessageContextRoundTrip(t *testing.T) {
	previousProvider := otel.GetTracerProvider()
	previousPropagator := otel.GetTextMapPropagator()
	provider := sdktrace.NewTracerProvider()
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	t.Cleanup(func() {
		otel.SetTracerProvider(previousProvider)
		otel.SetTextMapPropagator(previousPropagator)
		_ = provider.Shutdown(context.Background())
	})

	member, err := baggage.NewMember("request_scope", "upload")
	if err != nil {
		t.Fatal(err)
	}
	bag, err := baggage.New(member)
	if err != nil {
		t.Fatal(err)
	}
	ctx := baggage.ContextWithBaggage(context.Background(), bag)
	ctx, span := otel.Tracer("test").Start(ctx, "parent")
	defer span.End()

	headers := map[string]string{"content-type": "application/json"}
	InjectMessageContext(ctx, headers)
	if headers["traceparent"] == "" {
		t.Fatal("traceparent was not injected")
	}
	if headers["baggage"] == "" {
		t.Fatal("baggage was not injected")
	}

	extracted := ExtractMessageContext(context.Background(), headers)
	if got := baggage.FromContext(extracted).Member("request_scope").Value(); got != "upload" {
		t.Fatalf("baggage = %q, want upload", got)
	}
	if !trace.SpanContextFromContext(extracted).IsValid() {
		t.Fatal("extracted span context is invalid")
	}
}
