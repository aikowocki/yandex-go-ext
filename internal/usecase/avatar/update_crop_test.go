package avatar

import (
	"context"
	"errors"
	"testing"

	"github.com/aikowocki/yandex-go-ext/internal/domain"
	"github.com/google/uuid"
)

func TestUpdateCropPersistsCropAndQueuesProcessing(t *testing.T) {
	avatarID := uuid.New()
	repo := &avatarRepoStub{avatar: &domain.Avatar{
		ID:               avatarID,
		UserID:           "user-1",
		Width:            1200,
		Height:           800,
		UploadStatus:     domain.UploadStatusCompleted,
		ProcessingStatus: domain.ProcessingStatusCompleted,
		CropSize:         800,
		LastError:        "old error",
	}}
	publisher := &publisherStub{}
	useCase := New(repo, nil, &storageStub{}, publisher, nil, nil, nil, passthroughTx{}, nil)

	updated, err := useCase.UpdateCrop(context.Background(), "user-1", avatarID, domain.AvatarCrop{X: 200, Y: 0, Size: 800})
	if err != nil {
		t.Fatal(err)
	}
	if updated.CropX != 200 || updated.CropY != 0 || updated.CropSize != 800 {
		t.Fatalf("unexpected crop: %+v", updated)
	}
	if updated.ProcessingStatus != domain.ProcessingStatusPending || updated.LastError != "" {
		t.Fatalf("expected pending clean processing state: %+v", updated)
	}
	if publisher.message == nil || publisher.message.Topic != "avatar.uploaded" {
		t.Fatal("expected avatar processing event")
	}
}

func TestUpdateCropRejectsForeignAvatar(t *testing.T) {
	avatarID := uuid.New()
	repo := &avatarRepoStub{avatar: &domain.Avatar{
		ID:           avatarID,
		UserID:       "owner",
		Width:        100,
		Height:       100,
		UploadStatus: domain.UploadStatusCompleted,
	}}
	useCase := New(repo, nil, &storageStub{}, &publisherStub{}, nil, nil, nil, passthroughTx{}, nil)

	_, err := useCase.UpdateCrop(context.Background(), "other-user", avatarID, domain.AvatarCrop{Size: 50})
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

func TestUpdateCropRejectsInvalidSelection(t *testing.T) {
	avatarID := uuid.New()
	repo := &avatarRepoStub{avatar: &domain.Avatar{
		ID:           avatarID,
		UserID:       "user-1",
		Width:        100,
		Height:       100,
		UploadStatus: domain.UploadStatusCompleted,
	}}
	useCase := New(repo, nil, &storageStub{}, &publisherStub{}, nil, nil, nil, passthroughTx{}, nil)

	_, err := useCase.UpdateCrop(context.Background(), "user-1", avatarID, domain.AvatarCrop{X: 90, Y: 0, Size: 20})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected invalid input, got %v", err)
	}
}

type lifecycleAvatarRepoStub struct {
	*avatarRepoStub
	remaining []*domain.Avatar
}

func (r *lifecycleAvatarRepoStub) ListByUserID(_ context.Context, _ string, _, _ int) ([]*domain.Avatar, error) {
	return r.remaining, nil
}

func (r *lifecycleAvatarRepoStub) Activate(_ context.Context, _ string, avatarID uuid.UUID) error {
	r.activateCalls++
	for _, avatar := range r.remaining {
		avatar.IsActive = avatar.ID == avatarID
	}
	return nil
}

func (r *lifecycleAvatarRepoStub) Delete(_ context.Context, _ uuid.UUID) error {
	r.avatar.IsActive = false
	return nil
}

type lifecycleThumbnailRepoStub struct{}

func (lifecycleThumbnailRepoStub) Create(context.Context, *domain.Thumbnail) error { return nil }
func (lifecycleThumbnailRepoStub) ListByAvatarID(context.Context, uuid.UUID) ([]*domain.Thumbnail, error) {
	return nil, nil
}
func (lifecycleThumbnailRepoStub) DeleteByAvatarID(context.Context, uuid.UUID) error { return nil }

func TestDeleteActivatesNewestRemainingAvatar(t *testing.T) {
	deletedID := uuid.New()
	fallbackID := uuid.New()
	repo := &lifecycleAvatarRepoStub{
		avatarRepoStub: &avatarRepoStub{avatar: &domain.Avatar{
			ID:       deletedID,
			UserID:   "user-1",
			IsActive: true,
		}},
		remaining: []*domain.Avatar{{ID: fallbackID, UserID: "user-1"}},
	}
	useCase := New(repo, lifecycleThumbnailRepoStub{}, &storageStub{}, &publisherStub{}, nil, nil, nil, passthroughTx{}, nil)

	if err := useCase.Delete(context.Background(), "user-1", deletedID); err != nil {
		t.Fatal(err)
	}
	if repo.activateCalls != 1 || !repo.remaining[0].IsActive {
		t.Fatalf("expected fallback avatar to become active: calls=%d active=%v", repo.activateCalls, repo.remaining[0].IsActive)
	}
	if repo.avatar.IsActive {
		t.Fatal("deleted avatar must not remain active")
	}
}
