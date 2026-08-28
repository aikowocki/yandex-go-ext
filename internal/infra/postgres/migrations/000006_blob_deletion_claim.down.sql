UPDATE blobs
SET storage_status = 'failed'
WHERE storage_status = 'deleting';

ALTER TABLE blobs
    ALTER COLUMN storage_status TYPE TEXT
    USING storage_status::TEXT;

DROP TYPE blob_storage_status;
CREATE TYPE blob_storage_status AS ENUM ('uploading', 'ready', 'failed');

ALTER TABLE blobs
    ALTER COLUMN storage_status TYPE blob_storage_status
    USING storage_status::blob_storage_status;
