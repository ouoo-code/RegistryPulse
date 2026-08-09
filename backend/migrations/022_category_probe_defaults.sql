ALTER TABLE registry_categories ADD COLUMN IF NOT EXISTS default_test_tag text NOT NULL DEFAULT 'latest';
ALTER TABLE registry_categories ADD COLUMN IF NOT EXISTS default_probe_mode text NOT NULL DEFAULT 'registry';
ALTER TABLE registry_categories ADD COLUMN IF NOT EXISTS default_timeout_seconds integer NOT NULL DEFAULT 15;
ALTER TABLE registry_categories ADD COLUMN IF NOT EXISTS default_test_image_id uuid REFERENCES test_images(id) ON DELETE SET NULL;

INSERT INTO test_images(reference, enabled, max_bytes, is_default)
VALUES
  ('stefanprodan/podinfo:latest', true, 2097152, false),
  ('prometheus/busybox:latest', true, 2097152, false),
  ('dotnet/runtime:8.0', true, 2097152, false),
  ('pause:3.9', true, 2097152, false),
  ('google-containers/pause:3.9', true, 2097152, false),
  ('beats/metricbeat:8.15.0', true, 2097152, false),
  ('nvidia/cuda:12.4.1-base-ubuntu22.04', true, 2097152, false)
ON CONFLICT (reference) DO NOTHING;

UPDATE registry_categories SET default_test_repository='library/alpine', default_test_tag='latest', default_probe_mode='registry', default_timeout_seconds=15 WHERE id='dockerhub';
UPDATE registry_categories SET default_test_repository='stefanprodan/podinfo', default_test_tag='latest', default_probe_mode='manifest', default_timeout_seconds=20 WHERE id='ghcr';
UPDATE registry_categories SET default_test_repository='prometheus/busybox', default_test_tag='latest', default_probe_mode='manifest', default_timeout_seconds=20 WHERE id='quay';
UPDATE registry_categories SET default_test_repository='dotnet/runtime', default_test_tag='8.0', default_probe_mode='manifest', default_timeout_seconds=20 WHERE id='mcr';
UPDATE registry_categories SET default_test_repository='pause', default_test_tag='3.9', default_probe_mode='manifest', default_timeout_seconds=20 WHERE id='k8s';
UPDATE registry_categories SET default_test_repository='google-containers/pause', default_test_tag='3.9', default_probe_mode='manifest', default_timeout_seconds=20 WHERE id='gcr';
UPDATE registry_categories SET default_test_repository='beats/metricbeat', default_test_tag='8.15.0', default_probe_mode='manifest', default_timeout_seconds=20 WHERE id='elastic';
UPDATE registry_categories SET default_test_repository='nvidia/cuda', default_test_tag='12.4.1-base-ubuntu22.04', default_probe_mode='manifest', default_timeout_seconds=30 WHERE id='nvcr';

UPDATE registry_categories c SET default_test_image_id = i.id
FROM test_images i
WHERE i.reference = CASE c.id
  WHEN 'dockerhub' THEN 'library/alpine:latest'
  WHEN 'ghcr' THEN 'stefanprodan/podinfo:latest'
  WHEN 'quay' THEN 'prometheus/busybox:latest'
  WHEN 'mcr' THEN 'dotnet/runtime:8.0'
  WHEN 'k8s' THEN 'pause:3.9'
  WHEN 'gcr' THEN 'google-containers/pause:3.9'
  WHEN 'elastic' THEN 'beats/metricbeat:8.15.0'
  WHEN 'nvcr' THEN 'nvidia/cuda:12.4.1-base-ubuntu22.04'
END;

ALTER TABLE registry_sources ADD COLUMN IF NOT EXISTS probe_config_custom boolean NOT NULL DEFAULT false;

-- Existing sources inherit their category defaults. An administrator can opt
-- into source-level values later by enabling the custom configuration flag.
UPDATE registry_sources SET probe_config_custom=false;
