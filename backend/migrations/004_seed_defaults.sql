-- Initial application data for a fresh Registry Pulse installation.
-- Runtime probe results, users and sessions are intentionally not seeded.

INSERT INTO test_images(reference, enabled, max_bytes, is_default)
VALUES
  ('beats/metricbeat:8.15.0', true, 2097152, false),
  ('dotnet/runtime:8.0', true, 2097152, false),
  ('google-containers/pause:3.9', true, 2097152, false),
  ('library/alpine:latest', true, 1048576, true),
  ('library/hello-world:latest', true, 1048576, false),
  ('nvidia/cuda:12.4.1-base-ubuntu22.04', true, 2097152, false),
  ('pause:3.9', true, 2097152, false),
  ('prometheus/busybox:latest', true, 2097152, false),
  ('stefanprodan/podinfo:latest', true, 2097152, false);

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
