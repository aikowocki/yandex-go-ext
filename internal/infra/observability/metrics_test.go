package observability

import (
	"context"
	"testing"
	"time"
)

func TestBeginHTTPRecordsCompletionWithoutPanic(t *testing.T) {
	finish := BeginHTTP(context.Background(), "GET", "/api/v1/avatars/:id")
	finish(200, 12*time.Millisecond)
	ChangeQueueDepth("kafka", "avatar.uploaded", 1)
	ChangeQueueDepth("kafka", "avatar.uploaded", -1)
}

func TestMetricsNormalizeUnsafeValues(t *testing.T) {
	if got := safeRoute(""); got != "unknown" {
		t.Fatalf("safeRoute(\"\") = %q", got)
	}
	if got := normalizedStatus(0); got != 500 {
		t.Fatalf("normalizedStatus(0) = %d", got)
	}
	if got := safeStatus("unexpected"); got != "unknown" {
		t.Fatalf("safeStatus(unexpected) = %q", got)
	}
}
