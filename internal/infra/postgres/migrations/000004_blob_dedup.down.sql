ALTER TABLE thumbnails DROP CONSTRAINT IF EXISTS thumbnails_derivation_fk;
ALTER TABLE thumbnails DROP COLUMN IF EXISTS derivation_id;
ALTER TABLE thumbnails DROP COLUMN IF EXISTS blob_id;
ALTER TABLE avatars DROP COLUMN IF EXISTS source_blob_id;
DROP TABLE IF EXISTS blob_derivations;
DROP TABLE IF EXISTS blobs;
