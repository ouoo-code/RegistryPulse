ALTER TABLE registry_sources ADD COLUMN IF NOT EXISTS display_url text NOT NULL DEFAULT '';
ALTER TABLE registry_sources ADD COLUMN IF NOT EXISTS status text NOT NULL DEFAULT 'unknown';
ALTER TABLE registry_sources ADD COLUMN IF NOT EXISTS response_ms bigint NOT NULL DEFAULT 0;
ALTER TABLE registry_sources ADD COLUMN IF NOT EXISTS last_checked timestamptz;
ALTER TABLE registry_sources ADD COLUMN IF NOT EXISTS error text NOT NULL DEFAULT '';
ALTER TABLE probe_results ADD COLUMN IF NOT EXISTS manifest_duration_ms bigint NOT NULL DEFAULT 0;
CREATE TABLE IF NOT EXISTS sessions (
    token_hash text PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS sessions_user_idx ON sessions(user_id);
CREATE INDEX IF NOT EXISTS sessions_expiry_idx ON sessions(expires_at);
