-- Current source catalog snapshot generated from the working PostgreSQL data.
-- Runtime status, timestamps, probe history, sessions and secrets are intentionally excluded.
-- This migration is for fresh deployments; existing rows are preserved by the identity guard.
WITH seed(category_id, name, base_url, display_url, registry_host, description, provider,
          country, region, operator, tags, is_official, is_cloudflare, is_recommended,
          is_enabled, priority, sort_order, maintenance, probe_config_custom, probe_mode,
          test_repository, test_tag, test_digest, request_timeout_seconds, download_test_bytes) AS (
VALUES
  ('dockerhub', 'Docker Hub', 'https://registry-1.docker.io', 'https://registry-1.docker.io', 'registry-1.docker.io', '', '', '', '', '', '["CloudFront"]'::jsonb, true, false, false, true, 0, 0, false, false, 'registry', 'library/alpine', 'latest', '', 15, 1048576),
  ('dockerhub', 'mirror.ccs.tencentyun.com', 'https://mirror.ccs.tencentyun.com', 'https://mirror.ccs.tencentyun.com', 'mirror.ccs.tencentyun.com', '', '', '', '', '', '["mirror"]'::jsonb, false, false, false, true, 0, 1, false, false, 'registry', 'library/alpine', 'latest', '', 15, 2097152),
  ('dockerhub', 'docker.1ms.run (Free)', 'https://docker.1ms.run', 'https://docker.1ms.run', 'docker.1ms.run', '', '', '', '', '', '["CloudFlare"]'::jsonb, false, true, true, true, 0, 2, false, false, 'registry', 'library/alpine', 'latest', '', 15, 2097152),
  ('dockerhub', '1Panel', 'https://docker.1panel.live', 'https://docker.1panel.live', 'docker.1panel.live', '描述', '提供商', '国家', '地区', '运营方', '["1Panel","CloudFlare"]'::jsonb, false, true, false, true, 0, 4, false, false, 'registry', 'library/alpine', 'latest', '', 15, 1048576),
  ('dockerhub', 'docker.sparkcr.cn (Free)', 'https://docker.sparkcr.cn', 'https://docker.sparkcr.cn', 'docker.sparkcr.cn', '', '', '', '', '', '["ESA"]'::jsonb, false, false, false, true, 0, 5, false, false, 'registry', 'library/alpine', 'latest', '', 15, 1048576),
  ('dockerhub', 'hub.rat.dev', 'https://hub.rat.dev', 'https://hub.rat.dev', 'hub.rat.dev', '', '', '', '', '', '["CloudFlare"]'::jsonb, false, true, false, true, 0, 6, false, false, 'registry', 'library/alpine', 'latest', '', 15, 1048576),
  ('dockerhub', 'docker.xuanyuan.me (Free)', 'https://docker.xuanyuan.me', 'https://docker.xuanyuan.me', 'docker.xuanyuan.me', '', '', '', '', '', '["CloudFlare"]'::jsonb, false, true, false, true, 0, 7, false, false, 'registry', 'library/alpine', 'latest', '', 15, 1048576),
  ('dockerhub', 'docker.xuanyuan.run (Pro)', 'https://docker.xuanyuan.run', 'https://docker.xuanyuan.run', 'docker.xuanyuan.run', '', '', '', '', '', '["mirror"]'::jsonb, false, false, false, true, 0, 8, false, false, 'registry', 'library/alpine', 'latest', '', 15, 1048576),
  ('dockerhub', 'docker.xuanyuan.dev (Pro)', 'https://docker.xuanyuan.dev', 'https://docker.xuanyuan.dev', 'docker.xuanyuan.dev', '', '', '', '', '', '["CloudFlare"]'::jsonb, false, true, false, true, 0, 9, false, false, 'registry', 'library/alpine', 'latest', '', 15, 1048576),
  ('dockerhub', 'DockerProxy', 'https://dockerproxy.net', 'https://dockerproxy.net', 'dockerproxy.net', '', '', '', '', '', '["DockerProxy","Oracle CDN"]'::jsonb, false, false, false, true, 0, 10, false, false, 'registry', 'library/alpine', 'latest', '', 15, 1048576),
  ('dockerhub', 'docker-registry.nmqu.com', 'https://docker-registry.nmqu.com', 'https://docker-registry.nmqu.com', 'docker-registry.nmqu.com', '', '', '', '', '', '["CloudFlare"]'::jsonb, false, true, false, true, 0, 11, false, false, 'registry', 'library/alpine', 'latest', '', 15, 1048576),
  ('dockerhub', 'hub.amingg.com 1', 'https://hub.amingg.com', 'https://hub.amingg.com', 'hub.amingg.com', '', '', '', '', '', '["CloudFlare"]'::jsonb, false, true, false, true, 0, 12, false, false, 'registry', 'library/alpine', 'latest', '', 15, 1048576),
  ('dockerhub', 'docker.amingg.com 2', 'https://docker.amingg.com', 'https://docker.amingg.com', 'docker.amingg.com', '', '', '', '', '', '["CloudFlare"]'::jsonb, false, true, false, true, 0, 13, false, false, 'registry', 'library/alpine', 'latest', '', 15, 1048576),
  ('dockerhub', 'docker.hlmirror.com', 'https://docker.hlmirror.com', 'https://docker.hlmirror.com', 'docker.hlmirror.com', '', '', '', '', '', '["CloudFlare"]'::jsonb, false, true, false, true, 0, 14, false, false, 'registry', 'library/alpine', 'latest', '', 15, 1048576),
  ('dockerhub', 'hub1.nat.tf 1', 'https://hub1.nat.tf', 'https://hub1.nat.tf', 'hub1.nat.tf', '', '', '', '', '', '["Nginx"]'::jsonb, false, false, false, true, 0, 15, false, false, 'registry', 'library/alpine', 'latest', '', 15, 2097152),
  ('dockerhub', 'hub2.nat.tf 2', 'https://hub2.nat.tf', 'https://hub2.nat.tf', 'hub2.nat.tf', '', '', '', '', '', '["Nginx"]'::jsonb, false, false, false, true, 0, 16, false, false, 'registry', 'library/alpine', 'latest', '', 15, 2097152),
  ('dockerhub', 'hub3.nat.tf', 'https://hub3.nat.tf', 'https://hub3.nat.tf', 'hub3.nat.tf', '', '', '', '', '', '["Nginx"]'::jsonb, false, false, false, true, 0, 17, false, false, 'registry', 'library/alpine', 'latest', '', 15, 2097152),
  ('dockerhub', 'hub4.nat.tf', 'https://hub4.nat.tf', 'https://hub4.nat.tf', 'hub4.nat.tf', '', '', '', '', '', '["Nginx"]'::jsonb, false, false, false, true, 0, 18, false, false, 'registry', 'library/alpine', 'latest', '', 15, 2097152),
  ('dockerhub', 'DaoCloud', 'https://docker.m.daocloud.io', 'https://docker.m.daocloud.io', 'docker.m.daocloud.io', '', '', '', '', '', '["DaoCloud"]'::jsonb, false, false, true, true, 0, 19, false, false, 'registry', 'library/alpine', 'latest', '', 15, 2097152),
  ('dockerhub', 'docker.kejilion.pro', 'https://docker.kejilion.pro', 'https://docker.kejilion.pro', 'docker.kejilion.pro', '', '', '', '', '', '["Nginx"]'::jsonb, false, false, false, true, 0, 20, false, false, 'registry', 'library/alpine', 'latest', '', 15, 1048576),
  ('dockerhub', 'docker.367231.xyz 1', 'https://docker.367231.xyz', 'https://docker.367231.xyz', 'docker.367231.xyz', '', '', '', '', '', '["CloudFlare"]'::jsonb, false, true, false, true, 0, 21, false, false, 'registry', 'library/alpine', 'latest', '', 15, 1048576),
  ('dockerhub', 'hub.1panel.dev 1', 'https://hub.1panel.dev', 'https://hub.1panel.dev', 'hub.1panel.dev', '', '', '', '', '', '["CloudFlare"]'::jsonb, false, true, false, true, 0, 22, false, false, 'registry', 'library/alpine', 'latest', '', 15, 1048576),
  ('dockerhub', 'SUNBALCONY 1', 'https://dockerproxy.cool', 'https://dockerproxy.cool', 'dockerproxy.cool', '', '', '', '', '', '["EdgeOne"]'::jsonb, false, false, false, true, 0, 23, false, false, 'registry', 'library/alpine', 'latest', '', 15, 1048576),
  ('dockerhub', 'docker.fnnas.com', 'https://docker.fnnas.com', 'https://docker.fnnas.com', 'docker.fnnas.com', '', '', '', '', '', '["Nginx"]'::jsonb, false, false, false, true, 0, 24, false, false, 'registry', 'library/alpine', 'latest', '', 15, 1048576),
  ('elastic', 'Elastic', 'https://docker.elastic.co', 'https://docker.elastic.co', 'docker.elastic.co', '', '', '', '', '', '["Google Cloud"]'::jsonb, true, false, false, true, 0, 0, false, false, 'registry', 'library/alpine', 'latest', '', 15, 1048576),
  ('elastic', 'elastic.sparkcr.cn (Free)', 'https://elastic.sparkcr.cn', 'https://elastic.sparkcr.cn', 'elastic.sparkcr.cn', '', '', '', '', '', '["ESA"]'::jsonb, false, false, false, true, 0, 1, false, false, 'registry', 'library/alpine', 'latest', '', 15, 1048576),
  ('elastic', 'DaoCloud', 'https://elastic.m.daocloud.io', 'https://elastic.m.daocloud.io', 'elastic.m.daocloud.io', '', '', '', '', '', '["DaoCloud"]'::jsonb, false, false, false, true, 0, 2, false, false, 'registry', 'library/alpine', 'latest', '', 15, 1048576),
  ('elastic', 'elastic.1ms.run (Paid)', 'https://elastic.1ms.run', 'https://elastic.1ms.run', 'elastic.1ms.run', '', '', '', '', '', '["CDN"]'::jsonb, false, false, false, true, 0, 3, false, false, 'registry', 'library/alpine', 'latest', '', 15, 1048576),
  ('gcr', 'GCR', 'https://gcr.io', 'https://gcr.io', 'gcr.io', '', '', '', '', '', '["Google Cloud"]'::jsonb, true, false, false, true, 0, 0, false, false, 'registry', 'library/alpine', 'latest', '', 15, 1048576),
  ('gcr', 'gcr.nju.edu.cn', 'https://gcr.nju.edu.cn', 'https://gcr.nju.edu.cn', 'gcr.nju.edu.cn', '', '', '', '', '', '["mirror"]'::jsonb, false, false, false, true, 0, 1, false, false, 'registry', 'library/alpine', 'latest', '', 15, 1048576),
  ('gcr', 'DaoCloud', 'https://gcr.m.daocloud.io', 'https://gcr.m.daocloud.io', 'gcr.m.daocloud.io', '', '', '', '', '', '["DaoCloud"]'::jsonb, false, false, false, true, 0, 2, false, false, 'registry', 'library/alpine', 'latest', '', 15, 1048576),
  ('gcr', 'gcr.sparkcr.cn (Free)', 'https://gcr.sparkcr.cn', 'https://gcr.sparkcr.cn', 'gcr.sparkcr.cn', '', '', '', '', '', '["ESA"]'::jsonb, false, false, false, true, 0, 3, false, false, 'registry', 'library/alpine', 'latest', '', 15, 1048576),
  ('gcr', 'gcr.1ms.run (Paid)', 'https://gcr.1ms.run', 'https://gcr.1ms.run', 'gcr.1ms.run', '', '', '', '', '', '["CDN"]'::jsonb, false, false, false, true, 0, 4, false, false, 'registry', 'library/alpine', 'latest', '', 15, 1048576),
  ('gcr', 'DockerProxy', 'https://gcr.dockerproxy.net', 'https://gcr.dockerproxy.net', 'gcr.dockerproxy.net', '', '', '', '', '', '["DockerProxy","Oracle CDN"]'::jsonb, false, false, false, true, 0, 6, false, false, 'registry', 'library/alpine', 'latest', '', 15, 1048576),
  ('ghcr', 'GHCR', 'https://ghcr.io', 'https://ghcr.io', 'ghcr.io', '', '', '', '', '', '["Azure"]'::jsonb, true, false, false, true, 0, 0, false, false, 'registry', 'library/alpine', 'latest', '', 15, 1048576),
  ('ghcr', 'ghcr.1ms.run (Free)', 'https://ghcr.1ms.run', 'https://ghcr.1ms.run', 'ghcr.1ms.run', '', '', '', '', '', '["CloudFlare"]'::jsonb, false, true, false, true, 0, 1, false, false, 'registry', 'library/alpine', 'latest', '', 15, 1048576),
  ('ghcr', 'ghcr.nju.edu.cn', 'https://ghcr.nju.edu.cn', 'https://ghcr.nju.edu.cn', 'ghcr.nju.edu.cn', '', '', '', '', '', '["mirror"]'::jsonb, false, false, false, true, 0, 3, false, false, 'registry', 'library/alpine', 'latest', '', 15, 1048576),
  ('ghcr', 'ghcr.sparkcr.cn (Free)', 'https://ghcr.sparkcr.cn', 'https://ghcr.sparkcr.cn', 'ghcr.sparkcr.cn', '', '', '', '', '', '["ESA"]'::jsonb, false, false, false, true, 0, 4, false, false, 'registry', 'library/alpine', 'latest', '', 15, 1048576),
  ('ghcr', 'DockerProxy', 'https://ghcr.dockerproxy.net', 'https://ghcr.dockerproxy.net', 'ghcr.dockerproxy.net', '', '', '', '', '', '["DockerProxy","Oracle CDN"]'::jsonb, false, false, false, true, 0, 6, false, false, 'registry', 'library/alpine', 'latest', '', 15, 1048576),
  ('ghcr', 'DaoCloud', 'https://ghcr.m.daocloud.io', 'https://ghcr.m.daocloud.io', 'ghcr.m.daocloud.io', '', '', '', '', '', '["DaoCloud"]'::jsonb, false, false, false, true, 0, 7, false, false, 'registry', 'library/alpine', 'latest', '', 15, 1048576),
  ('k8s', 'K8s Registry', 'https://registry.k8s.io', 'https://registry.k8s.io', 'registry.k8s.io', '', '', '', '', '', '["Google Cloud"]'::jsonb, true, false, false, true, 0, 0, false, false, 'registry', 'library/alpine', 'latest', '', 15, 1048576),
  ('k8s', 'k8s.nju.edu.cn', 'https://k8s.nju.edu.cn', 'https://k8s.nju.edu.cn', 'k8s.nju.edu.cn', '', '', '', '', '', '["mirror"]'::jsonb, false, false, false, true, 0, 1, false, false, 'registry', 'library/alpine', 'latest', '', 15, 1048576),
  ('k8s', 'k8s.sparkcr.cn (Free)', 'https://k8s.sparkcr.cn', 'https://k8s.sparkcr.cn', 'k8s.sparkcr.cn', '', '', '', '', '', '["ESA"]'::jsonb, false, false, false, true, 0, 2, false, false, 'registry', 'library/alpine', 'latest', '', 15, 1048576),
  ('k8s', 'k8s.1ms.run (Paid)', 'https://k8s.1ms.run', 'https://k8s.1ms.run', 'k8s.1ms.run', '', '', '', '', '', '["CDN"]'::jsonb, false, false, false, true, 0, 3, false, false, 'registry', 'library/alpine', 'latest', '', 15, 2097152),
  ('k8s', 'DockerProxy', 'https://k8s.dockerproxy.net', 'https://k8s.dockerproxy.net', 'k8s.dockerproxy.net', '', '', '', '', '', '["DockerProxy","Oracle CDN"]'::jsonb, false, false, false, true, 0, 5, false, false, 'registry', 'library/alpine', 'latest', '', 15, 1048576),
  ('k8s', 'DaoCloud', 'https://k8s.m.daocloud.io', 'https://k8s.m.daocloud.io', 'k8s.m.daocloud.io', '', '', '', '', '', '["DaoCloud"]'::jsonb, false, false, false, true, 0, 6, false, false, 'registry', 'library/alpine', 'latest', '', 15, 1048576),
  ('mcr', 'MCR', 'https://mcr.microsoft.com', 'https://mcr.microsoft.com', 'mcr.microsoft.com', '', '', '', '', '', '["Azure"]'::jsonb, true, false, false, true, 0, 0, false, false, 'registry', 'library/alpine', 'latest', '', 15, 1048576),
  ('mcr', 'mcr.sparkcr.cn (Free)', 'https://mcr.sparkcr.cn', 'https://mcr.sparkcr.cn', 'mcr.sparkcr.cn', '', '', '', '', '', '["ESA"]'::jsonb, false, false, false, true, 0, 1, false, false, 'registry', 'library/alpine', 'latest', '', 15, 1048576),
  ('mcr', 'mcr.1ms.run (Paid)', 'https://mcr.1ms.run', 'https://mcr.1ms.run', 'mcr.1ms.run', '', '', '', '', '', '["CDN"]'::jsonb, false, false, false, true, 0, 2, false, false, 'registry', 'library/alpine', 'latest', '', 15, 2097152),
  ('mcr', 'DockerProxy', 'https://mcr.dockerproxy.net', 'https://mcr.dockerproxy.net', 'mcr.dockerproxy.net', '', '', '', '', '', '["DockerProxy","Oracle CDN"]'::jsonb, false, false, false, true, 0, 4, false, false, 'registry', 'library/alpine', 'latest', '', 15, 1048576),
  ('mcr', 'DaoCloud', 'https://mcr.m.daocloud.io', 'https://mcr.m.daocloud.io', 'mcr.m.daocloud.io', '', '', '', '', '', '["DaoCloud"]'::jsonb, false, false, false, true, 0, 5, false, false, 'registry', 'library/alpine', 'latest', '', 15, 1048576),
  ('nvcr', 'NVCR', 'https://nvcr.io', 'https://nvcr.io', 'nvcr.io', '', '', '', '', '', '["CloudFront"]'::jsonb, true, false, false, true, 0, 0, false, false, 'registry', 'library/alpine', 'latest', '', 15, 1048576),
  ('nvcr', 'ngc.nju.edu.cn', 'https://ngc.nju.edu.cn', 'https://ngc.nju.edu.cn', 'ngc.nju.edu.cn', '', '', '', '', '', '["mirror"]'::jsonb, false, false, false, true, 0, 1, false, false, 'registry', 'library/alpine', 'latest', '', 15, 1048576),
  ('nvcr', 'nvcr.sparkcr.cn (Free)', 'https://nvcr.sparkcr.cn', 'https://nvcr.sparkcr.cn', 'nvcr.sparkcr.cn', '', '', '', '', '', '["ESA"]'::jsonb, false, false, false, true, 0, 2, false, false, 'registry', 'library/alpine', 'latest', '', 15, 1048576),
  ('nvcr', 'nvcr.1ms.run (Paid)', 'https://nvcr.1ms.run', 'https://nvcr.1ms.run', 'nvcr.1ms.run', '', '', '', '', '', '["CDN"]'::jsonb, false, false, false, true, 0, 3, false, false, 'registry', 'library/alpine', 'latest', '', 15, 2097152),
  ('nvcr', 'DaoCloud', 'https://nvcr.m.daocloud.io', 'https://nvcr.m.daocloud.io', 'nvcr.m.daocloud.io', '', '', '', '', '', '["DaoCloud"]'::jsonb, false, false, false, true, 0, 5, false, false, 'registry', 'library/alpine', 'latest', '', 15, 1048576),
  ('quay', 'Quay', 'https://quay.io', 'https://quay.io', 'quay.io', '', '', '', '', '', '["CloudFront"]'::jsonb, true, false, false, true, 0, 0, false, false, 'registry', 'library/alpine', 'latest', '', 15, 1048576),
  ('quay', 'quay.nju.edu.cn', 'https://quay.nju.edu.cn', 'https://quay.nju.edu.cn', 'quay.nju.edu.cn', '', '', '', '', '', '["mirror"]'::jsonb, false, false, false, true, 0, 1, false, false, 'registry', 'library/alpine', 'latest', '', 15, 1048576),
  ('quay', 'quay.sparkcr.cn (Free)', 'https://quay.sparkcr.cn', 'https://quay.sparkcr.cn', 'quay.sparkcr.cn', '', '', '', '', '', '["ESA"]'::jsonb, false, false, false, true, 0, 2, false, false, 'registry', 'library/alpine', 'latest', '', 15, 1048576),
  ('quay', 'quay.1ms.run (Paid)', 'https://quay.1ms.run', 'https://quay.1ms.run', 'quay.1ms.run', '', '', '', '', '', '["CDN"]'::jsonb, false, false, false, true, 0, 3, false, false, 'registry', 'library/alpine', 'latest', '', 15, 2097152),
  ('quay', 'DockerProxy', 'https://quay.dockerproxy.net', 'https://quay.dockerproxy.net', 'quay.dockerproxy.net', '', '', '', '', '', '["DockerProxy","Oracle CDN"]'::jsonb, false, false, false, true, 0, 5, false, false, 'registry', 'library/alpine', 'latest', '', 15, 1048576),
  ('quay', 'DaoCloud', 'https://quay.m.daocloud.io', 'https://quay.m.daocloud.io', 'quay.m.daocloud.io', '', '', '', '', '', '["DaoCloud"]'::jsonb, false, false, false, true, 0, 6, false, false, 'registry', 'library/alpine', 'latest', '', 15, 1048576)
)
INSERT INTO registry_sources (
  category_id, name, base_url, display_url, registry_host, description, provider,
  country, region, operator, tags, is_official, is_cloudflare, is_recommended,
  is_enabled, priority, sort_order, maintenance, probe_config_custom, probe_mode,
  test_repository, test_tag, test_digest, request_timeout_seconds, download_test_bytes
)
SELECT
  s.category_id, s.name, s.base_url, s.display_url, s.registry_host, s.description, s.provider,
  s.country, s.region, s.operator, s.tags, s.is_official, s.is_cloudflare, s.is_recommended,
  s.is_enabled, s.priority, s.sort_order, s.maintenance, s.probe_config_custom, s.probe_mode,
  s.test_repository, s.test_tag, s.test_digest, s.request_timeout_seconds, s.download_test_bytes
FROM seed s
WHERE NOT EXISTS (
  SELECT 1 FROM registry_sources existing
  WHERE existing.category_id = s.category_id
    AND existing.name = s.name
    AND existing.base_url = s.base_url
);
