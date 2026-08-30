package observability

import (
	"context"
	"maps"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

const messagingTracerName = "gophprofile/messaging"

type headerCarrier map[string]string

func (c headerCarrier) Get(key string) string { return c[key] }
func (c headerCarrier) Set(key, value string) { c[key] = value }
func (c headerCarrier) Keys() []string {
	keys := make([]string, 0, len(c))
	for key := range c {
		keys = append(keys, key)
	}
	return keys
}

func InjectMessageContext(ctx context.Context, headers map[string]string) {
	if headers == nil {
		return
	}
	otel.GetTextMapPropagator().Inject(ctx, propagation.TextMapCarrier(headerCarrier(headers)))
}

func ExtractMessageContext(ctx context.Context, headers map[string]string) context.Context {
	return otel.GetTextMapPropagator().Extract(ctx, propagation.TextMapCarrier(headerCarrier(headers)))
}

func StartProducerSpan(ctx context.Context, system, destination string) (context.Context, trace.Span) {
	return otel.Tracer(messagingTracerName).Start(ctx, "messaging.publish", trace.WithSpanKind(trace.SpanKindProducer), trace.WithAttributes(
		attribute.String("messaging.system", system),
		attribute.String("messaging.destination.name", destination),
	))
}

func StartConsumerSpan(ctx context.Context, system, destination string, attempt int) (context.Context, trace.Span) {
	return otel.Tracer(messagingTracerName).Start(ctx, "messaging.consume", trace.WithSpanKind(trace.SpanKindConsumer), trace.WithAttributes(
		attribute.String("messaging.system", system),
		attribute.String("messaging.destination.name", destination),
		attribute.Int("messaging.delivery.attempt", attempt),
	))
}

func FinishMessagingSpan(span trace.Span, err error) {
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "message operation failed")
	} else {
		span.SetStatus(codes.Ok, "")
	}
	span.End()
}

func CloneHeaders(headers map[string]string) map[string]string {
	cloned := make(map[string]string, len(headers))
	maps.Copy(cloned, headers)
	return cloned
}
