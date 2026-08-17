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
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AvatarRepository сохраняет avatar в PostgreSQL.
type AvatarRepository struct {
	queries gen.Querier
	db      *pgxpool.Pool
}

// NewAvatarRepository создаёт репозиторий avatar.
func NewAvatarRepository(queries gen.Querier, pools ...*pgxpool.Pool) *AvatarRepository {
	var db *pgxpool.Pool
	if len(pools) > 0 {
		db = pools[0]
	}
	return &AvatarRepository{queries: queries, db: db}
}

// Create сохраняет avatar.
func (r *AvatarRepository) Create(ctx context.Context, avatar *domain.Avatar) error {
	if avatar == nil {
		return fmt.Errorf("create avatar: %w", domain.ErrInvalidInput)
	}
	if avatar.ID == uuid.Nil {
		avatar.ID = uuid.New()
	}
	if avatar.UploadStatus == "" {
		avatar.UploadStatus = domain.UploadStatusUploading
	}
	if avatar.ProcessingStatus == "" {
		avatar.ProcessingStatus = domain.ProcessingStatusPending
	}
	now := time.Now().UTC()
	if avatar.CreatedAt.IsZero() {
		avatar.CreatedAt = now
	}
	if avatar.UpdatedAt.IsZero() {
		avatar.UpdatedAt = avatar.CreatedAt
	}

	created, err := r.queries.CreateAvatar(ctx, gen.CreateAvatarParams{
		ID:                  toPGUUID(avatar.ID),
		UserID:              avatar.UserID,
		FileName:            avatar.FileName,
		MimeType:            avatar.MimeType,
		SizeBytes:           avatar.SizeBytes,
		Width:               int32(avatar.Width),
		Height:              int32(avatar.Height),
		S3KeyOriginal:       avatar.S3KeyOriginal,
		SourceBlobID:        toPGUUID(avatar.SourceBlobID),
		UploadStatus:        string(avatar.UploadStatus),
		ProcessingStatus:    string(avatar.ProcessingStatus),
		ProcessingStartedAt: toPGTime(avatar.ProcessingStartedAt),
		ProcessingAttempts:  int32(avatar.ProcessingAttempts),
		LastError:           avatar.LastError,
		CreatedAt:           toPGTime(&avatar.CreatedAt),
		UpdatedAt:           toPGTime(&avatar.UpdatedAt),
		IsActive:            avatar.IsActive,
		CropX:               avatar.CropX,
		CropY:               avatar.CropY,
		CropSize:            avatar.CropSize,
		DeletedAt:           toPGTime(avatar.DeletedAt),
	})
	if err != nil {
		return mapDatabaseError("create avatar", err)
	}
	*avatar = fromGeneratedAvatar(created)
	return nil
}

// GetByID возвращает avatar по идентификатору.
func (r *AvatarRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Avatar, error) {
	avatar, err := r.queries.GetAvatarByID(ctx, toPGUUID(id))
	if err != nil {
		return nil, mapDatabaseError("get avatar", err)
	}
	result := fromGeneratedAvatar(avatar)
	return &result, nil
}

// GetByUserID возвращает avatar пользователя.
func (r *AvatarRepository) GetByUserID(ctx context.Context, userID string) (*domain.Avatar, error) {
	avatar, err := r.queries.GetAvatarByUserID(ctx, userID)
	if err != nil {
		return nil, mapDatabaseError("get avatar by user", err)
	}
	result := fromGeneratedAvatar(avatar)
	return &result, nil
}

// FindByUserAndSourceBlobID ищет avatar по исходному blob-у.
func (r *AvatarRepository) FindByUserAndSourceBlobID(ctx context.Context, userID string, sourceBlobID uuid.UUID) (*domain.Avatar, error) {
	if r == nil || r.queries == nil {
		return nil, fmt.Errorf("find avatar by source blob: repository is not configured")
	}
	if userID == "" || sourceBlobID == uuid.Nil {
		return nil, fmt.Errorf("find avatar by source blob: %w", domain.ErrInvalidInput)
	}

	avatar, err := r.queries.FindAvatarByUserAndSourceBlobID(ctx, gen.FindAvatarByUserAndSourceBlobIDParams{
		UserID:       userID,
		SourceBlobID: toPGUUID(sourceBlobID),
	})
	if err != nil {
		return nil, mapDatabaseError("find avatar by source blob", err)
	}
	result := fromGeneratedAvatar(avatar)
	return &result, nil
}

// Activate делает avatar активным.
func (r *AvatarRepository) Activate(ctx context.Context, userID string, avatarID uuid.UUID) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("activate avatar: database pool is not configured")
	}
	if userID == "" || avatarID == uuid.Nil {
		return fmt.Errorf("activate avatar: %w", domain.ErrInvalidInput)
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("activate avatar: begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	queries := gen.New(tx)
	if err := queries.DeactivateCompetingAvatars(ctx, gen.DeactivateCompetingAvatarsParams{
		UserID: userID,
		ID:     toPGUUID(avatarID),
	}); err != nil {
		return mapDatabaseError("deactivate competing avatars", err)
	}
	result, err := queries.ActivateAvatar(ctx, gen.ActivateAvatarParams{
		ID:     toPGUUID(avatarID),
		UserID: userID,
	})
	if err != nil {
		return mapDatabaseError("activate avatar", err)
	}
	if result.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	if err := queries.RestoreAvatarThumbnails(ctx, toPGUUID(avatarID)); err != nil {
		return mapDatabaseError("restore avatar thumbnails", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("activate avatar: commit transaction: %w", err)
	}
	return nil
}

// ClaimForProcessing забирает avatar на обработку.
func (r *AvatarRepository) ClaimForProcessing(ctx context.Context, id uuid.UUID) (*domain.Avatar, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("claim avatar for processing: database pool is not configured")
	}
	avatar, err := gen.New(r.db).ClaimAvatarForProcessing(ctx, toPGUUID(id))
	if err != nil {
		return nil, mapDatabaseError("claim avatar for processing", err)
	}
	result := fromGeneratedAvatar(avatar)
	return &result, nil
}

// ListByUserID возвращает avatar пользователя постранично.
func (r *AvatarRepository) ListByUserID(ctx context.Context, userID string, limit, offset int) ([]*domain.Avatar, error) {
	limit, offset = normalizePagination(limit, offset)
	avatars, err := r.queries.ListAvatarsByUserID(ctx, gen.ListAvatarsByUserIDParams{
		UserID: userID,
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		return nil, mapDatabaseError("list avatars", err)
	}
	result := make([]*domain.Avatar, 0, len(avatars))
	for _, avatar := range avatars {
		converted := fromGeneratedAvatar(avatar)
		result = append(result, &converted)
	}
	return result, nil
}

// Update обновляет avatar.
func (r *AvatarRepository) Update(ctx context.Context, avatar *domain.Avatar) error {
	if avatar == nil {
		return fmt.Errorf("update avatar: %w", domain.ErrInvalidInput)
	}
	if avatar.UpdatedAt.IsZero() {
		avatar.UpdatedAt = time.Now().UTC()
	}
	updated, err := r.queries.UpdateAvatar(ctx, gen.UpdateAvatarParams{
		ID:                  toPGUUID(avatar.ID),
		UserID:              avatar.UserID,
		FileName:            avatar.FileName,
		MimeType:            avatar.MimeType,
		SizeBytes:           avatar.SizeBytes,
		Width:               int32(avatar.Width),
		Height:              int32(avatar.Height),
		S3KeyOriginal:       avatar.S3KeyOriginal,
		SourceBlobID:        toPGUUID(avatar.SourceBlobID),
		UploadStatus:        string(avatar.UploadStatus),
		ProcessingStatus:    string(avatar.ProcessingStatus),
		ProcessingStartedAt: toPGTime(avatar.ProcessingStartedAt),
		ProcessingAttempts:  int32(avatar.ProcessingAttempts),
		LastError:           avatar.LastError,
		UpdatedAt:           toPGTime(&avatar.UpdatedAt),
		IsActive:            avatar.IsActive,
		CropX:               avatar.CropX,
		CropY:               avatar.CropY,
		CropSize:            avatar.CropSize,
	})
	if err != nil {
		return mapDatabaseError("update avatar", err)
	}
	*avatar = fromGeneratedAvatar(updated)
	return nil
}

// Delete помечает avatar удалённым.
func (r *AvatarRepository) Delete(ctx context.Context, id uuid.UUID) error {
	now := time.Now().UTC()
	result, err := r.queries.SoftDeleteAvatar(ctx, gen.SoftDeleteAvatarParams{ID: toPGUUID(id), DeletedAt: toPGTime(&now)})
	if err != nil {
		return mapDatabaseError("delete avatar", err)
	}
	if result.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// HardDeleteExpired удаляет avatar старше указанного срока.
func (r *AvatarRepository) HardDeleteExpired(ctx context.Context, deletedBefore time.Time) (int64, error) {
	if r == nil || r.queries == nil {
		return 0, fmt.Errorf("hard delete expired avatars: repository is not configured")
	}
	result, err := r.queries.HardDeleteExpiredAvatars(ctx, toPGTime(&deletedBefore))
	if err != nil {
		return 0, mapDatabaseError("hard delete expired avatars", err)
	}
	return result.RowsAffected(), nil
}

// GetPendingForProcessing возвращает avatar, ожидающие обработки.
func (r *AvatarRepository) GetPendingForProcessing(ctx context.Context, limit int) ([]*domain.Avatar, error) {
	if limit <= 0 {
		limit = 100
	}
	avatars, err := r.queries.GetPendingAvatars(ctx, gen.GetPendingAvatarsParams{
		UploadStatus:     string(domain.UploadStatusCompleted),
		ProcessingStatus: string(domain.ProcessingStatusPending),
		Limit:            int32(limit),
	})
	if err != nil {
		return nil, mapDatabaseError("get pending avatars", err)
	}
	result := make([]*domain.Avatar, 0, len(avatars))
	for _, avatar := range avatars {
		converted := fromGeneratedAvatar(avatar)
		result = append(result, &converted)
	}
	return result, nil
}

// UpdateProcessingStatus меняет статус обработки avatar.
func (r *AvatarRepository) UpdateProcessingStatus(ctx context.Context, id uuid.UUID, status domain.ProcessingStatus) error {
	now := time.Now().UTC()
	result, err := r.queries.UpdateProcessingStatus(ctx, gen.UpdateProcessingStatusParams{
		ID:               toPGUUID(id),
		ProcessingStatus: string(status),
		UpdatedAt:        toPGTime(&now),
	})
	if err != nil {
		return mapDatabaseError("update processing status", err)
	}
	if result.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func fromGeneratedAvatar(avatar gen.Avatar) domain.Avatar {
	return domain.Avatar{
		ID:                  fromPGUUID(avatar.ID),
		UserID:              avatar.UserID,
		FileName:            avatar.FileName,
		MimeType:            avatar.MimeType,
		SizeBytes:           avatar.SizeBytes,
		Width:               int(avatar.Width),
		Height:              int(avatar.Height),
		SourceBlobID:        fromPGUUID(avatar.SourceBlobID),
		S3KeyOriginal:       avatar.S3KeyOriginal,
		UploadStatus:        domain.UploadStatus(avatar.UploadStatus),
		ProcessingStatus:    domain.ProcessingStatus(avatar.ProcessingStatus),
		ProcessingStartedAt: fromPGTime(avatar.ProcessingStartedAt),
		ProcessingAttempts:  int(avatar.ProcessingAttempts),
		LastError:           avatar.LastError,
		CreatedAt:           avatar.CreatedAt.Time,
		UpdatedAt:           avatar.UpdatedAt.Time,
		DeletedAt:           fromPGTime(avatar.DeletedAt),
		IsActive:            avatar.IsActive,
		CropX:               avatar.CropX,
		CropY:               avatar.CropY,
		CropSize:            avatar.CropSize,
	}
}

func toPGUUID(value uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: value, Valid: value != uuid.Nil}
}

func fromPGUUID(value pgtype.UUID) uuid.UUID {
	if !value.Valid {
		return uuid.Nil
	}
	return value.Bytes
}

func toPGTime(value *time.Time) pgtype.Timestamptz {
	if value == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: value.UTC(), Valid: true}
}

func fromPGTime(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	result := value.Time
	return &result
}

func normalizePagination(limit, offset int) (int, int) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

func mapDatabaseError(operation string, err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return fmt.Errorf("%s: %w", operation, domain.ErrAlreadyExists)
	}
	return fmt.Errorf("%s: %w", operation, err)
}
