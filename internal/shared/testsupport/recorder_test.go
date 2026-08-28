package testsupport

import (
	"context"
	"sync"
	"testing"

	"github.com/aikowocki/yandex-go-ext/internal/shared/logging"
)

func TestRecorderCapturesStructuredEvents(t *testing.T) {
	recorder := NewRecorder(nil)
	ctx := logging.WithSink(context.Background(), recorder)

	logging.Info(ctx, "user.created", "user_id", "user-1", "attempt", 2)
	logging.Error(ctx, "user.failed", logging.Err(context.Canceled))

	records := recorder.Records()
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}
	if records[0].Message != "user.created" || records[0].Fields["user_id"] != "user-1" {
		t.Fatalf("unexpected first record: %#v", records[0])
	}
	if records[1].Level != logging.ErrorLevel || records[1].Fields["error"] != context.Canceled {
		t.Fatalf("unexpected second record: %#v", records[1])
	}
}

func TestRecordersAreIsolatedForParallelContexts(t *testing.T) {
	t.Parallel()

	left := NewRecorder(nil)
	right := NewRecorder(nil)
	leftCtx := logging.WithSink(context.Background(), left)
	rightCtx := logging.WithSink(context.Background(), right)

	var group sync.WaitGroup
	for i := 0; i < 100; i++ {
		group.Add(2)
		go func() {
			defer group.Done()
			logging.Info(leftCtx, "left.event")
		}()
		go func() {
			defer group.Done()
			logging.Info(rightCtx, "right.event")
		}()
	}
	group.Wait()

	if got := len(left.Records()); got != 100 {
		t.Fatalf("expected 100 left records, got %d", got)
	}
	if got := len(right.Records()); got != 100 {
		t.Fatalf("expected 100 right records, got %d", got)
	}
}

func TestRecorderStopsAcceptingEventsAfterClose(t *testing.T) {
	recorder := NewRecorder(nil)
	recorder.Close()
	recorder.Record(logging.InfoLevel, "ignored")

	if got := len(recorder.Records()); got != 0 {
		t.Fatalf("expected closed recorder to remain empty, got %d records", got)
	}
}

func TestLoggerWithPreservesContextSink(t *testing.T) {
	ctx := Context(t)
	logger := logging.With(logging.String("component", "worker"))

	logger.Info(ctx, "job.completed", logging.String("job_id", "job-1"))

	Assert(t).
		Contains("job.completed").
		HasField("component", "worker").
		HasField("job_id", "job-1").
		NoErrors()
}
