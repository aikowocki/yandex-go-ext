-- name: GetOrCreateBlob :one
INSERT INTO blobs (
    id, sha256, size_bytes, mime_type, width, height, object_key,
    storage_status, last_error, created_at, updated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
ON CONFLICT (sha256) DO UPDATE SET updated_at = blobs.updated_at
RETURNING id, sha256, size_bytes, mime_type, width, height, object_key,
          storage_status, last_error, created_at, updated_at;

-- name: MarkBlobReady :execresult
UPDATE blobs
SET storage_status = 'ready', last_error = '', updated_at = NOW()
WHERE id = $1;

-- name: MarkBlobFailed :execresult
UPDATE blobs
SET storage_status = 'failed', last_error = $2, updated_at = NOW()
WHERE id = $1;

-- name: EnsureBlobDerivation :one
INSERT INTO blob_derivations (
    id, parent_blob_id, derived_blob_id, variant, processor_version,
    output_format, status, created_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (parent_blob_id, variant, processor_version, output_format)
DO UPDATE SET status = blob_derivations.status
RETURNING id, parent_blob_id, derived_blob_id, variant, processor_version,
          output_format, status, created_at;

-- name: LinkThumbnailBlob :execresult
UPDATE thumbnails
SET blob_id = $2, derivation_id = $3
WHERE id = $1;

-- name: ListUnreferencedBlobs :many
SELECT b.id, b.sha256, b.size_bytes, b.mime_type, b.width, b.height,
       b.object_key, b.storage_status, b.last_error, b.created_at, b.updated_at
FROM blobs b
WHERE b.created_at < $1
  AND NOT EXISTS (SELECT 1 FROM avatars a WHERE a.source_blob_id = b.id)
  AND NOT EXISTS (SELECT 1 FROM thumbnails t WHERE t.blob_id = b.id)
ORDER BY b.created_at
LIMIT $2;

-- name: DeleteBlobDerivations :exec
DELETE FROM blob_derivations
WHERE parent_blob_id = $1 OR derived_blob_id = $1;

-- name: DeleteBlobRow :exec
DELETE FROM blobs b
WHERE b.id = $1
  AND NOT EXISTS (SELECT 1 FROM avatars a WHERE a.source_blob_id = b.id)
  AND NOT EXISTS (SELECT 1 FROM thumbnails t WHERE t.blob_id = b.id);

-- name: HasBlobObject :one
SELECT EXISTS (SELECT 1 FROM blobs WHERE object_key = $1);

-- name: ClaimBlobForDeletion :one
UPDATE blobs b
SET storage_status = 'deleting', updated_at = NOW()
WHERE b.id = $1
  AND (b.storage_status <> 'deleting' OR b.updated_at < NOW() - INTERVAL '1 hour')
  AND NOT EXISTS (SELECT 1 FROM avatars a WHERE a.source_blob_id = b.id)
  AND NOT EXISTS (SELECT 1 FROM thumbnails t WHERE t.blob_id = b.id)
RETURNING b.id, b.sha256, b.size_bytes, b.mime_type, b.width, b.height,
          b.object_key, b.storage_status, b.last_error, b.created_at, b.updated_at;
