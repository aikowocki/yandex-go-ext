-- name: SaveOutboxEvent :exec
INSERT INTO outbox_events (id, topic, payload, attempts, next_attempt_at, last_error, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: ClaimPendingOutboxEvents :many
WITH candidates AS (
    SELECT id
    FROM outbox_events
    WHERE published_at IS NULL
      AND next_attempt_at <= NOW()
      AND (claimed_until IS NULL OR claimed_until < NOW())
    ORDER BY created_at
    FOR UPDATE SKIP LOCKED
    LIMIT $1
)
UPDATE outbox_events AS events
SET claimed_until = NOW() + ($2 * INTERVAL '1 second')
FROM candidates
WHERE events.id = candidates.id
RETURNING events.id, events.topic, events.payload, events.attempts,
          events.next_attempt_at, events.last_error, events.created_at;

-- name: ListPendingOutboxEvents :many
SELECT id, topic, payload, attempts, next_attempt_at, last_error, created_at
FROM outbox_events
WHERE published_at IS NULL
  AND next_attempt_at <= NOW()
  AND (claimed_until IS NULL OR claimed_until < NOW())
ORDER BY created_at
LIMIT $1;

-- name: MarkOutboxEventPublished :execresult
UPDATE outbox_events
SET published_at = NOW(), last_error = '', claimed_until = NULL
WHERE id = $1 AND published_at IS NULL;

-- name: MarkOutboxEventFailed :execresult
UPDATE outbox_events
SET attempts = attempts + 1, next_attempt_at = $2, last_error = $3, claimed_until = NULL
WHERE id = $1 AND published_at IS NULL;
