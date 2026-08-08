ALTER TABLE registry_sources ADD COLUMN IF NOT EXISTS maintenance boolean NOT NULL DEFAULT false;
