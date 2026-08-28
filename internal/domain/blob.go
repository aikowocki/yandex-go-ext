package domain

import (
	"time"

	"github.com/google/uuid"
)

// Blob описывает уникальный объект изображения в хранилище.
type Blob struct {
	ID            uuid.UUID
	SHA256        []byte
	SizeBytes     int64
	MimeType      string
	Width         int
	Height        int
	ObjectKey     string
	StorageStatus BlobStorageStatus
	LastError     string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// BlobStorageStatus описывает состояние blob в хранилище.
type BlobStorageStatus string

const (
	// BlobStorageStatusUploading означает, что blob загружается.
	BlobStorageStatusUploading BlobStorageStatus = "uploading"
	// BlobStorageStatusReady означает, что blob доступен.
	BlobStorageStatusReady BlobStorageStatus = "ready"
	// BlobStorageStatusDeleting означает, что blob готовится к удалению.
	BlobStorageStatusDeleting BlobStorageStatus = "deleting"
)

// IsReady проверяет доступность blob.
func (b *Blob) IsReady() bool {
	return b != nil && b.StorageStatus == BlobStorageStatusReady
}

// BlobDerivation описывает производный blob для миниатюры.
type BlobDerivation struct {
	ID               uuid.UUID
	ParentBlobID     uuid.UUID
	DerivedBlobID    uuid.UUID
	Variant          ThumbnailSize
	ProcessorVersion string
	OutputFormat     string
	Status           BlobStorageStatus
	CreatedAt        time.Time
}
