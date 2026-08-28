ALTER TABLE outbox_events
    ADD COLUMN IF NOT EXISTS claimed_until TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_outbox_claimable
    ON outbox_events (claimed_until, next_attempt_at, created_at)
    WHERE published_at IS NULL;
