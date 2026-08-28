-- name: CreateThumbnail :one
INSERT INTO thumbnails (
    id, avatar_id, size, s3_key, width, height, size_bytes, created_at, blob_id, derivation_id
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
ON CONFLICT (avatar_id, size) DO UPDATE SET
    s3_key = EXCLUDED.s3_key,
    width = EXCLUDED.width,
    height = EXCLUDED.height,
    size_bytes = EXCLUDED.size_bytes,
    blob_id = EXCLUDED.blob_id,
    derivation_id = EXCLUDED.derivation_id
RETURNING id, avatar_id, size, s3_key, width, height, size_bytes, created_at, deleted_at, blob_id, derivation_id;

-- name: ListThumbnailsByAvatarID :many
SELECT id, avatar_id, size, s3_key, width, height, size_bytes, created_at, deleted_at, blob_id, derivation_id
FROM thumbnails
WHERE avatar_id = $1 AND deleted_at IS NULL
ORDER BY width;

-- name: DeleteThumbnailsByAvatarID :exec
UPDATE thumbnails
SET deleted_at = NOW()
WHERE avatar_id = $1 AND deleted_at IS NULL;

-- name: HardDeleteExpiredThumbnails :execresult
DELETE FROM thumbnails
WHERE deleted_at IS NOT NULL AND deleted_at < $1;
