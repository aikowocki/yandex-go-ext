ALTER TABLE thumbnails
    ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_thumbnails_retention
    ON thumbnails (deleted_at)
    WHERE deleted_at IS NOT NULL;
