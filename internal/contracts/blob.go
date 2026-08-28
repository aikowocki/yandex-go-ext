package contracts

import (
	"context"
	"time"

	"github.com/aikowocki/yandex-go-ext/internal/domain"
	"github.com/google/uuid"
)

// BlobRepository управляет blob и их производными записями.
type BlobRepository interface {
	GetOrCreate(ctx context.Context, blob *domain.Blob) (*domain.Blob, error)
	MarkReady(ctx context.Context, id uuid.UUID) error
	MarkFailed(ctx context.Context, id uuid.UUID, message string) error
	EnsureDerivation(ctx context.Context, derivation *domain.BlobDerivation) (*domain.BlobDerivation, error)
	LinkThumbnail(ctx context.Context, thumbnailID, blobID, derivationID uuid.UUID) error
}

// BlobRetentionRepository управляет поиском и удалением неиспользуемых blob.
type BlobRetentionRepository interface {
	ListUnreferenced(ctx context.Context, createdBefore time.Time, limit int) ([]*domain.Blob, error)
	ClaimForDeletion(ctx context.Context, id uuid.UUID) (*domain.Blob, bool, error)
	Delete(ctx context.Context, id uuid.UUID) error
	HasObject(ctx context.Context, objectKey string) (bool, error)
}

// AvatarRetentionRepository удаляет avatars, истёкших по сроку хранения.
type AvatarRetentionRepository interface {
	HardDeleteExpired(ctx context.Context, deletedBefore time.Time) (int64, error)
}

// ThumbnailRetentionRepository удаляет миниатюры, истёкшие по сроку хранения.
type ThumbnailRetentionRepository interface {
	HardDeleteExpired(ctx context.Context, deletedBefore time.Time) (int64, error)
}
