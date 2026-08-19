package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/aikowocki/yandex-go-ext/internal/domain"
	"github.com/aikowocki/yandex-go-ext/internal/infra/postgres/gen"
	"github.com/google/uuid"
)

// ThumbnailRepository сохраняет миниатюры в PostgreSQL.
type ThumbnailRepository struct {
	baseRepo
}

// NewThumbnailRepository создаёт репозиторий миниатюр.
func NewThumbnailRepository(db *DB) *ThumbnailRepository {
	return &ThumbnailRepository{baseRepo: baseRepo{db: db}}
}

// Create сохраняет миниатюру.
func (r *ThumbnailRepository) Create(ctx context.Context, thumbnail *domain.Thumbnail) error {
	if thumbnail == nil || !thumbnail.Size.IsSupported() {
		return fmt.Errorf("create thumbnail: %w", domain.ErrInvalidInput)
	}
	if thumbnail.ID == uuid.Nil {
		thumbnail.ID = uuid.New()
	}
	if thumbnail.CreatedAt.IsZero() {
		thumbnail.CreatedAt = time.Now().UTC()
	}
	created, err := r.q(ctx).CreateThumbnail(ctx, gen.CreateThumbnailParams{
		ID:           toPGUUID(thumbnail.ID),
		AvatarID:     toPGUUID(thumbnail.AvatarID),
		Size:         string(thumbnail.Size),
		S3Key:        thumbnail.S3Key,
		Width:        int32(thumbnail.Width),
		Height:       int32(thumbnail.Height),
		SizeBytes:    thumbnail.SizeBytes,
		CreatedAt:    toPGTime(&thumbnail.CreatedAt),
		BlobID:       toPGUUID(thumbnail.BlobID),
		DerivationID: toPGUUID(thumbnail.DerivationID),
	})
	if err != nil {
		return mapDatabaseError("create thumbnail", err)
	}
	*thumbnail = fromGeneratedThumbnail(created)
	return nil
}

// ListByAvatarID возвращает миниатюры avatar.
func (r *ThumbnailRepository) ListByAvatarID(ctx context.Context, avatarID uuid.UUID) ([]*domain.Thumbnail, error) {
	thumbnails, err := r.q(ctx).ListThumbnailsByAvatarID(ctx, toPGUUID(avatarID))
	if err != nil {
		return nil, mapDatabaseError("list thumbnails", err)
	}
	result := make([]*domain.Thumbnail, 0, len(thumbnails))
	for _, thumbnail := range thumbnails {
		converted := fromGeneratedThumbnail(thumbnail)
		result = append(result, &converted)
	}
	return result, nil
}

// DeleteByAvatarID удаляет миниатюры avatar.
func (r *ThumbnailRepository) DeleteByAvatarID(ctx context.Context, avatarID uuid.UUID) error {
	if err := r.q(ctx).DeleteThumbnailsByAvatarID(ctx, toPGUUID(avatarID)); err != nil {
		return mapDatabaseError("delete thumbnails", err)
	}
	return nil
}

// HardDeleteExpired удаляет старые миниатюры.
func (r *ThumbnailRepository) HardDeleteExpired(ctx context.Context, deletedBefore time.Time) (int64, error) {
	result, err := r.q(ctx).HardDeleteExpiredThumbnails(ctx, toPGTime(&deletedBefore))
	if err != nil {
		return 0, mapDatabaseError("hard delete expired thumbnails", err)
	}
	return result.RowsAffected(), nil
}

func fromGeneratedThumbnail(thumbnail gen.Thumbnail) domain.Thumbnail {
	return domain.Thumbnail{
		ID:           fromPGUUID(thumbnail.ID),
		AvatarID:     fromPGUUID(thumbnail.AvatarID),
		Size:         domain.ThumbnailSize(thumbnail.Size),
		BlobID:       fromPGUUID(thumbnail.BlobID),
		DerivationID: fromPGUUID(thumbnail.DerivationID),
		S3Key:        thumbnail.S3Key,
		Width:        int(thumbnail.Width),
		Height:       int(thumbnail.Height),
		SizeBytes:    thumbnail.SizeBytes,
		CreatedAt:    thumbnail.CreatedAt.Time,
		DeletedAt:    fromPGTime(thumbnail.DeletedAt),
	}
}
