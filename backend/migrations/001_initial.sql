-- Registry Pulse initial database schema.
--
-- This file describes the complete schema for a fresh installation. It is
-- intentionally not a chain of ALTER TABLE compatibility steps. The API
-- records this baseline in schema_migrations so a container restart does not
-- execute it again.
CREATE EXTENSION pgcrypto;

CREATE TABLE test_images (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    reference text UNIQUE NOT NULL,
    enabled boolean NOT NULL DEFAULT true,
    max_bytes bigint NOT NULL DEFAULT 2097152,
    is_default boolean NOT NULL DEFAULT false,
    auth_strategy text NOT NULL DEFAULT 'anonymous'
        CHECK (auth_strategy IN ('anonymous', 'optional', 'required')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX test_images_single_default_idx
    ON test_images (is_default) WHERE is_default;

CREATE TABLE registry_categories (
    id text PRIMARY KEY,
    slug text UNIQUE NOT NULL,
    name text NOT NULL,
    description text NOT NULL DEFAULT '',
    icon text NOT NULL DEFAULT 'container',
    official_url text NOT NULL DEFAULT '',
    default_test_repository text NOT NULL DEFAULT 'library/alpine',
    default_test_tag text NOT NULL DEFAULT 'latest',
    default_test_image_id uuid REFERENCES test_images(id) ON DELETE SET NULL,
    default_probe_mode text NOT NULL DEFAULT 'registry',
    default_timeout_seconds integer NOT NULL DEFAULT 15,
    default_manifest_path text NOT NULL DEFAULT '/v2/{repository}/manifests/{reference}',
    auth_type text NOT NULL DEFAULT 'bearer_or_none',
    enabled boolean NOT NULL DEFAULT true,
    sort_order integer NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now()
);

-- An empty relation set means the image is unrestricted for that dimension.
CREATE TABLE test_image_categories (
    test_image_id uuid NOT NULL REFERENCES test_images(id) ON DELETE CASCADE,
    category_id text NOT NULL REFERENCES registry_categories(id) ON DELETE CASCADE,
    PRIMARY KEY (test_image_id, category_id)
);
CREATE INDEX test_image_categories_category_idx
    ON test_image_categories(category_id, test_image_id);

CREATE TABLE test_image_probe_modes (
    test_image_id uuid NOT NULL REFERENCES test_images(id) ON DELETE CASCADE,
    probe_mode text NOT NULL CHECK (probe_mode IN ('registry', 'manifest', 'http', 'docker_pull')),
    PRIMARY KEY (test_image_id, probe_mode)
);
CREATE INDEX test_image_probe_modes_mode_idx
    ON test_image_probe_modes(probe_mode, test_image_id);

CREATE TABLE registry_sources (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    category_id text NOT NULL REFERENCES registry_categories(id),
    name text NOT NULL,
    base_url text NOT NULL,
    display_url text NOT NULL DEFAULT '',
    registry_host text NOT NULL,
    description text NOT NULL DEFAULT '',
    provider text NOT NULL DEFAULT '',
    country text NOT NULL DEFAULT '',
    region text NOT NULL DEFAULT '',
    operator text NOT NULL DEFAULT '',
    tags jsonb NOT NULL DEFAULT '[]'::jsonb,
    is_official boolean NOT NULL DEFAULT false,
    is_cloudflare boolean NOT NULL DEFAULT false,
    is_recommended boolean NOT NULL DEFAULT false,
    is_enabled boolean NOT NULL DEFAULT false,
    priority integer NOT NULL DEFAULT 0,
    sort_order integer NOT NULL DEFAULT 0,
    maintenance boolean NOT NULL DEFAULT false,
    probe_config_custom boolean NOT NULL DEFAULT false,
    probe_mode text NOT NULL DEFAULT 'registry',
    test_repository text NOT NULL DEFAULT 'library/alpine',
    test_tag text NOT NULL DEFAULT 'latest',
    test_digest text NOT NULL DEFAULT '',
    request_timeout_seconds integer NOT NULL DEFAULT 10,
    download_test_bytes bigint NOT NULL DEFAULT 2097152,
    test_image_id uuid REFERENCES test_images(id) ON DELETE SET NULL,
    status text NOT NULL DEFAULT 'unknown',
    response_ms bigint NOT NULL DEFAULT 0,
    last_checked timestamptz,
    error text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX registry_sources_category_sort_idx
    ON registry_sources(category_id, sort_order, priority, name);

-- Credentials are independent from test images. Secrets are encrypted before
-- insertion; only selector metadata is used for matching and API responses.
CREATE TABLE credential_profiles (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name text UNIQUE NOT NULL,
    auth_type text NOT NULL DEFAULT 'basic'
        CHECK (auth_type IN ('basic', 'bearer', 'token')),
    username text NOT NULL DEFAULT '',
    secret_ciphertext bytea NOT NULL DEFAULT ''::bytea,
    secret_nonce bytea NOT NULL DEFAULT ''::bytea,
    secret_fingerprint text NOT NULL DEFAULT '',
    secret_last4 text NOT NULL DEFAULT '',
    source_id uuid REFERENCES registry_sources(id) ON DELETE CASCADE,
    registry_host text NOT NULL DEFAULT '',
    category_id text REFERENCES registry_categories(id) ON DELETE CASCADE,
    enabled boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (source_id IS NOT NULL OR NULLIF(registry_host, '') IS NOT NULL OR category_id IS NOT NULL)
);
CREATE INDEX credential_profiles_match_idx
    ON credential_profiles(source_id, lower(registry_host), category_id, enabled);

CREATE TABLE users (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    username text UNIQUE NOT NULL,
    password_hash text NOT NULL,
    is_active boolean NOT NULL DEFAULT true,
    totp_secret text NOT NULL DEFAULT '',
    totp_enabled boolean NOT NULL DEFAULT false,
    must_change_password boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE roles (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name text UNIQUE NOT NULL
);
CREATE TABLE permissions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name text UNIQUE NOT NULL
);
CREATE TABLE user_roles (
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id uuid NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, role_id)
);
CREATE TABLE role_permissions (
    role_id uuid NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    permission_id uuid NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_id)
);
CREATE TABLE tags (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name text UNIQUE NOT NULL
);
CREATE TABLE registry_source_tags (
    source_id uuid NOT NULL REFERENCES registry_sources(id) ON DELETE CASCADE,
    tag_id uuid NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    PRIMARY KEY (source_id, tag_id)
);

CREATE TABLE sessions (
    token_hash text PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX sessions_user_idx ON sessions(user_id);
CREATE INDEX sessions_expiry_idx ON sessions(expires_at);

CREATE TABLE incidents (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    source_id uuid NOT NULL REFERENCES registry_sources(id) ON DELETE CASCADE,
    status text NOT NULL,
    started_at timestamptz NOT NULL DEFAULT now(),
    resolved_at timestamptz,
    last_error text NOT NULL DEFAULT ''
);
CREATE INDEX incidents_source_status_idx ON incidents(source_id, status);
CREATE INDEX incidents_resolved_at_idx ON incidents(resolved_at)
    WHERE resolved_at IS NOT NULL;
CREATE UNIQUE INDEX incidents_one_active_per_source_idx
    ON incidents(source_id) WHERE resolved_at IS NULL;

CREATE TABLE incident_events (
    id bigserial PRIMARY KEY,
    incident_id uuid NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
    source_id uuid REFERENCES registry_sources(id) ON DELETE CASCADE,
    from_status text NOT NULL DEFAULT '',
    to_status text NOT NULL DEFAULT '',
    event_type text NOT NULL,
    message text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX incident_events_incident_created_idx
    ON incident_events(incident_id, created_at DESC, id DESC);
CREATE INDEX incident_events_source_created_idx
    ON incident_events(source_id, created_at DESC);

CREATE TABLE system_settings (
    key text PRIMARY KEY,
    value jsonb NOT NULL DEFAULT '{}'::jsonb,
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE audit_logs (
    id bigserial PRIMARY KEY,
    user_id uuid REFERENCES users(id) ON DELETE SET NULL,
    action text NOT NULL,
    resource text NOT NULL DEFAULT '',
    details jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX audit_logs_created_idx ON audit_logs(created_at DESC);
CREATE TABLE api_tokens (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid REFERENCES users(id) ON DELETE CASCADE,
    token_hash text UNIQUE NOT NULL,
    name text NOT NULL DEFAULT '',
    expires_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE notification_channels (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    type text NOT NULL,
    name text NOT NULL,
    enabled boolean NOT NULL DEFAULT false,
    config jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX notification_channels_type_idx ON notification_channels(type);
CREATE TABLE notification_rules (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    channel_id uuid NOT NULL REFERENCES notification_channels(id) ON DELETE CASCADE,
    event_type text NOT NULL,
    enabled boolean NOT NULL DEFAULT true,
    cooldown_seconds integer NOT NULL DEFAULT 300,
    aggregation_seconds integer NOT NULL DEFAULT 0,
    template text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(channel_id, event_type)
);
CREATE TABLE notification_logs (
    id bigserial PRIMARY KEY,
    channel_id uuid REFERENCES notification_channels(id) ON DELETE SET NULL,
    source_id uuid REFERENCES registry_sources(id) ON DELETE SET NULL,
    event_type text NOT NULL,
    status text NOT NULL,
    attempts integer NOT NULL DEFAULT 1,
    error text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX notification_logs_created_idx ON notification_logs(created_at DESC);
CREATE INDEX notification_logs_channel_event_window_idx
    ON notification_logs(channel_id, event_type, created_at DESC)
    WHERE status IN ('sent', 'coalesced');

CREATE TABLE probe_nodes (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name text UNIQUE NOT NULL,
    region text NOT NULL DEFAULT '',
    version text NOT NULL,
    capabilities jsonb NOT NULL DEFAULT '[]'::jsonb,
    status text NOT NULL DEFAULT 'offline',
    token_hash text UNIQUE NOT NULL,
    last_seen_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE probe_tasks (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    source_id uuid REFERENCES registry_sources(id) ON DELETE CASCADE,
    probe_node_id uuid REFERENCES probe_nodes(id) ON DELETE SET NULL,
    task_type text NOT NULL,
    payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    status text NOT NULL DEFAULT 'pending',
    lease_until timestamptz,
    started_at timestamptz,
    finished_at timestamptz,
    error text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX probe_tasks_status_idx ON probe_tasks(status, created_at);

CREATE TABLE probe_results (
    id bigserial PRIMARY KEY,
    source_id uuid NOT NULL REFERENCES registry_sources(id) ON DELETE CASCADE,
    probe_node_id uuid REFERENCES probe_nodes(id) ON DELETE SET NULL,
    task_id uuid REFERENCES probe_tasks(id) ON DELETE SET NULL,
    status text NOT NULL,
    dns_duration_ms bigint NOT NULL DEFAULT 0,
    tcp_duration_ms bigint NOT NULL DEFAULT 0,
    tls_duration_ms bigint NOT NULL DEFAULT 0,
    registry_duration_ms bigint NOT NULL DEFAULT 0,
    manifest_duration_ms bigint NOT NULL DEFAULT 0,
    blob_duration_ms bigint NOT NULL DEFAULT 0,
    blob_bytes bigint NOT NULL DEFAULT 0,
    blob_ttfb_ms bigint NOT NULL DEFAULT 0,
    blob_speed_bps bigint NOT NULL DEFAULT 0,
    http_status integer NOT NULL DEFAULT 0,
    remote_ip text NOT NULL DEFAULT '',
    remote_port integer NOT NULL DEFAULT 0,
    error_stage text NOT NULL DEFAULT '',
    error_code text NOT NULL DEFAULT '',
    error text NOT NULL DEFAULT '',
    error_message_public text NOT NULL DEFAULT '',
    error_message_internal text NOT NULL DEFAULT '',
    dns_success boolean NOT NULL DEFAULT false,
    resolved_ips jsonb NOT NULL DEFAULT '[]'::jsonb,
    dns_error text NOT NULL DEFAULT '',
    tcp_success boolean NOT NULL DEFAULT false,
    tcp_error text NOT NULL DEFAULT '',
    tls_success boolean NOT NULL DEFAULT false,
    tls_version text NOT NULL DEFAULT '',
    tls_cipher text NOT NULL DEFAULT '',
    tls_error text NOT NULL DEFAULT '',
    certificate_subject text NOT NULL DEFAULT '',
    certificate_issuer text NOT NULL DEFAULT '',
    certificate_not_before timestamptz,
    certificate_not_after timestamptz,
    certificate_days_remaining integer NOT NULL DEFAULT 0,
    registry_api_success boolean NOT NULL DEFAULT false,
    registry_api_status integer NOT NULL DEFAULT 0,
    registry_api_version text NOT NULL DEFAULT '',
    registry_api_error text NOT NULL DEFAULT '',
    manifest_success boolean NOT NULL DEFAULT false,
    manifest_status integer NOT NULL DEFAULT 0,
    manifest_content_type text NOT NULL DEFAULT '',
    manifest_digest text NOT NULL DEFAULT '',
    manifest_size bigint NOT NULL DEFAULT 0,
    manifest_error text NOT NULL DEFAULT '',
    blob_success boolean NOT NULL DEFAULT false,
    blob_status integer NOT NULL DEFAULT 0,
    blob_range_supported boolean NOT NULL DEFAULT false,
    blob_error text NOT NULL DEFAULT '',
    checked_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX probe_results_source_checked_idx
    ON probe_results(source_id, checked_at DESC);
CREATE INDEX probe_results_node_checked_idx
    ON probe_results(probe_node_id, checked_at DESC);
CREATE INDEX probe_results_status_checked_idx
    ON probe_results(status, checked_at DESC);
CREATE INDEX probe_results_task_idx ON probe_results(task_id);
CREATE INDEX probe_results_checked_at_idx ON probe_results(checked_at);

CREATE TABLE probe_stage_results (
    id bigserial PRIMARY KEY,
    probe_result_id bigint NOT NULL REFERENCES probe_results(id) ON DELETE CASCADE,
    stage text NOT NULL,
    status text NOT NULL,
    duration_ms bigint NOT NULL DEFAULT 0,
    error text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE probe_heartbeats (
    id bigserial PRIMARY KEY,
    probe_node_id uuid NOT NULL REFERENCES probe_nodes(id) ON DELETE CASCADE,
    status text NOT NULL DEFAULT 'online',
    version text NOT NULL DEFAULT '',
    capabilities jsonb NOT NULL DEFAULT '[]'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX probe_heartbeats_node_created_idx
    ON probe_heartbeats(probe_node_id, created_at DESC);
CREATE TABLE probe_hourly_stats (
    source_id uuid NOT NULL REFERENCES registry_sources(id) ON DELETE CASCADE,
    bucket timestamptz NOT NULL,
    samples bigint NOT NULL DEFAULT 0,
    online_samples bigint NOT NULL DEFAULT 0,
    avg_duration_ms double precision NOT NULL DEFAULT 0,
    PRIMARY KEY(source_id, bucket)
);
CREATE TABLE probe_daily_stats (
    source_id uuid NOT NULL REFERENCES registry_sources(id) ON DELETE CASCADE,
    bucket date NOT NULL,
    samples bigint NOT NULL DEFAULT 0,
    online_samples bigint NOT NULL DEFAULT 0,
    avg_duration_ms double precision NOT NULL DEFAULT 0,
    PRIMARY KEY(source_id, bucket)
);
