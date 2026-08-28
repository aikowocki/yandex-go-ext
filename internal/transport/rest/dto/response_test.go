package dto

import (
	"testing"
	"time"

	"github.com/aikowocki/yandex-go-ext/internal/domain"
	"github.com/google/uuid"
)

func TestUploadFromDomain(t *testing.T) {
	now := time.Now().UTC()
	id := uuid.New()
	got := UploadFromDomain(&domain.Avatar{
		ID:               id,
		UserID:           "user-1",
		ProcessingStatus: domain.ProcessingStatusCompleted,
		CreatedAt:        now,
	})
	if got.ID != id.String() || got.UserID != "user-1" || got.URL != "/api/v1/avatars/"+id.String() {
		t.Fatalf("unexpected upload response: %+v", got)
	}
	if got.Status != string(domain.ProcessingStatusCompleted) || !got.CreatedAt.Equal(now) {
		t.Fatalf("unexpected upload status/timestamp: %+v", got)
	}
}

func TestAvatarFromDomain(t *testing.T) {
	id := uuid.New()
	created := time.Now().UTC().Add(-time.Minute)
	updated := time.Now().UTC()
	got := AvatarFromDomain(&domain.Avatar{
		ID:               id,
		UserID:           "user-1",
		FileName:         "avatar.png",
		MimeType:         "image/png",
		SizeBytes:        1234,
		Width:            640,
		Height:           480,
		CropX:            10,
		CropY:            20,
		CropSize:         300,
		ProcessingStatus: domain.ProcessingStatusPending,
		IsActive:         true,
		CreatedAt:        created,
		UpdatedAt:        updated,
	})
	if got.ID != id.String() || got.UserID != "user-1" || got.FileName != "avatar.png" || got.MimeType != "image/png" {
		t.Fatalf("unexpected avatar identity fields: %+v", got)
	}
	if got.Size != 1234 || got.Dimensions.Width != 640 || got.Dimensions.Height != 480 {
		t.Fatalf("unexpected avatar dimensions: %+v", got)
	}
	if got.Crop != (CropResponse{X: 10, Y: 20, Size: 300}) || got.Status != string(domain.ProcessingStatusProcessing) || !got.IsActive {
		t.Fatalf("unexpected avatar crop/status: %+v", got)
	}
	if !got.CreatedAt.Equal(created) || !got.UpdatedAt.Equal(updated) {
		t.Fatalf("unexpected avatar timestamps: %+v", got)
	}
}

func TestThumbnailFromDomain(t *testing.T) {
	got := ThumbnailFromDomain(&domain.Thumbnail{Size: domain.ThumbnailSize300x300}, "https://storage.test/thumb")
	if got != (ThumbnailResponse{Size: "300x300", URL: "https://storage.test/thumb"}) {
		t.Fatalf("unexpected thumbnail response: %+v", got)
	}
}

func TestProcessingStatus(t *testing.T) {
	if got := processingStatus(domain.ProcessingStatusPending); got != string(domain.ProcessingStatusProcessing) {
		t.Fatalf("pending status = %q", got)
	}
	if got := processingStatus(domain.ProcessingStatusFailed); got != string(domain.ProcessingStatusFailed) {
		t.Fatalf("failed status = %q", got)
	}
}
