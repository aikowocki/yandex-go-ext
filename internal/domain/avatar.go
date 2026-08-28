package domain

import (
	"time"

	"github.com/google/uuid"
)

// Avatar описывает загруженное пользователем изображение.
type Avatar struct {
	ID            uuid.UUID
	UserID        string
	FileName      string
	MimeType      string
	SizeBytes     int64
	Width         int
	Height        int
	SourceBlobID  uuid.UUID
	S3KeyOriginal string
	CropX         float64
	CropY         float64
	CropSize      float64

	UploadStatus     UploadStatus
	ProcessingStatus ProcessingStatus

	ProcessingStartedAt *time.Time
	ProcessingAttempts  int
	LastError           string

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
	IsActive  bool
}

// AvatarCrop содержит параметры кадрирования изображения.
type AvatarCrop struct {
	X    float64
	Y    float64
	Size float64
}

// UploadStatus описывает состояние загрузки исходного файла.
type UploadStatus string

const (
	// UploadStatusUploading означает, что файл ещё загружается.
	UploadStatusUploading UploadStatus = "uploading"
	// UploadStatusCompleted означает, что файл загружен.
	UploadStatusCompleted UploadStatus = "completed"
	// UploadStatusFailed означает, что загрузка завершилась ошибкой.
	UploadStatusFailed UploadStatus = "failed"
)

// ProcessingStatus описывает состояние обработки avatar.
type ProcessingStatus string

const (
	// ProcessingStatusPending означает, что обработка ожидает запуска.
	ProcessingStatusPending ProcessingStatus = "pending"
	// ProcessingStatusProcessing означает, что avatar обрабатывается.
	ProcessingStatusProcessing ProcessingStatus = "processing"
	// ProcessingStatusCompleted означает, что обработка завершена.
	ProcessingStatusCompleted ProcessingStatus = "completed"
	// ProcessingStatusFailed означает, что обработка завершилась ошибкой.
	ProcessingStatusFailed ProcessingStatus = "failed"
)

// IsProcessable проверяет, можно ли начать обработку avatar.
func (a *Avatar) IsProcessable() bool {
	return a != nil && a.ProcessingStatus == ProcessingStatusPending && a.UploadStatus == UploadStatusCompleted
}

// MarkProcessing переводит avatar в состояние обработки.
func (a *Avatar) MarkProcessing() {
	if a == nil {
		return
	}

	a.ProcessingStatus = ProcessingStatusProcessing
	now := time.Now().UTC()
	a.ProcessingStartedAt = &now
	a.ProcessingAttempts++
	a.UpdatedAt = now
}

// MarkProcessed отмечает успешное завершение обработки avatar.
func (a *Avatar) MarkProcessed() {
	if a == nil {
		return
	}

	a.ProcessingStatus = ProcessingStatusCompleted
	a.UpdatedAt = time.Now().UTC()
}

// MarkFailed отмечает ошибку обработки avatar.
func (a *Avatar) MarkFailed(err error) {
	if a == nil {
		return
	}

	a.ProcessingStatus = ProcessingStatusFailed
	if err != nil {
		a.LastError = err.Error()
	}
	a.UpdatedAt = time.Now().UTC()
}
