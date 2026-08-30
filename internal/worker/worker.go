package worker

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/aikowocki/yandex-go-ext/internal/contracts"
	"github.com/aikowocki/yandex-go-ext/internal/domain"
	"github.com/aikowocki/yandex-go-ext/internal/domain/events"
	"github.com/aikowocki/yandex-go-ext/internal/infra/observability"
	"github.com/aikowocki/yandex-go-ext/internal/shared/logging"
	"github.com/disintegration/imaging"
	"github.com/google/uuid"
)

// Worker обрабатывает события загрузки и удаления avatar.
type Worker struct {
	logger logging.Logger

	avatarRepo    contracts.AvatarRepository
	thumbnailRepo contracts.ThumbnailRepository
	storage       contracts.ObjectStorage
	processor     contracts.ImageProcessor
	broker        contracts.Consumer
	publisher     contracts.Publisher
	outbox        contracts.OutboxRepository
	blobRepo      contracts.BlobRepository
	tx            contracts.TxManager
}

// NewWorker создаёт worker обработки avatar.
func NewWorker(
	avatarRepo contracts.AvatarRepository,
	thumbnailRepo contracts.ThumbnailRepository,
	storage contracts.ObjectStorage,
	processor contracts.ImageProcessor,
	broker contracts.Consumer,
	tx contracts.TxManager,
	logger logging.Logger,
	dependencies ...any,
) *Worker {
	if logger == nil {
		logger = logging.ComponentLogger(nil, "worker")
	}
	worker := &Worker{
		logger:        logger,
		avatarRepo:    avatarRepo,
		thumbnailRepo: thumbnailRepo,
		storage:       storage,
		processor:     processor,
		broker:        broker,
		tx:            tx,
	}
	for _, dependency := range dependencies {
		switch value := dependency.(type) {
		case contracts.Publisher:
			worker.publisher = value
		case contracts.OutboxRepository:
			worker.outbox = value
		case contracts.BlobRepository:
			worker.blobRepo = value
		}
	}
	return worker
}

// Start подписывает worker на события и запускает обработку.
func (w *Worker) Start(ctx context.Context) error {
	ctx = logging.WithLogger(ctx, w.logger)
	if err := w.broker.Subscribe(ctx, "avatar.uploaded", w.handleAvatarUploaded); err != nil {
		return fmt.Errorf("subscribe to avatar.uploaded: %w", err)
	}
	if err := w.broker.Subscribe(ctx, "avatar.deleted", w.handleAvatarDeleted); err != nil {
		return fmt.Errorf("subscribe to avatar.deleted: %w", err)
	}
	if w.outbox != nil && w.publisher != nil {
		go w.dispatchOutbox(ctx)
	}

	logging.Info(ctx, "worker started")
	<-ctx.Done()
	logging.Info(ctx, "worker stopping")
	return nil
}

func (w *Worker) handleAvatarUploaded(ctx context.Context, msg *contracts.Message) (err error) {
	ctx, span := startWorkerSpan(ctx, "avatar.process")
	defer func() { finishWorkerSpan(span, err) }()
	ctx = logging.WithLogger(ctx, w.logger)
	var event events.AvatarUploadedEvent
	if err := json.Unmarshal(msg.Payload, &event); err != nil {
		return fmt.Errorf("unmarshal upload event: %w", err)
	}
	avatarID, err := uuid.Parse(event.AvatarID)
	if err != nil {
		return fmt.Errorf("parse avatar id: %w", err)
	}

	logging.Info(ctx, "processing avatar upload", logging.String("avatar_id", event.AvatarID), logging.String("user_id", event.UserID))
	avatar, err := w.avatarRepo.GetByID(ctx, avatarID)
	if errors.Is(err, domain.ErrNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("get avatar: %w", err)
	}

	if claimer, ok := w.avatarRepo.(contracts.AvatarProcessorClaimer); ok {
		claimed, claimErr := claimer.ClaimForProcessing(ctx, avatarID)
		if errors.Is(claimErr, domain.ErrNotFound) {
			logging.Debug(ctx, "avatar was already claimed or processed", logging.String("avatar_id", event.AvatarID))
			return nil
		}
		if claimErr != nil {
			return fmt.Errorf("claim avatar for processing: %w", claimErr)
		}
		avatar = claimed
	} else {
		if !avatar.IsProcessable() {
			logging.Debug(ctx, "avatar is not processable", logging.String("avatar_id", event.AvatarID), logging.String("status", string(avatar.ProcessingStatus)))
			return nil
		}

		avatar.MarkProcessing()
		if err := w.avatarRepo.Update(ctx, avatar); err != nil {
			return fmt.Errorf("mark avatar processing: %w", err)
		}
	}

	original, err := w.storage.Download(ctx, avatar.S3KeyOriginal)
	if err != nil {
		return w.failAvatar(ctx, avatar, fmt.Errorf("download original: %w", err), isFinalDelivery(msg))
	}
	defer func() { _ = original.Close() }()

	img, format, err := w.processor.Decode(original)
	if err != nil {
		return w.failAvatar(ctx, avatar, fmt.Errorf("decode original: %w", err), isFinalDelivery(msg))
	}
	for _, size := range thumbnailSizes(event.Operations) {
		if err := w.createThumbnail(ctx, avatar, img, size, format); err != nil {
			return w.failAvatar(ctx, avatar, fmt.Errorf("create thumbnail %s: %w", size, err), isFinalDelivery(msg))
		}
	}

	avatar.MarkProcessed()
	if err := w.avatarRepo.Update(ctx, avatar); err != nil {
		return fmt.Errorf("mark avatar processed: %w", err)
	}
	logging.Info(ctx, "avatar processed successfully", logging.String("avatar_id", event.AvatarID))
	return nil
}

func (w *Worker) failAvatar(ctx context.Context, avatar *domain.Avatar, err error, finalAttempt bool) error {
	if finalAttempt {
		avatar.MarkFailed(err)
	} else {
		avatar.ProcessingStatus = domain.ProcessingStatusPending
		avatar.ProcessingStartedAt = nil
		avatar.LastError = err.Error()
		avatar.UpdatedAt = time.Now().UTC()
	}
	if updateErr := w.avatarRepo.Update(ctx, avatar); updateErr != nil {
		return errors.Join(err, fmt.Errorf("update avatar after processing failure: %w", updateErr))
	}
	return err
}

func isFinalDelivery(msg *contracts.Message) bool {
	if msg == nil || len(msg.Headers) == 0 {
		return true
	}
	attempt, attemptErr := strconv.Atoi(msg.Headers[contracts.DeliveryAttemptHeader])
	maxAttempts, maxErr := strconv.Atoi(msg.Headers[contracts.MaxDeliveryAttemptsHeader])
	if attemptErr != nil || maxErr != nil || attempt < 1 || maxAttempts < 1 {
		return true
	}
	return attempt >= maxAttempts
}

const thumbnailProcessorVersion = "v1"

func cropProcessorVersion(crop domain.AvatarCrop) string {
	return fmt.Sprintf("%s-crop-%.4f-%.4f-%.4f", thumbnailProcessorVersion, crop.X, crop.Y, crop.Size)
}

func applyAvatarCrop(img image.Image, crop domain.AvatarCrop) image.Image {
	if img == nil || crop.Size <= 0 {
		return img
	}
	bounds := img.Bounds()
	size := int(math.Round(crop.Size))
	x := int(math.Round(crop.X))
	y := int(math.Round(crop.Y))
	if size <= 0 || x < 0 || y < 0 || size > bounds.Dx() || size > bounds.Dy() {
		return img
	}
	if x+size > bounds.Dx() {
		x = bounds.Dx() - size
	}
	if y+size > bounds.Dy() {
		y = bounds.Dy() - size
	}
	return imaging.Crop(img, image.Rect(bounds.Min.X+x, bounds.Min.Y+y, bounds.Min.X+x+size, bounds.Min.Y+y+size))
}

func (w *Worker) createThumbnail(ctx context.Context, avatar *domain.Avatar, img image.Image, size domain.ThumbnailSize, sourceFormat string) (err error) {
	ctx, span := startWorkerSpan(ctx, "thumbnail.create")
	defer func() { finishWorkerSpan(span, err) }()
	width, height, ok := size.Dimensions()
	if !ok {
		return fmt.Errorf("unsupported thumbnail size: %s", size)
	}
	cropped := applyAvatarCrop(img, domain.AvatarCrop{X: avatar.CropX, Y: avatar.CropY, Size: avatar.CropSize})
	thumbnail := w.processor.CreateThumbnail(cropped, width, height)
	if thumbnail == nil {
		return fmt.Errorf("processor returned nil thumbnail")
	}

	format := strings.ToLower(sourceFormat)
	if format != "jpeg" && format != "png" {
		format = "jpeg"
	}
	var encoded bytes.Buffer
	if err := w.processor.Encode(&encoded, thumbnail, format, 85); err != nil {
		return fmt.Errorf("encode thumbnail: %w", err)
	}

	s3Key := fmt.Sprintf("avatars/thumbnails/%s/%s/%s.%s", avatar.UserID, avatar.ID.String(), size, format)
	var blobID, derivationID uuid.UUID
	if w.blobRepo != nil && avatar.SourceBlobID != uuid.Nil {
		digest := sha256.Sum256(encoded.Bytes())
		blob, err := w.blobRepo.GetOrCreate(ctx, &domain.Blob{
			ID:            uuid.New(),
			SHA256:        digest[:],
			SizeBytes:     int64(encoded.Len()),
			MimeType:      mimeTypeForFormat(format),
			Width:         width,
			Height:        height,
			ObjectKey:     "blobs/" + hex.EncodeToString(digest[:]),
			StorageStatus: domain.BlobStorageStatusUploading,
		})
		if err != nil {
			return fmt.Errorf("get or create thumbnail blob: %w", err)
		}
		if !blob.IsReady() {
			if err := w.storage.Upload(ctx, blob.ObjectKey, bytes.NewReader(encoded.Bytes()), int64(encoded.Len()), mimeTypeForFormat(format)); err != nil {
				if markErr := w.blobRepo.MarkFailed(ctx, blob.ID, err.Error()); markErr != nil {
					logging.Warn(ctx, "failed to mark thumbnail blob as failed", logging.Err(markErr), logging.UUID("blob_id", blob.ID))
				}
				return fmt.Errorf("upload thumbnail blob: %w", err)
			}
			if err := w.blobRepo.MarkReady(ctx, blob.ID); err != nil {
				return fmt.Errorf("mark thumbnail blob ready: %w", err)
			}
		}
		derivation, err := w.blobRepo.EnsureDerivation(ctx, &domain.BlobDerivation{
			ID:               uuid.New(),
			ParentBlobID:     avatar.SourceBlobID,
			DerivedBlobID:    blob.ID,
			Variant:          size,
			ProcessorVersion: cropProcessorVersion(domain.AvatarCrop{X: avatar.CropX, Y: avatar.CropY, Size: avatar.CropSize}),
			OutputFormat:     format,
			Status:           domain.BlobStorageStatusReady,
		})
		if err != nil {
			return fmt.Errorf("ensure thumbnail derivation: %w", err)
		}
		s3Key = blob.ObjectKey
		blobID = blob.ID
		derivationID = derivation.ID
	}

	if blobID == uuid.Nil {
		if err := w.storage.Upload(ctx, s3Key, bytes.NewReader(encoded.Bytes()), int64(encoded.Len()), mimeTypeForFormat(format)); err != nil {
			return fmt.Errorf("upload thumbnail: %w", err)
		}
	}

	thumbnailRecord := &domain.Thumbnail{
		ID:           uuid.New(),
		AvatarID:     avatar.ID,
		Size:         size,
		BlobID:       blobID,
		DerivationID: derivationID,
		S3Key:        s3Key,
		Width:        width,
		Height:       height,
		SizeBytes:    int64(encoded.Len()),
		CreatedAt:    time.Now().UTC(),
	}
	if err := w.thumbnailRepo.Create(ctx, thumbnailRecord); err != nil && !errors.Is(err, domain.ErrAlreadyExists) {
		return fmt.Errorf("save thumbnail record: %w", err)
	}
	if blobID != uuid.Nil {
		if err := w.blobRepo.LinkThumbnail(ctx, thumbnailRecord.ID, blobID, derivationID); err != nil && !errors.Is(err, domain.ErrNotFound) {
			return fmt.Errorf("link thumbnail blob: %w", err)
		}
	}
	return nil
}

func thumbnailSizes(operations []events.ProcessingOperation) []domain.ThumbnailSize {
	if len(operations) == 0 {
		return domain.SupportedThumbnailSizes
	}
	result := make([]domain.ThumbnailSize, 0, len(operations))
	for _, operation := range operations {
		switch operation {
		case events.ProcessingOperationThumbnail100:
			result = append(result, domain.ThumbnailSize100x100)
		case events.ProcessingOperationThumbnail300:
			result = append(result, domain.ThumbnailSize300x300)
		}
	}
	if len(result) == 0 {
		return domain.SupportedThumbnailSizes
	}
	return result
}

func (w *Worker) dispatchOutbox(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	w.flushOutbox(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.flushOutbox(ctx)
		}
	}
}

func (w *Worker) flushOutbox(ctx context.Context) {
	var outboxEvents []*contracts.OutboxEvent
	err := w.tx.Do(ctx, func(txctx context.Context) error {
		var err error
		outboxEvents, err = w.outbox.ClaimPending(txctx, 100, 2*time.Minute)
		return err
	})
	if err != nil {
		logging.Warn(ctx, "failed to claim pending outbox events", logging.Err(err))
		return
	}
	for _, event := range outboxEvents {
		eventCtx := observability.ExtractMessageContext(ctx, event.Headers)
		message := contracts.NewMessageWithID(event.ID, event.Topic, event.Payload)
		message.Headers = observability.CloneHeaders(event.Headers)
		if err := w.publisher.Publish(eventCtx, event.Topic, message); err != nil {
			nextAttempt := time.Now().UTC().Add(outboxRetryDelay(event.Attempts + 1))
			if markErr := w.outbox.MarkFailed(ctx, event.ID, err.Error(), nextAttempt); markErr != nil {
				logging.Warn(ctx, "failed to update outbox retry state", logging.Err(markErr), logging.String("event_id", event.ID))
			}
			continue
		}
		if err := w.outbox.MarkPublished(ctx, event.ID); err != nil {
			logging.Warn(ctx, "failed to mark outbox event published", logging.Err(err), logging.String("event_id", event.ID))
		}
	}
}

func outboxRetryDelay(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	if attempt > 9 {
		attempt = 9
	}
	return time.Duration(1<<(attempt-1)) * time.Second
}

func (w *Worker) handleAvatarDeleted(ctx context.Context, msg *contracts.Message) (err error) {
	ctx, span := startWorkerSpan(ctx, "avatar.delete")
	defer func() { finishWorkerSpan(span, err) }()
	var event events.AvatarDeletedEvent
	if err := json.Unmarshal(msg.Payload, &event); err != nil {
		return fmt.Errorf("unmarshal deletion event: %w", err)
	}

	if w.blobRepo != nil {
		logging.Info(ctx, "avatar deletion acknowledged; shared blob cleanup is deferred", logging.String("avatar_id", event.AvatarID))
		return nil
	}

	var deleteErrs []error
	for _, key := range event.S3Keys {
		if err := w.storage.Delete(ctx, key); err != nil {
			deleteErrs = append(deleteErrs, fmt.Errorf("delete %q: %w", key, err))
		}
	}
	if err := errors.Join(deleteErrs...); err != nil {
		logging.Error(ctx, "failed to delete avatar objects", logging.Err(err), logging.String("avatar_id", event.AvatarID))
		return err
	}
	logging.Info(ctx, "avatar objects deleted", logging.String("avatar_id", event.AvatarID))
	return nil
}

func mimeTypeForFormat(format string) string {
	if format == "jpeg" {
		return "image/jpeg"
	}
	return "image/" + format
}
