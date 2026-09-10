package objectstore

import (
	"context"
	"io"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const storageTracerName = "gophprofile/objectstore"

func startStorageSpan(ctx context.Context, operation string) (context.Context, trace.Span) {
	return otel.Tracer(storageTracerName).Start(ctx, "storage."+operation,
		trace.WithAttributes(
			attribute.String("storage.system", "s3"),
			attribute.String("storage.operation", operation),
		),
	)
}

func finishStorageSpan(span trace.Span, err error) {
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "object storage operation failed")
	} else {
		span.SetStatus(codes.Ok, "")
	}
	span.End()
}

type tracedReadCloser struct {
	io.ReadCloser
	span trace.Span
	err  error
}

func (r *tracedReadCloser) Read(p []byte) (int, error) {
	n, err := r.ReadCloser.Read(p)
	if err != nil && err != io.EOF {
		r.err = err
	}
	return n, err
}

func (r *tracedReadCloser) Close() error {
	err := r.ReadCloser.Close()
	if err != nil && r.err == nil {
		r.err = err
	}
	finishStorageSpan(r.span, r.err)
	return err
}
