package cronjob

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const cronjobTracerName = "gophprofile/cronjob"

func startCronjobSpan(ctx context.Context, operation string) (context.Context, trace.Span) {
	return otel.Tracer(cronjobTracerName).Start(ctx, "cronjob."+operation,
		trace.WithAttributes(attribute.String("cronjob.operation", operation)),
	)
}

func finishCronjobSpan(span trace.Span, err error) {
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "cronjob operation failed")
	} else {
		span.SetStatus(codes.Ok, "")
	}
	span.End()
}
