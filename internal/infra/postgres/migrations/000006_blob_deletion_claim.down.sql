ALTER TABLE blobs
    DROP CONSTRAINT IF EXISTS blobs_storage_status_check;

ALTER TABLE blobs
    ADD CONSTRAINT blobs_storage_status_check
    CHECK (storage_status IN ('uploading', 'ready', 'failed'));
