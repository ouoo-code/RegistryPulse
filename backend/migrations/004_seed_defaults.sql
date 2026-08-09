-- Initial application data for a fresh Registry Pulse installation.
-- Runtime probe results, users and sessions are intentionally not seeded.

INSERT INTO test_images(reference, enabled, max_bytes, is_default, auth_strategy)
VALUES
  ('beats/metricbeat:8.15.0', true, 2097152, false, 'optional'),
  ('dotnet/runtime:8.0', true, 2097152, false, 'optional'),
  ('google-containers/pause:3.9', true, 2097152, false, 'optional'),
  ('library/alpine:latest', true, 1048576, true, 'anonymous'),
  ('library/hello-world:latest', true, 1048576, false, 'anonymous'),
  ('nvidia/cuda:12.4.1-base-ubuntu22.04', true, 2097152, false, 'optional'),
  ('pause:3.9', true, 2097152, false, 'anonymous'),
  ('prometheus/busybox:latest', true, 2097152, false, 'optional'),
  ('stefanprodan/podinfo:latest', true, 2097152, false, 'optional');

INSERT INTO registry_categories(
    id, slug, name, description, icon, official_url,
    default_test_repository, default_test_tag, default_probe_mode,
    default_timeout_seconds, default_manifest_path, auth_type, enabled, sort_order
)
VALUES
  ('dockerhub', 'DockerHub', 'Docker Hub', 'Docker official container registry', 'docker', 'https://hub.docker.com', 'library/alpine', 'latest', 'registry', 15, '/v2/{repository}/manifests/{reference}', 'bearer_or_none', true, 10),
  ('ghcr', 'GHCR', 'GitHub Container Registry', 'GitHub OCI container registry', 'github', 'https://ghcr.io', 'stefanprodan/podinfo', 'latest', 'manifest', 20, '/v2/{repository}/manifests/{reference}', 'bearer_or_none', true, 20),
  ('quay', 'Quay', 'Quay', 'Red Hat Quay container registry', 'quay', 'https://quay.io', 'prometheus/busybox', 'latest', 'manifest', 20, '/v2/{repository}/manifests/{reference}', 'bearer_or_none', true, 30),
  ('mcr', 'MCR', 'Microsoft Container Registry', 'Microsoft container registry', 'microsoft', 'https://mcr.microsoft.com', 'dotnet/runtime', '8.0', 'manifest', 20, '/v2/{repository}/manifests/{reference}', 'bearer_or_none', true, 40),
  ('k8s', 'K8s', 'Kubernetes Registry', 'Kubernetes container registry', 'kubernetes', 'https://registry.k8s.io', 'pause', '3.9', 'manifest', 20, '/v2/{repository}/manifests/{reference}', 'bearer_or_none', true, 50),
  ('gcr', 'GCR', 'Google Container Registry', 'Google container registry', 'google', 'https://gcr.io', 'google-containers/pause', '3.9', 'manifest', 20, '/v2/{repository}/manifests/{reference}', 'bearer_or_none', true, 60),
  ('elastic', 'Elastic', 'Elastic Container Registry', 'Elastic official container registry', 'elastic', 'https://www.elastic.co', 'beats/metricbeat', '8.15.0', 'manifest', 20, '/v2/{repository}/manifests/{reference}', 'bearer_or_none', true, 70),
  ('nvcr', 'NVCR', 'NVIDIA Container Registry', 'NVIDIA container registry', 'nvidia', 'https://catalog.ngc.nvidia.com', 'nvidia/cuda', '12.4.1-base-ubuntu22.04', 'manifest', 30, '/v2/{repository}/manifests/{reference}', 'bearer_or_none', true, 80),
  ('custom', 'custom', 'Custom OCI Registry', 'Custom OCI-compatible registry', 'registry', '', 'library/alpine', 'latest', 'registry', 15, '/v2/{repository}/manifests/{reference}', 'bearer_or_none', true, 90);

UPDATE registry_categories c
SET default_test_image_id = i.id
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
  ELSE NULL
END;

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

INSERT INTO permissions(name)
VALUES
  ('source.read'), ('source.write'), ('probe.read'), ('probe.write'),
  ('incident.read'), ('settings.read'), ('settings.write'), ('audit.read'),
  ('agent.manage');
INSERT INTO roles(name) VALUES ('operator'), ('viewer');
INSERT INTO role_permissions(role_id, permission_id)
SELECT r.id, p.id
FROM roles r CROSS JOIN permissions p
WHERE r.name = 'operator'
  AND p.name IN ('source.read', 'source.write', 'probe.read', 'probe.write', 'incident.read', 'settings.read');
INSERT INTO role_permissions(role_id, permission_id)
SELECT r.id, p.id
FROM roles r CROSS JOIN permissions p
WHERE r.name = 'viewer'
  AND p.name IN ('source.read', 'probe.read', 'incident.read', 'settings.read');

INSERT INTO system_settings(key, value)
VALUES
  ('public_api_enabled', 'false'::jsonb),
  ('probe_interval_minutes', '{"value":60}'::jsonb),
  ('probe_retry_interval_minutes', '{"value":5}'::jsonb);
