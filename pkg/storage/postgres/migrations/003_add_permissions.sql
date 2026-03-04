ALTER TABLE vector_stores ADD COLUMN IF NOT EXISTS permissions TEXT NOT NULL DEFAULT 'rwd|---|---';
ALTER TABLE files ADD COLUMN IF NOT EXISTS permissions TEXT NOT NULL DEFAULT 'rwd|---|---';
