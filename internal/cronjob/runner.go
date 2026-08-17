package cronjob

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aikowocki/yandex-go-ext/internal/config"
	"github.com/aikowocki/yandex-go-ext/internal/contracts"
	"github.com/aikowocki/yandex-go-ext/internal/shared/logging"
)

const (
	defaultRetentionPeriod   = 30 * 24 * time.Hour
	defaultOrphanGracePeriod = 24 * time.Hour
	defaultCleanupBatchSize  = 100
)

// Runner выполняет очистку avatar, thumbnails и blob-ов.
type Runner struct {
	avatarRetention    contracts.AvatarRetentionRepository
	thumbnailRetention contracts.ThumbnailRetentionRepository
	blobRetention      contracts.BlobRetentionRepository
	storage            contracts.ObjectStorage
	storageLister      contracts.ObjectStorageLister
	retentionPeriod    time.Duration
	orphanGracePeriod  time.Duration
	cleanupBatchSize   int
}

// New создаёт runner для плановых задач.
func New(
	avatarRetention contracts.AvatarRetentionRepository,
	thumbnailRetention contracts.ThumbnailRetentionRepository,
	blobRetention contracts.BlobRetentionRepository,
	storage contracts.ObjectStorage,
	storageLister contracts.ObjectStorageLister,
	cfg config.WorkerConfig,
) *Runner {
	retentionPeriod := cfg.RetentionPeriod
	if retentionPeriod <= 0 {
		retentionPeriod = defaultRetentionPeriod
	}
	orphanGracePeriod := cfg.OrphanGracePeriod
	if orphanGracePeriod <= 0 {
		orphanGracePeriod = defaultOrphanGracePeriod
	}
	batchSize := cfg.CleanupBatchSize
	if batchSize <= 0 {
		batchSize = defaultCleanupBatchSize
	}
	return &Runner{
		avatarRetention:    avatarRetention,
		thumbnailRetention: thumbnailRetention,
		blobRetention:      blobRetention,
		storage:            storage,
		storageLister:      storageLister,
		retentionPeriod:    retentionPeriod,
		orphanGracePeriod:  orphanGracePeriod,
		cleanupBatchSize:   batchSize,
	}
}

// RunRetention удаляет устаревшие записи и blob-ы.
func (r *Runner) RunRetention(ctx context.Context) error {
	if r == nil {
		return fmt.Errorf("run retention: runner is nil")
	}
	deletedBefore := time.Now().UTC().Add(-r.retentionPeriod)
	var runErrs []error

	if r.avatarRetention != nil {
		count, err := r.avatarRetention.HardDeleteExpired(ctx, deletedBefore)
		if err != nil {
			runErrs = append(runErrs, fmt.Errorf("hard-delete avatars: %w", err))
		} else {
			logging.Info(ctx, "cronjob hard-deleted expired avatars", logging.Int64("count", count))
		}
	}
	if r.thumbnailRetention != nil {
		count, err := r.thumbnailRetention.HardDeleteExpired(ctx, deletedBefore)
		if err != nil {
			runErrs = append(runErrs, fmt.Errorf("hard-delete thumbnails: %w", err))
		} else {
			logging.Info(ctx, "cronjob hard-deleted expired thumbnails", logging.Int64("count", count))
		}
	}
	if r.blobRetention == nil || r.storage == nil {
		return errors.Join(runErrs...)
	}

	candidates, err := r.blobRetention.ListUnreferenced(
		ctx,
		time.Now().UTC().Add(-r.orphanGracePeriod),
		r.cleanupBatchSize,
	)
	if err != nil {
		runErrs = append(runErrs, fmt.Errorf("list unreferenced blobs: %w", err))
		return errors.Join(runErrs...)
	}
	for _, candidate := range candidates {
		if candidate == nil {
			continue
		}
		blob, claimed, claimErr := r.blobRetention.ClaimForDeletion(ctx, candidate.ID)
		if claimErr != nil {
			runErrs = append(runErrs, fmt.Errorf("claim blob %s: %w", candidate.ID, claimErr))
			continue
		}
		if !claimed || blob == nil || blob.ObjectKey == "" {
			continue
		}
		if err := r.storage.Delete(ctx, blob.ObjectKey); err != nil {
			runErrs = append(runErrs, fmt.Errorf("delete MinIO object %q: %w", blob.ObjectKey, err))
			continue
		}
		if err := r.blobRetention.Delete(ctx, blob.ID); err != nil {
			runErrs = append(runErrs, fmt.Errorf("delete blob row %s: %w", blob.ID, err))
			continue
		}
		logging.Info(ctx, "cronjob deleted orphan blob", logging.UUID("blob_id", blob.ID), logging.String("object_key", blob.ObjectKey))
	}
	return errors.Join(runErrs...)
}

// RunReconcile удаляет объекты хранилища без ссылок в базе.
func (r *Runner) RunReconcile(ctx context.Context) error {
	if r == nil || r.storageLister == nil || r.blobRetention == nil || r.storage == nil {
		return fmt.Errorf("run reconcile: required storage capabilities are not configured")
	}
	objects, err := r.storageLister.List(ctx, "blobs/", time.Now().UTC().Add(-r.orphanGracePeriod))
	if err != nil {
		return fmt.Errorf("list MinIO objects: %w", err)
	}
	var runErrs []error
	for _, object := range objects {
		exists, err := r.blobRetention.HasObject(ctx, object.Key)
		if err != nil {
			runErrs = append(runErrs, fmt.Errorf("check object %q: %w", object.Key, err))
			continue
		}
		if exists {
			continue
		}
		if err := r.storage.Delete(ctx, object.Key); err != nil {
			runErrs = append(runErrs, fmt.Errorf("delete storage orphan %q: %w", object.Key, err))
			continue
		}
		logging.Info(ctx, "cronjob deleted storage orphan", logging.String("object_key", object.Key))
	}
	return errors.Join(runErrs...)
}
