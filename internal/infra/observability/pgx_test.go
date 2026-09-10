package observability

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func TestSQLOperationSkipsLeadingComments(t *testing.T) {
	tests := []struct {
		query string
		want  string
	}{
		{query: "SELECT 1", want: "SELECT"},
		{query: "-- name: GetAvatar :one\nSELECT * FROM avatars", want: "SELECT"},
		{query: "/* migration */\nINSERT INTO avatars VALUES (1)", want: "INSERT"},
		{query: "-- comment only", want: "QUERY"},
		{query: "", want: "QUERY"},
	}
	for _, test := range tests {
		if got := sqlOperation(test.query); got != test.want {
			t.Errorf("sqlOperation(%q) = %q, want %q", test.query, got, test.want)
		}
	}
}

func TestPGXQueryTracerStartAndEnd(t *testing.T) {
	provider := sdktrace.NewTracerProvider()
	previous := otel.GetTracerProvider()
	otel.SetTracerProvider(provider)
	t.Cleanup(func() {
		otel.SetTracerProvider(previous)
		_ = provider.Shutdown(context.Background())
	})

	tracer := PGXQueryTracer{}
	ctx := tracer.TraceQueryStart(context.Background(), nil, pgx.TraceQueryStartData{SQL: "SELECT 1"})
	tracer.TraceQueryEnd(ctx, nil, pgx.TraceQueryEndData{})
	ctx = tracer.TraceQueryStart(context.Background(), nil, pgx.TraceQueryStartData{SQL: "UPDATE avatars SET x = 1"})
	tracer.TraceQueryEnd(ctx, nil, pgx.TraceQueryEndData{Err: errors.New("query failed")})
	attrs := traceAttributes("SELECT")
	if len(attrs) != 2 {
		t.Fatalf("traceAttributes len = %d", len(attrs))
	}
}
