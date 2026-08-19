package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aikowocki/yandex-go-ext/internal/domain"
	"github.com/aikowocki/yandex-go-ext/internal/infra/postgres/gen"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// BlobRepository управляет blob-ами в PostgreSQL.
type BlobRepository struct {
	baseRepo
}

// NewBlobRepository создаёт репозиторий blob-ов.
func NewBlobRepository(db *DB) *BlobRepository {
	return &BlobRepository{baseRepo: baseRepo{db: db}}
}

// GetOrCreate возвращает существующий или создаёт новый blob.
func (r *BlobRepository) GetOrCreate(ctx context.Context, blob *domain.Blob) (*domain.Blob, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("get or create blob: database pool is not configured")
	}
	if blob == nil || len(blob.SHA256) != 32 || blob.SizeBytes < 0 || blob.ObjectKey == "" {
		return nil, fmt.Errorf("get or create blob: %w", domain.ErrInvalidInput)
	}
	if blob.ID == uuid.Nil {
		blob.ID = uuid.New()
	}
	if blob.StorageStatus == "" {
		blob.StorageStatus = domain.BlobStorageStatusUploading
	}
	now := time.Now().UTC()
	if blob.CreatedAt.IsZero() {
		blob.CreatedAt = now
	}
	if blob.UpdatedAt.IsZero() {
		blob.UpdatedAt = blob.CreatedAt
	}

	result, err := r.q(ctx).GetOrCreateBlob(ctx, gen.GetOrCreateBlobParams{
		ID:            toPGUUID(blob.ID),
		Sha256:        blob.SHA256,
		SizeBytes:     blob.SizeBytes,
		MimeType:      blob.MimeType,
		Width:         int32(blob.Width),
		Height:        int32(blob.Height),
		ObjectKey:     blob.ObjectKey,
		StorageStatus: gen.BlobStorageStatus(blob.StorageStatus),
		LastError:     blob.LastError,
		CreatedAt:     toPGTime(&blob.CreatedAt),
		UpdatedAt:     toPGTime(&blob.UpdatedAt),
	})
	if err != nil {
		return nil, mapDatabaseError("get or create blob", err)
	}
	converted := fromGeneratedBlob(result)
	return &converted, nil
}

// MarkReady помечает blob готовым.
func (r *BlobRepository) MarkReady(ctx context.Context, id uuid.UUID) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("mark blob ready: database pool is not configured")
	}
	result, err := r.q(ctx).MarkBlobReady(ctx, toPGUUID(id))
	if err != nil {
		return mapDatabaseError("mark blob ready", err)
	}
	if result.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// MarkFailed сохраняет ошибку обработки blob-а.
func (r *BlobRepository) MarkFailed(ctx context.Context, id uuid.UUID, message string) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("mark blob failed: database pool is not configured")
	}
	result, err := r.q(ctx).MarkBlobFailed(ctx, gen.MarkBlobFailedParams{ID: toPGUUID(id), LastError: message})
	if err != nil {
		return mapDatabaseError("mark blob failed", err)
	}
	if result.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// EnsureDerivation создаёт или возвращает производный blob.
func (r *BlobRepository) EnsureDerivation(ctx context.Context, derivation *domain.BlobDerivation) (*domain.BlobDerivation, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("ensure blob derivation: database pool is not configured")
	}
	if derivation == nil || derivation.ParentBlobID == uuid.Nil || derivation.DerivedBlobID == uuid.Nil || !derivation.Variant.IsSupported() {
		return nil, fmt.Errorf("ensure blob derivation: %w", domain.ErrInvalidInput)
	}
	if derivation.ID == uuid.Nil {
		derivation.ID = uuid.New()
	}
	if derivation.ProcessorVersion == "" {
		derivation.ProcessorVersion = "v1"
	}
	if derivation.OutputFormat == "" {
		derivation.OutputFormat = "jpeg"
	}
	if derivation.Status == "" {
		derivation.Status = domain.BlobStorageStatusReady
	}
	if derivation.CreatedAt.IsZero() {
		derivation.CreatedAt = time.Now().UTC()
	}

	result, err := r.q(ctx).EnsureBlobDerivation(ctx, gen.EnsureBlobDerivationParams{
		ID:               toPGUUID(derivation.ID),
		ParentBlobID:     toPGUUID(derivation.ParentBlobID),
		DerivedBlobID:    toPGUUID(derivation.DerivedBlobID),
		Variant:          string(derivation.Variant),
		ProcessorVersion: derivation.ProcessorVersion,
		OutputFormat:     derivation.OutputFormat,
		Status:           gen.BlobDerivationStatus(derivation.Status),
		CreatedAt:        toPGTime(&derivation.CreatedAt),
	})
	if err != nil {
		return nil, mapDatabaseError("ensure blob derivation", err)
	}
	converted := fromGeneratedBlobDerivation(result)
	return &converted, nil
}

// LinkThumbnail связывает миниатюру с blob-ом.
func (r *BlobRepository) LinkThumbnail(ctx context.Context, thumbnailID, blobID, derivationID uuid.UUID) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("link thumbnail blob: database pool is not configured")
	}
	result, err := r.q(ctx).LinkThumbnailBlob(ctx, gen.LinkThumbnailBlobParams{
		ID:           toPGUUID(thumbnailID),
		BlobID:       toPGUUID(blobID),
		DerivationID: toPGUUID(derivationID),
	})
	if err != nil {
		return mapDatabaseError("link thumbnail blob", err)
	}
	if result.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// ListUnreferenced возвращает неиспользуемые blob-ы.
func (r *BlobRepository) ListUnreferenced(ctx context.Context, createdBefore time.Time, limit int) ([]*domain.Blob, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("list unreferenced blobs: database pool is not configured")
	}
	if limit <= 0 {
		limit = 100
	}
	rows, err := r.q(ctx).ListUnreferencedBlobs(ctx, gen.ListUnreferencedBlobsParams{
		CreatedAt: toPGTime(&createdBefore),
		Limit:     int32(limit),
	})
	if err != nil {
		return nil, mapDatabaseError("list unreferenced blobs", err)
	}
	result := make([]*domain.Blob, 0, len(rows))
	for _, row := range rows {
		converted := fromGeneratedBlob(row)
		result = append(result, &converted)
	}
	return result, nil
}

// Delete удаляет blob и его производные записи.
func (r *BlobRepository) Delete(ctx context.Context, id uuid.UUID) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("delete blob: database pool is not configured")
	}
	queries := r.q(ctx)
	if err := queries.DeleteBlobDerivations(ctx, toPGUUID(id)); err != nil {
		return mapDatabaseError("delete blob derivations", err)
	}
	if err := queries.DeleteBlobRow(ctx, toPGUUID(id)); err != nil {
		return mapDatabaseError("delete blob row", err)
	}
	return nil
}

// HasObject проверяет наличие объекта blob-а.
func (r *BlobRepository) HasObject(ctx context.Context, objectKey string) (bool, error) {
	if r == nil || r.db == nil {
		return false, fmt.Errorf("check blob object: database pool is not configured")
	}
	exists, err := r.q(ctx).HasBlobObject(ctx, objectKey)
	if err != nil {
		return false, mapDatabaseError("check blob object", err)
	}
	return exists, nil
}

// ClaimForDeletion забирает blob на удаление.
func (r *BlobRepository) ClaimForDeletion(ctx context.Context, id uuid.UUID) (*domain.Blob, bool, error) {
	if r == nil || r.db == nil {
		return nil, false, fmt.Errorf("claim blob for deletion: database pool is not configured")
	}
	row, err := r.q(ctx).ClaimBlobForDeletion(ctx, toPGUUID(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, mapDatabaseError("claim blob for deletion", err)
	}
	converted := fromGeneratedBlob(row)
	return &converted, true, nil
}

func fromGeneratedBlob(blob gen.Blob) domain.Blob {
	return domain.Blob{
		ID:            fromPGUUID(blob.ID),
		SHA256:        append([]byte(nil), blob.Sha256...),
		SizeBytes:     blob.SizeBytes,
		MimeType:      blob.MimeType,
		Width:         int(blob.Width),
		Height:        int(blob.Height),
		ObjectKey:     blob.ObjectKey,
		StorageStatus: domain.BlobStorageStatus(blob.StorageStatus),
		LastError:     blob.LastError,
		CreatedAt:     blob.CreatedAt.Time,
		UpdatedAt:     blob.UpdatedAt.Time,
	}
}

func fromGeneratedBlobDerivation(derivation gen.BlobDerivation) domain.BlobDerivation {
	return domain.BlobDerivation{
		ID:               fromPGUUID(derivation.ID),
		ParentBlobID:     fromPGUUID(derivation.ParentBlobID),
		DerivedBlobID:    fromPGUUID(derivation.DerivedBlobID),
		Variant:          domain.ThumbnailSize(derivation.Variant),
		ProcessorVersion: derivation.ProcessorVersion,
		OutputFormat:     derivation.OutputFormat,
		Status:           domain.BlobStorageStatus(derivation.Status),
		CreatedAt:        derivation.CreatedAt.Time,
	}
}
