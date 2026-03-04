-- Migration 002: Add owner column for per-user data isolation

ALTER TABLE responses ADD COLUMN IF NOT EXISTS owner TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_responses_owner ON responses(owner);
