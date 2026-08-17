package contracts

import (
	"context"

	"github.com/aikowocki/yandex-go-ext/internal/domain"
	"github.com/google/uuid"
)

// AvatarRepository хранит и изменяет avatars.
type AvatarRepository interface {
	Create(ctx context.Context, avatar *domain.Avatar) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Avatar, error)
	GetByUserID(ctx context.Context, userID string) (*domain.Avatar, error)
	ListByUserID(ctx context.Context, userID string, limit, offset int) ([]*domain.Avatar, error)
	Update(ctx context.Context, avatar *domain.Avatar) error
	Delete(ctx context.Context, id uuid.UUID) error

	GetPendingForProcessing(ctx context.Context, limit int) ([]*domain.Avatar, error)
	UpdateProcessingStatus(ctx context.Context, id uuid.UUID, status domain.ProcessingStatus) error
}

// AvatarProcessorClaimer атомарно забирает avatar на обработку.
type AvatarProcessorClaimer interface {
	ClaimForProcessing(ctx context.Context, id uuid.UUID) (*domain.Avatar, error)
}

// AvatarDedupRepository поддерживает поиск и активацию дубликатов.
type AvatarDedupRepository interface {
	FindByUserAndSourceBlobID(ctx context.Context, userID string, sourceBlobID uuid.UUID) (*domain.Avatar, error)
	Activate(ctx context.Context, userID string, avatarID uuid.UUID) error
}

// ThumbnailRepository хранит и удаляет миниатюры avatar.
type ThumbnailRepository interface {
	Create(ctx context.Context, thumbnail *domain.Thumbnail) error
	ListByAvatarID(ctx context.Context, avatarID uuid.UUID) ([]*domain.Thumbnail, error)
	DeleteByAvatarID(ctx context.Context, avatarID uuid.UUID) error
}
