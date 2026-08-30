package observability

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const pgxTracerName = "gophprofile/postgres"

// PGXQueryTracer создает сегменты базы данных без записи SQL-запроса или аргументов.
type PGXQueryTracer struct{}

func (PGXQueryTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	operation := sqlOperation(data.SQL)
	ctx, _ = otel.Tracer(pgxTracerName).Start(ctx, "db."+strings.ToLower(operation),
		trace.WithAttributes(traceAttributes(operation)...),
	)
	return ctx
}

func (PGXQueryTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	span := trace.SpanFromContext(ctx)
	if data.Err != nil {
		span.RecordError(data.Err)
		span.SetStatus(codes.Error, "database query failed")
	} else {
		span.SetStatus(codes.Ok, "")
	}
	span.End()
}

func sqlOperation(query string) string {
	for {
		query = strings.TrimSpace(query)
		switch {
		case strings.HasPrefix(query, "--"):
			if newline := strings.IndexByte(query, '\n'); newline >= 0 {
				query = query[newline+1:]
				continue
			}
			return "QUERY"
		case strings.HasPrefix(query, "/*"):
			if end := strings.Index(query, "*/"); end >= 0 {
				query = query[end+2:]
				continue
			}
			return "QUERY"
		}
		break
	}
	fields := strings.Fields(query)
	if len(fields) == 0 {
		return "QUERY"
	}
	return strings.ToUpper(fields[0])
}

func traceAttributes(operation string) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String("db.system", "postgresql"),
		attribute.String("db.operation.name", operation),
	}
}
