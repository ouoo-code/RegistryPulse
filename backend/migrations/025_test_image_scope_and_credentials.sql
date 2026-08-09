-- Schema-only upgrade for installations created before the fresh-install
-- baseline. It preserves all existing probe data and only adds the new
-- test-image scope and encrypted credential metadata.

ALTER TABLE test_images
    ADD COLUMN IF NOT EXISTS auth_strategy text NOT NULL DEFAULT 'anonymous';

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'test_images_auth_strategy_check'
    ) THEN
        ALTER TABLE test_images
            ADD CONSTRAINT test_images_auth_strategy_check
            CHECK (auth_strategy IN ('anonymous', 'optional', 'required'));
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS test_image_categories (
    test_image_id uuid NOT NULL REFERENCES test_images(id) ON DELETE CASCADE,
    category_id text NOT NULL REFERENCES registry_categories(id) ON DELETE CASCADE,
    PRIMARY KEY (test_image_id, category_id)
);
CREATE INDEX IF NOT EXISTS test_image_categories_category_idx
    ON test_image_categories(category_id, test_image_id);

CREATE TABLE IF NOT EXISTS test_image_probe_modes (
    test_image_id uuid NOT NULL REFERENCES test_images(id) ON DELETE CASCADE,
    probe_mode text NOT NULL CHECK (probe_mode IN ('registry', 'manifest', 'http', 'docker_pull')),
    PRIMARY KEY (test_image_id, probe_mode)
);
CREATE INDEX IF NOT EXISTS test_image_probe_modes_mode_idx
    ON test_image_probe_modes(probe_mode, test_image_id);

CREATE TABLE IF NOT EXISTS credential_profiles (
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
CREATE INDEX IF NOT EXISTS credential_profiles_match_idx
    ON credential_profiles(source_id, lower(registry_host), category_id, enabled);

-- Existing default images keep working without a restrictive scope. These
-- associations narrow only the known registry-specific examples; an empty
-- association remains the documented unrestricted state.
INSERT INTO test_image_categories(test_image_id, category_id)
SELECT i.id, mapping.category_id
FROM test_images i
JOIN (VALUES
  ('beats/metricbeat:8.15.0', 'elastic'),
  ('dotnet/runtime:8.0', 'mcr'),
  ('google-containers/pause:3.9', 'gcr'),
  ('library/alpine:latest', 'dockerhub'),
  ('library/alpine:latest', 'custom'),
  ('nvidia/cuda:12.4.1-base-ubuntu22.04', 'nvcr'),
  ('pause:3.9', 'k8s'),
  ('prometheus/busybox:latest', 'quay'),
  ('stefanprodan/podinfo:latest', 'ghcr')
) AS mapping(reference, category_id) ON mapping.reference = i.reference
ON CONFLICT DO NOTHING;

INSERT INTO test_image_probe_modes(test_image_id, probe_mode)
SELECT i.id, mode.probe_mode
FROM test_images i
CROSS JOIN (VALUES ('registry'), ('manifest'), ('docker_pull')) AS mode(probe_mode)
WHERE i.reference <> 'library/hello-world:latest'
ON CONFLICT DO NOTHING;
