-- Complete registry source metadata and per-source probe defaults.
ALTER TABLE registry_sources ADD COLUMN IF NOT EXISTS description text NOT NULL DEFAULT '';
ALTER TABLE registry_sources ADD COLUMN IF NOT EXISTS country text NOT NULL DEFAULT '';
ALTER TABLE registry_sources ADD COLUMN IF NOT EXISTS operator text NOT NULL DEFAULT '';
ALTER TABLE registry_sources ADD COLUMN IF NOT EXISTS is_official boolean NOT NULL DEFAULT false;
ALTER TABLE registry_sources ADD COLUMN IF NOT EXISTS is_cloudflare boolean NOT NULL DEFAULT false;
ALTER TABLE registry_sources ADD COLUMN IF NOT EXISTS is_recommended boolean NOT NULL DEFAULT false;
ALTER TABLE registry_sources ADD COLUMN IF NOT EXISTS priority integer NOT NULL DEFAULT 0;
ALTER TABLE registry_sources ADD COLUMN IF NOT EXISTS sort_order integer NOT NULL DEFAULT 0;
ALTER TABLE registry_sources ADD COLUMN IF NOT EXISTS test_repository text NOT NULL DEFAULT 'library/alpine';
ALTER TABLE registry_sources ADD COLUMN IF NOT EXISTS test_tag text NOT NULL DEFAULT 'latest';
ALTER TABLE registry_sources ADD COLUMN IF NOT EXISTS test_digest text NOT NULL DEFAULT '';
ALTER TABLE registry_sources ADD COLUMN IF NOT EXISTS request_timeout_seconds integer NOT NULL DEFAULT 10;
ALTER TABLE registry_sources ADD COLUMN IF NOT EXISTS download_test_bytes bigint NOT NULL DEFAULT 2097152;
CREATE INDEX IF NOT EXISTS registry_sources_category_sort_idx ON registry_sources(category_id, sort_order, priority, name);
