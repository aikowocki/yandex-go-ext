package avatar

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"mime/multipart"
	"strings"
	"time"

	"github.com/aikowocki/yandex-go-ext/internal/contracts"
	"github.com/aikowocki/yandex-go-ext/internal/domain"
	"github.com/aikowocki/yandex-go-ext/internal/domain/events"
	"github.com/aikowocki/yandex-go-ext/internal/shared/logging"
	"github.com/google/uuid"
)

const maxAvatarSize int64 = 10 * 1024 * 1024

// UseCase содержит бизнес-логику работы с avatar.
type UseCase struct {
	avatarRepo    contracts.AvatarRepository
	thumbnailRepo contracts.ThumbnailRepository
	storage       contracts.ObjectStorage
	broker        contracts.Publisher
	processor     contracts.ImageProcessor
	outbox        contracts.OutboxRepository
	blobRepo      contracts.BlobRepository
}

// New создаёт use case для работы с avatar.
func New(
	avatarRepo contracts.AvatarRepository,
	thumbnailRepo contracts.ThumbnailRepository,
	storage contracts.ObjectStorage,
	broker contracts.Publisher,
	processor contracts.ImageProcessor,
	outbox contracts.OutboxRepository,
	dependencies ...any,
) *UseCase {
	useCase := &UseCase{
		avatarRepo:    avatarRepo,
		thumbnailRepo: thumbnailRepo,
		storage:       storage,
		broker:        broker,
		processor:     processor,
		outbox:        outbox,
	}
	for _, dependency := range dependencies {
		if blobRepo, ok := dependency.(contracts.BlobRepository); ok {
			useCase.blobRepo = blobRepo
		}
	}
	return useCase
}

// Create загружает файл и создаёт avatar пользователя.
func (uc *UseCase) Create(ctx context.Context, userID string, file *multipart.FileHeader, requestedCrop ...domain.AvatarCrop) (*domain.Avatar, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, fmt.Errorf("user id: %w", domain.ErrInvalidInput)
	}
	if err := validateFileHeader(file); err != nil {
		return nil, fmt.Errorf("validate file: %w", err)
	}

	src, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	defer func() { _ = src.Close() }()

	format, err := uc.validateContent(src, file.Header.Get("Content-Type"))
	if err != nil {
		return nil, fmt.Errorf("validate image content: %w", err)
	}
	if _, err := src.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("rewind file: %w", err)
	}

	img, decodedFormat, err := uc.processor.Decode(src)
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}
	if decodedFormat != format {
		return nil, fmt.Errorf("image format changed during decode: %w", domain.ErrInvalidFormat)
	}
	bounds := img.Bounds()
	if bounds.Dx() <= 0 || bounds.Dy() <= 0 {
		return nil, fmt.Errorf("image dimensions: %w", domain.ErrInvalidFormat)
	}
	crop := domain.AvatarCrop{}
	if len(requestedCrop) > 0 {
		crop = requestedCrop[0]
	}
	crop, err = normalizeCrop(crop, bounds.Dx(), bounds.Dy())
	if err != nil {
		return nil, fmt.Errorf("validate crop: %w", err)
	}
	if _, err := src.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("rewind file for upload: %w", err)
	}

	var sourceBlob *domain.Blob
	s3KeyOriginal := ""
	if uc.blobRepo != nil {
		digest, err := hashSource(src)
		if err != nil {
			return nil, fmt.Errorf("hash original: %w", err)
		}
		if _, err := src.Seek(0, io.SeekStart); err != nil {
			return nil, fmt.Errorf("rewind file after hashing: %w", err)
		}
		sourceBlob, err = uc.blobRepo.GetOrCreate(ctx, &domain.Blob{
			ID:            uuid.New(),
			SHA256:        digest,
			SizeBytes:     file.Size,
			MimeType:      mimeTypeForFormat(format),
			Width:         bounds.Dx(),
			Height:        bounds.Dy(),
			ObjectKey:     generateBlobKey(digest),
			StorageStatus: domain.BlobStorageStatusUploading,
		})
		if err != nil {
			return nil, fmt.Errorf("get or create source blob: %w", err)
		}
		if sourceBlob.StorageStatus == domain.BlobStorageStatusDeleting {
			return nil, fmt.Errorf("source blob cleanup is in progress: %w", domain.ErrConflict)
		}
		s3KeyOriginal = sourceBlob.ObjectKey
	}

	var avatarDedup contracts.AvatarDedupRepository
	if sourceBlob != nil {
		avatarDedup, _ = uc.avatarRepo.(contracts.AvatarDedupRepository)
		if avatarDedup != nil {
			existing, findErr := avatarDedup.FindByUserAndSourceBlobID(ctx, userID, sourceBlob.ID)
			if findErr != nil && !errors.Is(findErr, domain.ErrNotFound) {
				return nil, fmt.Errorf("find existing avatar: %w", findErr)
			}
			if existing != nil && existing.UploadStatus == domain.UploadStatusCompleted && sourceBlob.IsReady() {
				if err := avatarDedup.Activate(ctx, userID, existing.ID); err != nil {
					return nil, fmt.Errorf("reactivate existing avatar: %w", err)
				}
				existing.DeletedAt = nil
				existing.IsActive = true
				existing.CropX = crop.X
				existing.CropY = crop.Y
				existing.CropSize = crop.Size
				existing.ProcessingStatus = domain.ProcessingStatusPending
				existing.ProcessingStartedAt = nil
				existing.LastError = ""
				existing.UpdatedAt = time.Now().UTC()
				if err := uc.avatarRepo.Update(ctx, existing); err != nil {
					return nil, fmt.Errorf("update existing avatar crop: %w", err)
				}
				if err := uc.publishUploadedEvent(ctx, existing); err != nil {
					return nil, fmt.Errorf("queue existing avatar processing: %w", err)
				}
				return existing, nil
			}
		}
	}

	avatar := &domain.Avatar{
		ID:               uuid.New(),
		UserID:           userID,
		FileName:         file.Filename,
		MimeType:         mimeTypeForFormat(format),
		SizeBytes:        file.Size,
		Width:            bounds.Dx(),
		Height:           bounds.Dy(),
		CropX:            crop.X,
		CropY:            crop.Y,
		CropSize:         crop.Size,
		SourceBlobID:     uuid.Nil,
		S3KeyOriginal:    s3KeyOriginal,
		UploadStatus:     domain.UploadStatusUploading,
		ProcessingStatus: domain.ProcessingStatusPending,
	}
	if sourceBlob != nil {
		avatar.SourceBlobID = sourceBlob.ID
	} else {
		avatar.S3KeyOriginal = generateS3Key(userID, avatar.ID, format)
	}

	if err := uc.avatarRepo.Create(ctx, avatar); err != nil {
		return nil, fmt.Errorf("create avatar metadata: %w", err)
	}

	if sourceBlob == nil || !sourceBlob.IsReady() {
		if err := uc.storage.Upload(ctx, avatar.S3KeyOriginal, src, file.Size, avatar.MimeType); err != nil {
			if sourceBlob != nil {
				if markErr := uc.blobRepo.MarkFailed(ctx, sourceBlob.ID, err.Error()); markErr != nil {
					logging.Warn(ctx, "failed to mark source blob as failed", logging.Err(markErr), logging.UUID("blob_id", sourceBlob.ID))
				}
			}
			avatar.MarkFailed(err)
			if updateErr := uc.avatarRepo.Update(ctx, avatar); updateErr != nil {
				logging.Warn(ctx, "failed to mark avatar upload as failed", logging.Err(updateErr), logging.UUID("avatar_id", avatar.ID))
			}
			return nil, fmt.Errorf("upload original: %w", err)
		}
		if sourceBlob != nil {
			if err := uc.blobRepo.MarkReady(ctx, sourceBlob.ID); err != nil {
				avatar.MarkFailed(err)
				if updateErr := uc.avatarRepo.Update(ctx, avatar); updateErr != nil {
					logging.Warn(ctx, "failed to mark avatar after source blob failure", logging.Err(updateErr), logging.UUID("avatar_id", avatar.ID))
				}
				return nil, fmt.Errorf("mark source blob ready: %w", err)
			}
		}
	}

	avatar.UploadStatus = domain.UploadStatusCompleted
	if err := uc.avatarRepo.Update(ctx, avatar); err != nil {
		if sourceBlob == nil {
			if deleteErr := uc.storage.Delete(ctx, avatar.S3KeyOriginal); deleteErr != nil {
				logging.Warn(ctx, "failed to clean up uploaded original", logging.Err(deleteErr), logging.UUID("avatar_id", avatar.ID))
			}
		} else {
			logging.Warn(ctx, "leaving shared source blob for reconciliation", logging.UUID("avatar_id", avatar.ID), logging.UUID("blob_id", sourceBlob.ID))
		}
		return nil, fmt.Errorf("mark avatar uploaded: %w", err)
	}

	if avatarDedup != nil {
		if err := avatarDedup.Activate(ctx, userID, avatar.ID); err != nil {
			return nil, fmt.Errorf("activate uploaded avatar: %w", err)
		}
		avatar.IsActive = true
	}

	if err := uc.publishUploadedEvent(ctx, avatar); err != nil {
		return nil, fmt.Errorf("queue avatar uploaded event: %w", err)
	}
	return avatar, nil
}

// UpdateCrop обновляет кадрирование avatar и ставит его на повторную обработку.
func (uc *UseCase) UpdateCrop(ctx context.Context, userID string, id uuid.UUID, requestedCrop domain.AvatarCrop) (*domain.Avatar, error) {
	if strings.TrimSpace(userID) == "" || id == uuid.Nil {
		return nil, domain.ErrInvalidInput
	}
	avatar, err := uc.avatarRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if avatar.UserID != userID {
		return nil, domain.ErrForbidden
	}
	if avatar.UploadStatus != domain.UploadStatusCompleted || avatar.Width <= 0 || avatar.Height <= 0 {
		return nil, domain.ErrConflict
	}
	crop, err := normalizeCrop(requestedCrop, avatar.Width, avatar.Height)
	if err != nil {
		return nil, fmt.Errorf("validate crop: %w", err)
	}

	if avatarDedup, ok := uc.avatarRepo.(contracts.AvatarDedupRepository); ok {
		if err := avatarDedup.Activate(ctx, userID, id); err != nil {
			return nil, fmt.Errorf("activate edited avatar: %w", err)
		}
		avatar.IsActive = true
	}
	avatar.CropX = crop.X
	avatar.CropY = crop.Y
	avatar.CropSize = crop.Size
	avatar.ProcessingStatus = domain.ProcessingStatusPending
	avatar.ProcessingStartedAt = nil
	avatar.LastError = ""
	avatar.UpdatedAt = time.Now().UTC()
	if err := uc.avatarRepo.Update(ctx, avatar); err != nil {
		return nil, fmt.Errorf("update avatar crop: %w", err)
	}
	if err := uc.publishUploadedEvent(ctx, avatar); err != nil {
		return nil, fmt.Errorf("queue avatar crop processing: %w", err)
	}
	return avatar, nil
}

// Get возвращает avatar по идентификатору.
func (uc *UseCase) Get(ctx context.Context, id uuid.UUID) (*domain.Avatar, error) {
	if id == uuid.Nil {
		return nil, domain.ErrInvalidInput
	}
	return uc.avatarRepo.GetByID(ctx, id)
}

// GetByUserID возвращает avatar пользователя.
func (uc *UseCase) GetByUserID(ctx context.Context, userID string) (*domain.Avatar, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, domain.ErrInvalidInput
	}
	return uc.avatarRepo.GetByUserID(ctx, userID)
}

// List возвращает страницу avatar пользователя.
func (uc *UseCase) List(ctx context.Context, userID string, limit, offset int) ([]*domain.Avatar, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, domain.ErrInvalidInput
	}
	return uc.avatarRepo.ListByUserID(ctx, userID, limit, offset)
}

// Delete удаляет avatar пользователя и публикует событие удаления.
func (uc *UseCase) Delete(ctx context.Context, userID string, id uuid.UUID) error {
	if strings.TrimSpace(userID) == "" || id == uuid.Nil {
		return domain.ErrInvalidInput
	}
	avatar, err := uc.avatarRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if avatar.UserID != userID {
		return domain.ErrForbidden
	}

	wasActive := avatar.IsActive
	thumbnails, err := uc.thumbnailRepo.ListByAvatarID(ctx, id)
	if err != nil {
		return fmt.Errorf("list thumbnails: %w", err)
	}
	thumbnailKeys := make([]string, 0, len(thumbnails))
	for _, thumbnail := range thumbnails {
		thumbnailKeys = append(thumbnailKeys, thumbnail.S3Key)
	}

	if err := uc.avatarRepo.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete avatar metadata: %w", err)
	}
	if err := uc.thumbnailRepo.DeleteByAvatarID(ctx, id); err != nil {
		return fmt.Errorf("delete thumbnail metadata: %w", err)
	}

	if wasActive {
		remaining, err := uc.avatarRepo.ListByUserID(ctx, userID, 1, 0)
		if err != nil {
			return fmt.Errorf("find fallback avatar: %w", err)
		}
		if len(remaining) > 0 {
			fallback := remaining[0]
			fallback.IsActive = true
			if avatarDedup, ok := uc.avatarRepo.(contracts.AvatarDedupRepository); ok {
				if err := avatarDedup.Activate(ctx, userID, fallback.ID); err != nil {
					return fmt.Errorf("activate fallback avatar: %w", err)
				}
			} else if err := uc.avatarRepo.Update(ctx, fallback); err != nil {
				return fmt.Errorf("activate fallback avatar: %w", err)
			}
		}
	}

	event := events.NewAvatarDeletedEvent(avatar, thumbnailKeys)
	payload, err := marshalEvent(event)
	if err != nil {
		return err
	}
	if err := publishEvent(ctx, uc.broker, uc.outbox, "avatar.deleted", payload); err != nil {
		return fmt.Errorf("publish deletion event: %w", err)
	}
	return nil
}

func validateFileHeader(file *multipart.FileHeader) error {
	if file == nil {
		return domain.ErrInvalidInput
	}
	if file.Size <= 0 {
		return domain.ErrInvalidFormat
	}
	if file.Size > maxAvatarSize {
		return domain.ErrFileTooLarge
	}
	contentType := strings.ToLower(strings.TrimSpace(file.Header.Get("Content-Type")))
	switch contentType {
	case "image/jpeg", "image/jpg", "image/png", "image/webp":
		return nil
	default:
		return domain.ErrInvalidFormat
	}
}

func (uc *UseCase) validateContent(src multipart.File, declaredType string) (string, error) {
	magic := make([]byte, 512)
	n, err := src.Read(magic)
	if err != nil && err != io.EOF {
		return "", err
	}
	format, err := uc.processor.ValidateMagicBytes(magic[:n])
	if err != nil {
		return "", domain.ErrInvalidFormat
	}
	declaredFormat := formatForMimeType(declaredType)
	if declaredFormat == "" || declaredFormat != format {
		return "", domain.ErrInvalidFormat
	}
	return format, nil
}

func generateS3Key(userID string, avatarID uuid.UUID, format string) string {
	return fmt.Sprintf("avatars/originals/%s/%s.%s", userID, avatarID.String(), format)
}

func generateBlobKey(digest []byte) string {
	return "blobs/" + hex.EncodeToString(digest)
}

func hashSource(src io.ReadSeeker) ([]byte, error) {
	if _, err := src.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	hasher := sha256.New()
	if _, err := io.Copy(hasher, src); err != nil {
		return nil, err
	}
	return hasher.Sum(nil), nil
}

func (uc *UseCase) publishUploadedEvent(ctx context.Context, avatar *domain.Avatar) error {
	event := events.NewAvatarUploadedEvent(avatar)
	payload, err := marshalEvent(event)
	if err != nil {
		return err
	}
	return publishEvent(ctx, uc.broker, uc.outbox, "avatar.uploaded", payload)
}

func formatForMimeType(mimeType string) string {
	switch strings.ToLower(strings.TrimSpace(mimeType)) {
	case "image/jpeg", "image/jpg":
		return "jpeg"
	case "image/png":
		return "png"
	case "image/webp":
		return "webp"
	default:
		return ""
	}
}

func mimeTypeForFormat(format string) string {
	if format == "jpg" {
		return "image/jpeg"
	}
	return "image/" + format
}

func normalizeCrop(crop domain.AvatarCrop, width, height int) (domain.AvatarCrop, error) {
	if width <= 0 || height <= 0 {
		return domain.AvatarCrop{}, domain.ErrInvalidFormat
	}
	if crop.Size == 0 && crop.X == 0 && crop.Y == 0 {
		size := float64(width)
		if height < width {
			size = float64(height)
		}
		return domain.AvatarCrop{
			X:    (float64(width) - size) / 2,
			Y:    (float64(height) - size) / 2,
			Size: size,
		}, nil
	}
	if math.IsNaN(crop.X) || math.IsNaN(crop.Y) || math.IsNaN(crop.Size) ||
		math.IsInf(crop.X, 0) || math.IsInf(crop.Y, 0) || math.IsInf(crop.Size, 0) ||
		crop.X < 0 || crop.Y < 0 || crop.Size <= 0 ||
		crop.Size > float64(width) || crop.Size > float64(height) ||
		crop.X+crop.Size > float64(width) || crop.Y+crop.Size > float64(height) {
		return domain.AvatarCrop{}, domain.ErrInvalidInput
	}
	return crop, nil
}
