CREATE TABLE IF NOT EXISTS blobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    sha256 BYTEA NOT NULL UNIQUE CHECK (octet_length(sha256) = 32),
    size_bytes BIGINT NOT NULL CHECK (size_bytes >= 0),
    mime_type TEXT NOT NULL,
    width INTEGER NOT NULL CHECK (width > 0),
    height INTEGER NOT NULL CHECK (height > 0),
    object_key TEXT NOT NULL UNIQUE,
    storage_status TEXT NOT NULL CHECK (storage_status IN ('uploading', 'ready', 'failed')),
    last_error TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE avatars
    DROP CONSTRAINT IF EXISTS avatars_s3_key_original_key;

ALTER TABLE avatars
    ADD COLUMN IF NOT EXISTS source_blob_id UUID REFERENCES blobs(id);

ALTER TABLE thumbnails
    DROP CONSTRAINT IF EXISTS thumbnails_s3_key_key;

ALTER TABLE thumbnails
    ADD COLUMN IF NOT EXISTS blob_id UUID REFERENCES blobs(id),
    ADD COLUMN IF NOT EXISTS derivation_id UUID;

CREATE TABLE IF NOT EXISTS blob_derivations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    parent_blob_id UUID NOT NULL REFERENCES blobs(id),
    derived_blob_id UUID NOT NULL REFERENCES blobs(id),
    variant TEXT NOT NULL,
    processor_version TEXT NOT NULL,
    output_format TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('uploading', 'ready', 'failed')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (parent_blob_id, variant, processor_version, output_format)
);

ALTER TABLE thumbnails
    ADD CONSTRAINT thumbnails_derivation_fk
    FOREIGN KEY (derivation_id) REFERENCES blob_derivations(id);

CREATE INDEX IF NOT EXISTS idx_avatars_source_blob ON avatars (source_blob_id);
CREATE INDEX IF NOT EXISTS idx_thumbnails_blob ON thumbnails (blob_id);
CREATE INDEX IF NOT EXISTS idx_blob_derivations_derived ON blob_derivations (derived_blob_id);
