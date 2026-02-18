-- Migration 001: Create responses table

CREATE TABLE IF NOT EXISTS schema_migrations (
    version    INTEGER PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS responses (
    id                   TEXT PRIMARY KEY,
    tenant_id            TEXT NOT NULL DEFAULT '',
    status               TEXT NOT NULL,
    model                TEXT NOT NULL,
    previous_response_id TEXT,
    input                JSONB NOT NULL DEFAULT '[]',
    output               JSONB NOT NULL DEFAULT '[]',
    usage_input_tokens   INTEGER NOT NULL DEFAULT 0,
    usage_output_tokens  INTEGER NOT NULL DEFAULT 0,
    usage_total_tokens   INTEGER NOT NULL DEFAULT 0,
    error                JSONB,
    extensions           JSONB,
    created_at           BIGINT NOT NULL,
    deleted_at           TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_responses_tenant ON responses(tenant_id);
CREATE INDEX IF NOT EXISTS idx_responses_previous ON responses(previous_response_id);
CREATE INDEX IF NOT EXISTS idx_responses_created ON responses(created_at);
