ALTER TABLE probe_results ADD COLUMN IF NOT EXISTS probe_node_id uuid REFERENCES probe_nodes(id) ON DELETE SET NULL;
ALTER TABLE probe_results ADD COLUMN IF NOT EXISTS task_id uuid REFERENCES probe_tasks(id) ON DELETE SET NULL;
ALTER TABLE probe_results ADD COLUMN IF NOT EXISTS blob_ttfb_ms bigint NOT NULL DEFAULT 0;
ALTER TABLE probe_results ADD COLUMN IF NOT EXISTS http_status integer NOT NULL DEFAULT 0;
ALTER TABLE probe_results ADD COLUMN IF NOT EXISTS remote_ip text NOT NULL DEFAULT '';
ALTER TABLE probe_results ADD COLUMN IF NOT EXISTS error_stage text NOT NULL DEFAULT '';
ALTER TABLE probe_results ADD COLUMN IF NOT EXISTS error_code text NOT NULL DEFAULT '';
ALTER TABLE probe_results ADD COLUMN IF NOT EXISTS error_message_public text NOT NULL DEFAULT '';
ALTER TABLE probe_results ADD COLUMN IF NOT EXISTS error_message_internal text NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS probe_results_source_checked_idx ON probe_results(source_id, checked_at DESC);
CREATE INDEX IF NOT EXISTS probe_results_node_checked_idx ON probe_results(probe_node_id, checked_at DESC);
CREATE INDEX IF NOT EXISTS probe_results_status_checked_idx ON probe_results(status, checked_at DESC);
CREATE INDEX IF NOT EXISTS probe_results_task_idx ON probe_results(task_id);
CREATE TABLE IF NOT EXISTS probe_heartbeats (
    id bigserial PRIMARY KEY,
    probe_node_id uuid NOT NULL REFERENCES probe_nodes(id) ON DELETE CASCADE,
    status text NOT NULL DEFAULT 'online',
    version text NOT NULL DEFAULT '',
    capabilities jsonb NOT NULL DEFAULT '[]',
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS probe_heartbeats_node_created_idx ON probe_heartbeats(probe_node_id, created_at DESC);
CREATE TABLE IF NOT EXISTS probe_hourly_stats (
    source_id uuid NOT NULL REFERENCES registry_sources(id) ON DELETE CASCADE,
    bucket timestamptz NOT NULL,
    samples bigint NOT NULL DEFAULT 0,
    online_samples bigint NOT NULL DEFAULT 0,
    avg_duration_ms double precision NOT NULL DEFAULT 0,
    PRIMARY KEY(source_id, bucket)
);
CREATE TABLE IF NOT EXISTS probe_daily_stats (
    source_id uuid NOT NULL REFERENCES registry_sources(id) ON DELETE CASCADE,
    bucket date NOT NULL,
    samples bigint NOT NULL DEFAULT 0,
    online_samples bigint NOT NULL DEFAULT 0,
    avg_duration_ms double precision NOT NULL DEFAULT 0,
    PRIMARY KEY(source_id, bucket)
);
