WITH ranked_active AS (
    SELECT id,
           row_number() OVER (PARTITION BY user_id ORDER BY created_at DESC, id DESC) AS row_number
    FROM avatars
    WHERE deleted_at IS NULL
)
UPDATE avatars AS a
SET deleted_at = NOW(),
    updated_at = NOW()
FROM ranked_active AS ranked
WHERE a.id = ranked.id
  AND ranked.row_number > 1;

UPDATE thumbnails AS t
SET deleted_at = NOW()
FROM avatars AS a
WHERE t.avatar_id = a.id
  AND a.deleted_at IS NOT NULL
  AND t.deleted_at IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_avatars_one_active_per_user
    ON avatars (user_id)
    WHERE deleted_at IS NULL;
