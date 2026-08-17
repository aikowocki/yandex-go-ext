package domain

import (
	"time"

	"github.com/google/uuid"
)

// Thumbnail описывает миниатюру avatar.
type Thumbnail struct {
	ID           uuid.UUID
	AvatarID     uuid.UUID
	Size         ThumbnailSize
	BlobID       uuid.UUID
	DerivationID uuid.UUID
	S3Key        string
	Width        int
	Height       int
	SizeBytes    int64
	CreatedAt    time.Time
	DeletedAt    *time.Time
}

// ThumbnailSize задаёт поддерживаемый размер миниатюры.
type ThumbnailSize string

const (
	// ThumbnailSize100x100 — миниатюра размером 100x100.
	ThumbnailSize100x100 ThumbnailSize = "100x100"
	// ThumbnailSize300x300 — миниатюра размером 300x300.
	ThumbnailSize300x300 ThumbnailSize = "300x300"
)

// SupportedThumbnailSizes содержит все поддерживаемые размеры миниатюр.
var SupportedThumbnailSizes = []ThumbnailSize{
	ThumbnailSize100x100,
	ThumbnailSize300x300,
}

// Dimensions возвращает ширину и высоту размера миниатюры.
func (s ThumbnailSize) Dimensions() (width, height int, ok bool) {
	switch s {
	case ThumbnailSize100x100:
		return 100, 100, true
	case ThumbnailSize300x300:
		return 300, 300, true
	default:
		return 0, 0, false
	}
}

// IsSupported проверяет, поддерживается ли размер миниатюры.
func (s ThumbnailSize) IsSupported() bool {
	_, _, ok := s.Dimensions()
	return ok
}
