package avatar

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"testing"
	"time"

	"github.com/aikowocki/yandex-go-ext/internal/domain"
	imageprocessor "github.com/aikowocki/yandex-go-ext/internal/usecase/image"
	"github.com/google/uuid"
)

type blobRepoStub struct {
	blob *domain.Blob
}

func (r *blobRepoStub) GetOrCreate(_ context.Context, _ *domain.Blob) (*domain.Blob, error) {
	return r.blob, nil
}
func (r *blobRepoStub) MarkReady(_ context.Context, _ uuid.UUID) error            { return nil }
func (r *blobRepoStub) MarkFailed(_ context.Context, _ uuid.UUID, _ string) error { return nil }
func (r *blobRepoStub) EnsureDerivation(_ context.Context, derivation *domain.BlobDerivation) (*domain.BlobDerivation, error) {
	if derivation.ID == uuid.Nil {
		derivation.ID = uuid.New()
	}
	return derivation, nil
}
func (r *blobRepoStub) LinkThumbnail(_ context.Context, _, _, _ uuid.UUID) error { return nil }

func TestCreateReusesReadySourceBlob(t *testing.T) {
	content := validJPEG(t)
	blobID := uuid.New()
	blob := &domain.Blob{
		ID:            blobID,
		SHA256:        make([]byte, 32),
		SizeBytes:     int64(len(content)),
		MimeType:      "image/jpeg",
		Width:         10,
		Height:        10,
		ObjectKey:     "blobs/existing",
		StorageStatus: domain.BlobStorageStatusReady,
		CreatedAt:     time.Now().UTC(),
	}
	storage := &storageStub{}
	useCase := New(&avatarRepoStub{}, nil, storage, &publisherStub{}, imageprocessor.NewProcessor(), nil, &blobRepoStub{blob: blob}, passthroughTx{}, nil)

	avatar, err := useCase.Create(context.Background(), "user-1", multipartFileHeader(t, content))
	if err != nil {
		t.Fatal(err)
	}
	if avatar.SourceBlobID != blobID {
		t.Fatalf("expected source blob %s, got %s", blobID, avatar.SourceBlobID)
	}
	if avatar.S3KeyOriginal != blob.ObjectKey {
		t.Fatalf("expected reused object key %q, got %q", blob.ObjectKey, avatar.S3KeyOriginal)
	}
	if storage.key != "" {
		t.Fatalf("ready source blob was uploaded again with key %q", storage.key)
	}
}

func validJPEG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	var content bytes.Buffer
	if err := jpeg.Encode(&content, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	return content.Bytes()
}

func TestCreateReactivatesExistingSoftDeletedAvatar(t *testing.T) {
	content := validJPEG(t)
	blobID := uuid.New()
	avatarID := uuid.New()
	deletedAt := time.Now().UTC().Add(-time.Hour)
	blob := &domain.Blob{
		ID:            blobID,
		SHA256:        make([]byte, 32),
		SizeBytes:     int64(len(content)),
		MimeType:      "image/jpeg",
		Width:         10,
		Height:        10,
		ObjectKey:     "blobs/existing",
		StorageStatus: domain.BlobStorageStatusReady,
	}
	repo := &avatarRepoStub{dedupAvatar: &domain.Avatar{
		ID:               avatarID,
		UserID:           "user-1",
		FileName:         "avatar.jpg",
		MimeType:         "image/jpeg",
		SourceBlobID:     blobID,
		S3KeyOriginal:    blob.ObjectKey,
		UploadStatus:     domain.UploadStatusCompleted,
		ProcessingStatus: domain.ProcessingStatusCompleted,
		DeletedAt:        &deletedAt,
	}}
	storage := &storageStub{}
	useCase := New(repo, nil, storage, &publisherStub{}, imageprocessor.NewProcessor(), nil, &blobRepoStub{blob: blob}, passthroughTx{}, nil)

	avatar, err := useCase.Create(context.Background(), "user-1", multipartFileHeader(t, content))
	if err != nil {
		t.Fatal(err)
	}
	if avatar.ID != avatarID {
		t.Fatalf("expected existing avatar %s, got %s", avatarID, avatar.ID)
	}
	if repo.createCalls != 0 {
		t.Fatalf("expected no new avatar row, got %d create calls", repo.createCalls)
	}
	if repo.activateCalls != 1 || repo.dedupAvatar.DeletedAt != nil {
		t.Fatalf("expected one reactivation, calls=%d deleted_at=%v", repo.activateCalls, repo.dedupAvatar.DeletedAt)
	}
	if storage.key != "" {
		t.Fatalf("existing source blob was uploaded again with key %q", storage.key)
	}
}
