-- Current non-runtime initialization additions from the working PostgreSQL data.
-- Do not copy users, sessions, API tokens, probe nodes, audit logs or probe history.
-- Existing customized settings are preserved; only the old repository defaults are upgraded.
INSERT INTO test_images(reference, enabled, max_bytes)
VALUES
  ('beats/metricbeat:8.15.0', true, 2097152),
  ('dotnet/runtime:8.0', true, 2097152),
  ('google-containers/pause:3.9', true, 2097152),
  ('library/alpine:latest', true, 1048576),
  ('library/hello-world:latest', true, 1048576),
  ('nvidia/cuda:12.4.1-base-ubuntu22.04', true, 2097152),
  ('pause:3.9', true, 2097152),
  ('prometheus/busybox:latest', true, 2097152),
  ('stefanprodan/podinfo:latest', true, 2097152)
ON CONFLICT (reference) DO UPDATE
SET enabled = EXCLUDED.enabled,
    max_bytes = EXCLUDED.max_bytes
WHERE test_images.max_bytes = 2097152
  AND test_images.enabled = true;

-- Keep one deterministic default for a clean installation without overriding
-- an administrator's explicitly selected default.
UPDATE test_images
SET is_default = true, updated_at = now()
WHERE reference = 'library/alpine:latest'
  AND NOT EXISTS (SELECT 1 FROM test_images WHERE is_default);

UPDATE registry_categories SET default_test_image_id=(SELECT id FROM test_images WHERE reference='library/alpine:latest') WHERE id='dockerhub';
UPDATE registry_categories SET default_test_image_id=(SELECT id FROM test_images WHERE reference='beats/metricbeat:8.15.0') WHERE id='elastic';
UPDATE registry_categories SET default_test_image_id=(SELECT id FROM test_images WHERE reference='google-containers/pause:3.9') WHERE id='gcr';
UPDATE registry_categories SET default_test_image_id=(SELECT id FROM test_images WHERE reference='stefanprodan/podinfo:latest') WHERE id='ghcr';
UPDATE registry_categories SET default_test_image_id=(SELECT id FROM test_images WHERE reference='pause:3.9') WHERE id='k8s';
UPDATE registry_categories SET default_test_image_id=(SELECT id FROM test_images WHERE reference='dotnet/runtime:8.0') WHERE id='mcr';
UPDATE registry_categories SET default_test_image_id=(SELECT id FROM test_images WHERE reference='nvidia/cuda:12.4.1-base-ubuntu22.04') WHERE id='nvcr';
UPDATE registry_categories SET default_test_image_id=(SELECT id FROM test_images WHERE reference='prometheus/busybox:latest') WHERE id='quay';

INSERT INTO system_settings(key, value)
VALUES
  ('public_api_enabled', 'false'::jsonb),
  ('probe_interval_minutes', '{"value":60}'::jsonb),
  ('probe_retry_interval_minutes', '{"value":5}'::jsonb)
ON CONFLICT (key) DO UPDATE
SET value = EXCLUDED.value, updated_at = now()
WHERE (system_settings.key = 'probe_interval_minutes' AND system_settings.value = '{"value":30}'::jsonb)
   OR (system_settings.key = 'probe_retry_interval_minutes' AND system_settings.value = '{"value":3}'::jsonb);
