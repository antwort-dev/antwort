-- Migration 004: Add background processing columns for async responses.
-- Supports the distributed worker architecture (spec 044).

ALTER TABLE responses ADD COLUMN IF NOT EXISTS background BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE responses ADD COLUMN IF NOT EXISTS background_request JSONB;
ALTER TABLE responses ADD COLUMN IF NOT EXISTS worker_id TEXT;
ALTER TABLE responses ADD COLUMN IF NOT EXISTS worker_heartbeat TIMESTAMPTZ;
ALTER TABLE responses ADD COLUMN IF NOT EXISTS completed_at BIGINT;

-- Index for worker polling: find queued background responses efficiently.
CREATE INDEX IF NOT EXISTS idx_responses_status_background
    ON responses (status, background) WHERE deleted_at IS NULL;

-- Index for stale detection: find in_progress responses with old heartbeats.
CREATE INDEX IF NOT EXISTS idx_responses_worker_heartbeat
    ON responses (worker_heartbeat) WHERE status = 'in_progress' AND background = TRUE;
