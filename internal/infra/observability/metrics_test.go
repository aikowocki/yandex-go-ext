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
	if got := safeRoute("/avatars"); got != "/avatars" {
		t.Fatalf("safeRoute(route) = %q", got)
	}
	if got := normalizedStatus(0); got != 500 {
		t.Fatalf("normalizedStatus(0) = %d", got)
	}
	if got := normalizedStatus(201); got != 201 {
		t.Fatalf("normalizedStatus(201) = %d", got)
	}
	if got := safeStatus("unexpected"); got != "unknown" {
		t.Fatalf("safeStatus(unexpected) = %q", got)
	}
	for _, status := range []string{"success", "error", "skipped"} {
		if got := safeStatus(status); got != status {
			t.Fatalf("safeStatus(%q) = %q", status, got)
		}
	}
	if got := safeMessagingValue(""); got != "unknown" {
		t.Fatalf("safeMessagingValue(\"\") = %q", got)
	}
	if got := safeMessagingValue("kafka"); got != "kafka" {
		t.Fatalf("safeMessagingValue(kafka) = %q", got)
	}
}

func TestMetricsRecordersAndDependencyState(t *testing.T) {
	ctx := context.Background()
	RecordAvatarUpload(ctx, "success", time.Millisecond, 42)
	RecordAvatarUpload(ctx, "unexpected", time.Millisecond, -1)
	RecordAvatarProcessing(ctx, "error", time.Millisecond)
	RecordMessagingPublish(ctx, "", "", "success", time.Millisecond)
	RecordMessagingConsume(ctx, "kafka", "avatars", "error", time.Millisecond)
	RecordMessagingRetry(ctx, "kafka", "avatars", false)
	RecordMessagingRetry(ctx, "kafka", "avatars", true)

	ChangeQueueDepth("", "", 1)
	ChangeQueueDepth("", "", -1)
	SetDependencyAvailability("database", true)
	SetDependencyAvailability("database", false)
	SetDependencyAvailability("storage", true)
	SetDependencyAvailability("broker", false)
	SetDependencyAvailability("unknown", true)
}
