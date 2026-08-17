ALTER TABLE avatars
    ADD COLUMN IF NOT EXISTS is_active BOOLEAN NOT NULL DEFAULT FALSE;

DROP INDEX IF EXISTS idx_avatars_one_active_per_user;

WITH ranked_visible AS (
    SELECT id,
           row_number() OVER (PARTITION BY user_id ORDER BY created_at DESC, id DESC) AS row_number
    FROM avatars
    WHERE deleted_at IS NULL
)
UPDATE avatars AS a
SET is_active = (ranked.row_number = 1)
FROM ranked_visible AS ranked
WHERE a.id = ranked.id;

CREATE UNIQUE INDEX IF NOT EXISTS idx_avatars_one_active_per_user
    ON avatars (user_id)
    WHERE is_active AND deleted_at IS NULL;
