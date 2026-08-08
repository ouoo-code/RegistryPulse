CREATE TABLE IF NOT EXISTS test_images (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    reference text UNIQUE NOT NULL,
    enabled boolean NOT NULL DEFAULT false,
    max_bytes bigint NOT NULL DEFAULT 2097152,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS probe_nodes (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name text UNIQUE NOT NULL,
    region text NOT NULL DEFAULT '',
    version text NOT NULL,
    capabilities jsonb NOT NULL DEFAULT '[]',
    status text NOT NULL DEFAULT 'offline',
    token_hash text UNIQUE NOT NULL,
    last_seen_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS probe_tasks (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    source_id uuid REFERENCES registry_sources(id) ON DELETE CASCADE,
    probe_node_id uuid REFERENCES probe_nodes(id) ON DELETE SET NULL,
    task_type text NOT NULL,
    payload jsonb NOT NULL DEFAULT '{}',
    status text NOT NULL DEFAULT 'pending',
    lease_until timestamptz,
    started_at timestamptz,
    finished_at timestamptz,
    error text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS probe_tasks_status_idx ON probe_tasks(status, created_at);
CREATE TABLE IF NOT EXISTS probe_stage_results (
    id bigserial PRIMARY KEY,
    probe_result_id bigint NOT NULL REFERENCES probe_results(id) ON DELETE CASCADE,
    stage text NOT NULL,
    status text NOT NULL,
    duration_ms bigint NOT NULL DEFAULT 0,
    error text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now()
);
