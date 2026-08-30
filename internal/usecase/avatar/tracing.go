package avatar

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const avatarTracerName = "gophprofile/avatar"

func startAvatarSpan(ctx context.Context, operation string) (context.Context, trace.Span) {
	return otel.Tracer(avatarTracerName).Start(ctx, "avatar."+operation,
		trace.WithAttributes(attribute.String("avatar.operation", operation)),
	)
}

func finishAvatarSpan(span trace.Span, err error) {
	if span == nil {
		return
	}
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "avatar operation failed")
	} else {
		span.SetStatus(codes.Ok, "")
	}
	span.End()
}
