-- name: CreateAvatar :one
INSERT INTO avatars (
    id, user_id, file_name, mime_type, size_bytes, width, height,
    s3_key_original, upload_status, processing_status,
    processing_started_at, processing_attempts, last_error,
    created_at, updated_at, deleted_at, source_blob_id, is_active, crop_x, crop_y, crop_size
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21)
RETURNING id, user_id, file_name, mime_type, size_bytes, width, height,
          s3_key_original, upload_status, processing_status,
          processing_started_at, processing_attempts, last_error,
          created_at, updated_at, deleted_at, source_blob_id, is_active, crop_x, crop_y, crop_size;

-- name: GetAvatarByID :one
SELECT id, user_id, file_name, mime_type, size_bytes, width, height,
       s3_key_original, upload_status, processing_status, processing_started_at,
       processing_attempts, last_error, created_at, updated_at, deleted_at, source_blob_id, is_active, crop_x, crop_y, crop_size
FROM avatars
WHERE id = $1 AND deleted_at IS NULL;

-- name: GetAvatarByUserID :one
SELECT id, user_id, file_name, mime_type, size_bytes, width, height,
       s3_key_original, upload_status, processing_status, processing_started_at,
       processing_attempts, last_error, created_at, updated_at, deleted_at, source_blob_id, is_active, crop_x, crop_y, crop_size
FROM avatars
WHERE user_id = $1 AND deleted_at IS NULL AND is_active
ORDER BY is_active DESC, created_at DESC
LIMIT 1;

-- name: ListAvatarsByUserID :many
SELECT id, user_id, file_name, mime_type, size_bytes, width, height,
       s3_key_original, upload_status, processing_status, processing_started_at,
       processing_attempts, last_error, created_at, updated_at, deleted_at, source_blob_id, is_active, crop_x, crop_y, crop_size
FROM avatars
WHERE user_id = $1 AND deleted_at IS NULL
ORDER BY is_active DESC, created_at DESC
LIMIT $2 OFFSET $3;

-- name: UpdateAvatar :one
UPDATE avatars SET
    user_id = $2,
    file_name = $3,
    mime_type = $4,
    size_bytes = $5,
    width = $6,
    height = $7,
    s3_key_original = $8,
    upload_status = $9,
    processing_status = $10,
    processing_started_at = $11,
    processing_attempts = $12,
    last_error = $13,
    updated_at = $14,
    source_blob_id = $15,
    is_active = $16,
    crop_x = $17,
    crop_y = $18,
    crop_size = $19
WHERE id = $1 AND deleted_at IS NULL
RETURNING id, user_id, file_name, mime_type, size_bytes, width, height,
          s3_key_original, upload_status, processing_status,
          processing_started_at, processing_attempts, last_error,
          created_at, updated_at, deleted_at, source_blob_id, is_active, crop_x, crop_y, crop_size;

-- name: SoftDeleteAvatar :execresult
UPDATE avatars
SET deleted_at = $2, updated_at = $2, is_active = FALSE
WHERE id = $1 AND deleted_at IS NULL;

-- name: GetPendingAvatars :many
SELECT id, user_id, file_name, mime_type, size_bytes, width, height,
       s3_key_original, upload_status, processing_status, processing_started_at,
       processing_attempts, last_error, created_at, updated_at, deleted_at, source_blob_id, is_active, crop_x, crop_y, crop_size
FROM avatars
WHERE upload_status = $1 AND processing_status = $2 AND deleted_at IS NULL
ORDER BY created_at
LIMIT $3;

-- name: UpdateProcessingStatus :execresult
UPDATE avatars
SET processing_status = $2, updated_at = $3
WHERE id = $1 AND deleted_at IS NULL;


-- name: FindAvatarByUserAndSourceBlobID :one
SELECT id, user_id, file_name, mime_type, size_bytes, width, height,
       s3_key_original, upload_status, processing_status, processing_started_at,
       processing_attempts, last_error, created_at, updated_at, deleted_at,
       source_blob_id, is_active, crop_x, crop_y, crop_size
FROM avatars
WHERE user_id = $1 AND source_blob_id = $2
ORDER BY (deleted_at IS NULL) DESC, created_at DESC, id DESC
LIMIT 1;

-- name: DeactivateCompetingAvatars :exec
UPDATE avatars
SET is_active = FALSE, updated_at = NOW()
WHERE user_id = $1 AND deleted_at IS NULL AND id <> $2;

-- name: ActivateAvatar :execresult
UPDATE avatars
SET deleted_at = NULL, is_active = TRUE, updated_at = NOW()
WHERE id = $1 AND user_id = $2;

-- name: RestoreAvatarThumbnails :exec
UPDATE thumbnails
SET deleted_at = NULL
WHERE avatar_id = $1;

-- name: ClaimAvatarForProcessing :one
UPDATE avatars
SET processing_status = 'processing',
    processing_started_at = NOW(),
    processing_attempts = processing_attempts + 1,
    updated_at = NOW()
WHERE id = $1
  AND deleted_at IS NULL
  AND upload_status = 'completed'
  AND processing_status = 'pending'
RETURNING id, user_id, file_name, mime_type, size_bytes, width, height,
          s3_key_original, upload_status, processing_status,
          processing_started_at, processing_attempts, last_error,
          created_at, updated_at, deleted_at, source_blob_id, is_active,
          crop_x, crop_y, crop_size;

-- name: HardDeleteExpiredAvatars :execresult
DELETE FROM avatars
WHERE deleted_at IS NOT NULL AND deleted_at < $1;
