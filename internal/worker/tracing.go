package worker

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const workerTracerName = "gophprofile/worker"

func startWorkerSpan(ctx context.Context, operation string) (context.Context, trace.Span) {
	return otel.Tracer(workerTracerName).Start(ctx, "worker."+operation,
		trace.WithAttributes(attribute.String("worker.operation", operation)),
	)
}

func finishWorkerSpan(span trace.Span, err error) {
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "worker operation failed")
	} else {
		span.SetStatus(codes.Ok, "")
	}
	span.End()
}
