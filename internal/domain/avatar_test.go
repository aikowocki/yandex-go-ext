package domain

import (
	"errors"
	"testing"
)

func TestAvatarProcessingLifecycle(t *testing.T) {
	avatar := &Avatar{UploadStatus: UploadStatusCompleted, ProcessingStatus: ProcessingStatusPending}
	if !avatar.IsProcessable() {
		t.Fatal("completed upload with pending processing must be processable")
	}

	avatar.MarkProcessing()
	if avatar.ProcessingStatus != ProcessingStatusProcessing {
		t.Fatalf("unexpected processing status: %s", avatar.ProcessingStatus)
	}
	if avatar.ProcessingAttempts != 1 || avatar.ProcessingStartedAt == nil {
		t.Fatal("processing metadata was not set")
	}

	avatar.MarkProcessed()
	if avatar.ProcessingStatus != ProcessingStatusCompleted {
		t.Fatalf("unexpected completed status: %s", avatar.ProcessingStatus)
	}

	avatar.MarkFailed(errors.New("thumbnail error"))
	if avatar.ProcessingStatus != ProcessingStatusFailed || avatar.LastError != "thumbnail error" {
		t.Fatal("failure metadata was not set")
	}
}

func TestThumbnailSizeDimensions(t *testing.T) {
	width, height, ok := ThumbnailSize100x100.Dimensions()
	if !ok || width != 100 || height != 100 {
		t.Fatalf("unexpected 100x100 dimensions: %d x %d", width, height)
	}
	if ThumbnailSize("unsupported").IsSupported() {
		t.Fatal("unsupported thumbnail size reported as supported")
	}
}
func TestAvatarLifecycleNilAndFailureBranches(t *testing.T) {
	var nilAvatar *Avatar
	nilAvatar.MarkProcessing()
	nilAvatar.MarkProcessed()
	nilAvatar.MarkFailed(errors.New("ignored"))
	avatar := &Avatar{UploadStatus: UploadStatusCompleted, ProcessingStatus: ProcessingStatusPending}
	avatar.MarkProcessing()
	if avatar.ProcessingStatus != ProcessingStatusProcessing || avatar.ProcessingAttempts != 1 || avatar.ProcessingStartedAt == nil {
		t.Fatalf("processing state = %+v", avatar)
	}
	avatar.MarkFailed(errors.New("failure"))
	if avatar.ProcessingStatus != ProcessingStatusFailed || avatar.LastError != "failure" {
		t.Fatalf("failure state = %+v", avatar)
	}
	avatar.MarkProcessed()
	if avatar.ProcessingStatus != ProcessingStatusCompleted {
		t.Fatalf("processed state = %+v", avatar)
	}
}
