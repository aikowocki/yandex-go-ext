ALTER TABLE thumbnails DROP COLUMN IF EXISTS deleted_at;
DROP INDEX IF EXISTS idx_thumbnails_retention;
