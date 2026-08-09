-- Keep category IDs as stable machine keys. Slugs are the human-facing
-- category codes shown in the UI and may use the preferred casing.
UPDATE registry_categories
SET slug = CASE id
  WHEN 'dockerhub' THEN 'DockerHub'
  WHEN 'ghcr' THEN 'GHCR'
  WHEN 'quay' THEN 'Quay'
  WHEN 'mcr' THEN 'MCR'
  WHEN 'k8s' THEN 'K8s'
  WHEN 'gcr' THEN 'GCR'
  WHEN 'elastic' THEN 'Elastic'
  WHEN 'nvcr' THEN 'NVCR'
  ELSE slug
END
WHERE id IN ('dockerhub', 'ghcr', 'quay', 'mcr', 'k8s', 'gcr', 'elastic', 'nvcr');
