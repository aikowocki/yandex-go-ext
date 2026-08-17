package dto

import (
	"fmt"
	"time"

	"github.com/aikowocki/yandex-go-ext/internal/domain"
)

// UploadResponse возвращается после загрузки avatar.
type UploadResponse struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	URL       string    `json:"url"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// DimensionsResponse содержит размеры изображения.
type DimensionsResponse struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

// CropResponse содержит параметры кадрирования изображения.
type CropResponse struct {
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
	Size float64 `json:"size"`
}

// AvatarResponse представляет avatar в REST API.
type AvatarResponse struct {
	ID         string              `json:"id"`
	UserID     string              `json:"user_id"`
	FileName   string              `json:"file_name"`
	MimeType   string              `json:"mime_type"`
	Size       int64               `json:"size"`
	Dimensions DimensionsResponse  `json:"dimensions"`
	Crop       CropResponse        `json:"crop"`
	Thumbnails []ThumbnailResponse `json:"thumbnails"`
	Status     string              `json:"status"`
	IsActive   bool                `json:"is_active"`
	CreatedAt  time.Time           `json:"created_at"`
	UpdatedAt  time.Time           `json:"updated_at"`
}

// ThumbnailResponse представляет миниатюру в REST API.
type ThumbnailResponse struct {
	Size string `json:"size"`
	URL  string `json:"url"`
}

// UploadFromDomain преобразует domain avatar в ответ загрузки.
func UploadFromDomain(avatar *domain.Avatar) UploadResponse {
	return UploadResponse{
		ID:        avatar.ID.String(),
		UserID:    avatar.UserID,
		URL:       fmt.Sprintf("/api/v1/avatars/%s", avatar.ID),
		Status:    processingStatus(avatar.ProcessingStatus),
		CreatedAt: avatar.CreatedAt,
	}
}

// AvatarFromDomain преобразует domain avatar в REST-ответ.
func AvatarFromDomain(avatar *domain.Avatar) AvatarResponse {
	return AvatarResponse{
		ID:       avatar.ID.String(),
		UserID:   avatar.UserID,
		FileName: avatar.FileName,
		MimeType: avatar.MimeType,
		Size:     avatar.SizeBytes,
		Dimensions: DimensionsResponse{
			Width:  avatar.Width,
			Height: avatar.Height,
		},
		Crop: CropResponse{
			X:    avatar.CropX,
			Y:    avatar.CropY,
			Size: avatar.CropSize,
		},
		Status:    processingStatus(avatar.ProcessingStatus),
		IsActive:  avatar.IsActive,
		CreatedAt: avatar.CreatedAt,
		UpdatedAt: avatar.UpdatedAt,
	}
}

// ThumbnailFromDomain преобразует domain thumbnail в REST-ответ.
func ThumbnailFromDomain(thumbnail *domain.Thumbnail, url string) ThumbnailResponse {
	return ThumbnailResponse{Size: string(thumbnail.Size), URL: url}
}

func processingStatus(status domain.ProcessingStatus) string {
	if status == domain.ProcessingStatusPending {
		return string(domain.ProcessingStatusProcessing)
	}
	return string(status)
}
