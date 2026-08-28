DROP INDEX IF EXISTS idx_avatars_one_active_per_user;
ALTER TABLE avatars DROP COLUMN IF EXISTS is_active;
