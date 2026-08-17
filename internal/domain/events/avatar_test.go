package events

import (
	"testing"

	"github.com/aikowocki/yandex-go-ext/internal/domain"
	"github.com/google/uuid"
)

func TestAvatarEventConstructors(t *testing.T) {
	avatar := &domain.Avatar{ID: uuid.New(), UserID: "user-1", S3KeyOriginal: "original.jpg", MimeType: "image/jpeg"}
	process := NewAvatarProcessEvent(avatar)
	if process.AvatarID != avatar.ID.String() || len(process.Operations) != 2 {
		t.Fatalf("process event = %+v", process)
	}
	uploaded := NewAvatarUploadedEvent(avatar)
	if uploaded.AvatarID != avatar.ID.String() || uploaded.UserID != avatar.UserID || uploaded.S3Key != avatar.S3KeyOriginal || uploaded.Timestamp.IsZero() {
		t.Fatalf("uploaded event = %+v", uploaded)
	}
	deleted := NewAvatarDeletedEvent(avatar, []string{"100x100.jpg", "300x300.jpg"})
	if len(deleted.S3Keys) != 3 || deleted.S3Keys[0] != avatar.S3KeyOriginal || deleted.UserID != avatar.UserID {
		t.Fatalf("deleted event = %+v", deleted)
	}
}
