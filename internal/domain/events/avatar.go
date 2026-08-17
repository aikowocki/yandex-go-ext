package events

import (
	"time"

	"github.com/aikowocki/yandex-go-ext/internal/domain"
)

// События для брокера сообщений.

// ProcessingOperation описывает операцию обработки avatar.
type ProcessingOperation string

const (
	// ProcessingOperationThumbnail100 — операция создания миниатюры 100x100.
	ProcessingOperationThumbnail100 ProcessingOperation = "thumbnail:100x100"
	// ProcessingOperationThumbnail300 — операция создания миниатюры 300x300.
	ProcessingOperationThumbnail300 ProcessingOperation = "thumbnail:300x300"
)

// AvatarProcessEvent описывает запланированные операции обработки avatar.
type AvatarProcessEvent struct {
	AvatarID   string                `json:"avatar_id"`
	Operations []ProcessingOperation `json:"operations"`
}

// AvatarUploadedEvent сообщает о загрузке исходного avatar.
type AvatarUploadedEvent struct {
	AvatarID   string                `json:"avatar_id"`
	UserID     string                `json:"user_id"`
	S3Key      string                `json:"s3_key"`
	MimeType   string                `json:"mime_type"`
	Operations []ProcessingOperation `json:"operations"`
	Timestamp  time.Time             `json:"timestamp"`
}

// NewAvatarProcessEvent создаёт событие с полным набором операций обработки.
func NewAvatarProcessEvent(avatar *domain.Avatar) *AvatarProcessEvent {
	return &AvatarProcessEvent{
		AvatarID: avatar.ID.String(),
		Operations: []ProcessingOperation{
			ProcessingOperationThumbnail100,
			ProcessingOperationThumbnail300,
		},
	}
}

// NewAvatarUploadedEvent создаёт событие об успешно загруженном avatar.
func NewAvatarUploadedEvent(avatar *domain.Avatar) *AvatarUploadedEvent {
	process := NewAvatarProcessEvent(avatar)
	return &AvatarUploadedEvent{
		AvatarID:   process.AvatarID,
		UserID:     avatar.UserID,
		S3Key:      avatar.S3KeyOriginal,
		MimeType:   avatar.MimeType,
		Operations: process.Operations,
		Timestamp:  time.Now(),
	}
}

// AvatarDeletedEvent сообщает об удалении avatar и связанных объектов.
type AvatarDeletedEvent struct {
	AvatarID  string    `json:"avatar_id"`
	UserID    string    `json:"user_id"`
	S3Keys    []string  `json:"s3_keys"` // оригинал + миниатюры
	Timestamp time.Time `json:"timestamp"`
}

// NewAvatarDeletedEvent создаёт событие об удалённом avatar.
func NewAvatarDeletedEvent(avatar *domain.Avatar, thumbnailKeys []string) *AvatarDeletedEvent {
	s3Keys := []string{avatar.S3KeyOriginal}
	s3Keys = append(s3Keys, thumbnailKeys...)

	return &AvatarDeletedEvent{
		AvatarID:  avatar.ID.String(),
		UserID:    avatar.UserID,
		S3Keys:    s3Keys,
		Timestamp: time.Now(),
	}
}
