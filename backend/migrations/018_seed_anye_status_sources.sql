-- Default registry catalog fetched from https://status.anye.xyz/ on 2026-08-07.
-- Remote status is intentionally not copied: the local worker must probe every source.
-- Display names are ASCII-safe host-based fallbacks where the remote label is non-ASCII.
-- The NOT EXISTS guard keeps existing administrator-created records untouched.
WITH seed(category_id, name, base_url, registry_host, tags, is_official, is_cloudflare, is_recommended, sort_order) AS (
VALUES
  ('dockerhub', 'Docker Hub', 'https://registry-1.docker.io', 'registry-1.docker.io', '["CloudFront"]'::jsonb, true, false, false, 0),
  ('dockerhub', 'mirror.ccs.tencentyun.com', 'https://mirror.ccs.tencentyun.com', 'mirror.ccs.tencentyun.com', '["mirror"]'::jsonb, false, false, false, 1),
  ('dockerhub', 'docker.1ms.run (Free)', 'https://docker.1ms.run', 'docker.1ms.run', '["CloudFlare"]'::jsonb, false, true, true, 2),
  ('dockerhub', 'docker.1ms.run (Paid)', 'https://docker.1ms.run', 'docker.1ms.run', '["CDN"]'::jsonb, false, false, false, 3),
  ('dockerhub', '1Panel', 'https://docker.1panel.live', 'docker.1panel.live', '["1Panel","CloudFlare"]'::jsonb, false, true, false, 4),
  ('dockerhub', 'docker.sparkcr.cn (Free)', 'https://docker.sparkcr.cn', 'docker.sparkcr.cn', '["ESA"]'::jsonb, false, false, true, 5),
  ('dockerhub', 'hub.rat.dev', 'https://hub.rat.dev', 'hub.rat.dev', '["CloudFlare"]'::jsonb, false, true, true, 6),
  ('dockerhub', 'docker.xuanyuan.me (Free)', 'https://docker.xuanyuan.me', 'docker.xuanyuan.me', '["CloudFlare"]'::jsonb, false, true, false, 7),
  ('dockerhub', 'docker.xuanyuan.run (Pro)', 'https://docker.xuanyuan.run', 'docker.xuanyuan.run', '["mirror"]'::jsonb, false, false, false, 8),
  ('dockerhub', 'docker.xuanyuan.dev (Pro)', 'https://docker.xuanyuan.dev', 'docker.xuanyuan.dev', '["CloudFlare"]'::jsonb, false, true, false, 9),
  ('dockerhub', 'DockerProxy', 'https://dockerproxy.net', 'dockerproxy.net', '["DockerProxy","Oracle CDN"]'::jsonb, false, false, true, 10),
  ('dockerhub', 'docker-registry.nmqu.com', 'https://docker-registry.nmqu.com', 'docker-registry.nmqu.com', '["CloudFlare"]'::jsonb, false, true, true, 11),
  ('dockerhub', 'hub.amingg.com 1', 'https://hub.amingg.com', 'hub.amingg.com', '["CloudFlare"]'::jsonb, false, true, false, 12),
  ('dockerhub', 'docker.amingg.com 2', 'https://docker.amingg.com', 'docker.amingg.com', '["CloudFlare"]'::jsonb, false, true, false, 13),
  ('dockerhub', 'docker.hlmirror.com', 'https://docker.hlmirror.com', 'docker.hlmirror.com', '["CloudFlare"]'::jsonb, false, true, false, 14),
  ('dockerhub', 'hub1.nat.tf 1', 'https://hub1.nat.tf', 'hub1.nat.tf', '["Nginx"]'::jsonb, false, false, false, 15),
  ('dockerhub', 'hub2.nat.tf 2', 'https://hub2.nat.tf', 'hub2.nat.tf', '["Nginx"]'::jsonb, false, false, false, 16),
  ('dockerhub', 'hub3.nat.tf', 'https://hub3.nat.tf', 'hub3.nat.tf', '["Nginx"]'::jsonb, false, false, false, 17),
  ('dockerhub', 'hub4.nat.tf', 'https://hub4.nat.tf', 'hub4.nat.tf', '["Nginx"]'::jsonb, false, false, false, 18),
  ('dockerhub', 'DaoCloud', 'https://docker.m.daocloud.io', 'docker.m.daocloud.io', '["DaoCloud"]'::jsonb, false, false, false, 19),
  ('dockerhub', 'docker.kejilion.pro', 'https://docker.kejilion.pro', 'docker.kejilion.pro', '["Nginx"]'::jsonb, false, false, false, 20),
  ('dockerhub', 'docker.367231.xyz 1', 'https://docker.367231.xyz', 'docker.367231.xyz', '["CloudFlare"]'::jsonb, false, true, false, 21),
  ('dockerhub', 'hub.1panel.dev 1', 'https://hub.1panel.dev', 'hub.1panel.dev', '["CloudFlare"]'::jsonb, false, true, false, 22),
  ('dockerhub', 'SUNBALCONY 1', 'https://dockerproxy.cool', 'dockerproxy.cool', '["EdgeOne"]'::jsonb, false, false, false, 23),
  ('dockerhub', 'docker.fnnas.com', 'https://docker.fnnas.com', 'docker.fnnas.com', '["Nginx"]'::jsonb, false, false, false, 24),
  ('ghcr', 'GHCR', 'https://ghcr.io', 'ghcr.io', '["Azure"]'::jsonb, true, false, false, 0),
  ('ghcr', 'ghcr.1ms.run (Free)', 'https://ghcr.1ms.run', 'ghcr.1ms.run', '["CloudFlare"]'::jsonb, false, true, true, 1),
  ('ghcr', 'ghcr.1ms.run (Paid)', 'https://ghcr.1ms.run', 'ghcr.1ms.run', '["CDN"]'::jsonb, false, false, false, 2),
  ('ghcr', 'ghcr.nju.edu.cn', 'https://ghcr.nju.edu.cn', 'ghcr.nju.edu.cn', '["mirror"]'::jsonb, false, false, true, 3),
  ('ghcr', 'ghcr.sparkcr.cn (Free)', 'https://ghcr.sparkcr.cn', 'ghcr.sparkcr.cn', '["ESA"]'::jsonb, false, false, true, 4),
  ('ghcr', 'xxx-ghcr.xuanyuan.run (Pro)', 'https://xxx-ghcr.xuanyuan.run', 'xxx-ghcr.xuanyuan.run', '["mirror"]'::jsonb, false, false, false, 5),
  ('ghcr', 'DockerProxy', 'https://ghcr.dockerproxy.net', 'ghcr.dockerproxy.net', '["DockerProxy","Oracle CDN"]'::jsonb, false, false, true, 6),
  ('ghcr', 'DaoCloud', 'https://ghcr.m.daocloud.io', 'ghcr.m.daocloud.io', '["DaoCloud"]'::jsonb, false, false, true, 7),
  ('quay', 'Quay', 'https://quay.io', 'quay.io', '["CloudFront"]'::jsonb, true, false, false, 0),
  ('quay', 'quay.nju.edu.cn', 'https://quay.nju.edu.cn', 'quay.nju.edu.cn', '["mirror"]'::jsonb, false, false, true, 1),
  ('quay', 'quay.sparkcr.cn (Free)', 'https://quay.sparkcr.cn', 'quay.sparkcr.cn', '["ESA"]'::jsonb, false, false, true, 2),
  ('quay', 'quay.1ms.run (Paid)', 'https://quay.1ms.run', 'quay.1ms.run', '["CDN"]'::jsonb, false, false, false, 3),
  ('quay', 'xxx-quay.xuanyuan.run (Pro)', 'https://xxx-quay.xuanyuan.run', 'xxx-quay.xuanyuan.run', '["mirror"]'::jsonb, false, false, false, 4),
  ('quay', 'DockerProxy', 'https://quay.dockerproxy.net', 'quay.dockerproxy.net', '["DockerProxy","Oracle CDN"]'::jsonb, false, false, true, 5),
  ('quay', 'DaoCloud', 'https://quay.m.daocloud.io', 'quay.m.daocloud.io', '["DaoCloud"]'::jsonb, false, false, true, 6),
  ('mcr', 'MCR', 'https://mcr.microsoft.com', 'mcr.microsoft.com', '["Azure"]'::jsonb, true, false, false, 0),
  ('mcr', 'mcr.sparkcr.cn (Free)', 'https://mcr.sparkcr.cn', 'mcr.sparkcr.cn', '["ESA"]'::jsonb, false, false, true, 1),
  ('mcr', 'mcr.1ms.run (Paid)', 'https://mcr.1ms.run', 'mcr.1ms.run', '["CDN"]'::jsonb, false, false, false, 2),
  ('mcr', 'xxx-mcr.xuanyuan.run (Pro)', 'https://xxx-mcr.xuanyuan.run', 'xxx-mcr.xuanyuan.run', '["mirror"]'::jsonb, false, false, false, 3),
  ('mcr', 'DockerProxy', 'https://mcr.dockerproxy.net', 'mcr.dockerproxy.net', '["DockerProxy","Oracle CDN"]'::jsonb, false, false, true, 4),
  ('mcr', 'DaoCloud', 'https://mcr.m.daocloud.io', 'mcr.m.daocloud.io', '["DaoCloud"]'::jsonb, false, false, true, 5),
  ('k8s', 'K8s Registry', 'https://registry.k8s.io', 'registry.k8s.io', '["Google Cloud"]'::jsonb, true, false, false, 0),
  ('k8s', 'k8s.nju.edu.cn', 'https://k8s.nju.edu.cn', 'k8s.nju.edu.cn', '["mirror"]'::jsonb, false, false, true, 1),
  ('k8s', 'k8s.sparkcr.cn (Free)', 'https://k8s.sparkcr.cn', 'k8s.sparkcr.cn', '["ESA"]'::jsonb, false, false, true, 2),
  ('k8s', 'k8s.1ms.run (Paid)', 'https://k8s.1ms.run', 'k8s.1ms.run', '["CDN"]'::jsonb, false, false, false, 3),
  ('k8s', 'xxx-k8s.xuanyuan.run (Pro)', 'https://xxx-k8s.xuanyuan.run', 'xxx-k8s.xuanyuan.run', '["mirror"]'::jsonb, false, false, false, 4),
  ('k8s', 'DockerProxy', 'https://k8s.dockerproxy.net', 'k8s.dockerproxy.net', '["DockerProxy","Oracle CDN"]'::jsonb, false, false, true, 5),
  ('k8s', 'DaoCloud', 'https://k8s.m.daocloud.io', 'k8s.m.daocloud.io', '["DaoCloud"]'::jsonb, false, false, true, 6),
  ('gcr', 'GCR', 'https://gcr.io', 'gcr.io', '["Google Cloud"]'::jsonb, true, false, false, 0),
  ('gcr', 'gcr.nju.edu.cn', 'https://gcr.nju.edu.cn', 'gcr.nju.edu.cn', '["mirror"]'::jsonb, false, false, true, 1),
  ('gcr', 'DaoCloud', 'https://gcr.m.daocloud.io', 'gcr.m.daocloud.io', '["DaoCloud"]'::jsonb, false, false, true, 2),
  ('gcr', 'gcr.sparkcr.cn (Free)', 'https://gcr.sparkcr.cn', 'gcr.sparkcr.cn', '["ESA"]'::jsonb, false, false, true, 3),
  ('gcr', 'gcr.1ms.run (Paid)', 'https://gcr.1ms.run', 'gcr.1ms.run', '["CDN"]'::jsonb, false, false, false, 4),
  ('gcr', 'xxx-gcr.xuanyuan.run (Pro)', 'https://xxx-gcr.xuanyuan.run', 'xxx-gcr.xuanyuan.run', '["mirror"]'::jsonb, false, false, false, 5),
  ('gcr', 'DockerProxy', 'https://gcr.dockerproxy.net', 'gcr.dockerproxy.net', '["DockerProxy","Oracle CDN"]'::jsonb, false, false, true, 6),
  ('elastic', 'Elastic', 'https://docker.elastic.co', 'docker.elastic.co', '["Google Cloud"]'::jsonb, true, false, false, 0),
  ('elastic', 'elastic.sparkcr.cn (Free)', 'https://elastic.sparkcr.cn', 'elastic.sparkcr.cn', '["ESA"]'::jsonb, false, false, true, 1),
  ('elastic', 'DaoCloud', 'https://elastic.m.daocloud.io', 'elastic.m.daocloud.io', '["DaoCloud"]'::jsonb, false, false, true, 2),
  ('elastic', 'elastic.1ms.run (Paid)', 'https://elastic.1ms.run', 'elastic.1ms.run', '["CDN"]'::jsonb, false, false, false, 3),
  ('elastic', 'xxx-elastic.xuanyuan.run (Pro)', 'https://xxx-elastic.xuanyuan.run', 'xxx-elastic.xuanyuan.run', '["mirror"]'::jsonb, false, false, false, 4),
  ('nvcr', 'NVCR', 'https://nvcr.io', 'nvcr.io', '["CloudFront"]'::jsonb, true, false, false, 0),
  ('nvcr', 'ngc.nju.edu.cn', 'https://ngc.nju.edu.cn', 'ngc.nju.edu.cn', '["mirror"]'::jsonb, false, false, true, 1),
  ('nvcr', 'nvcr.sparkcr.cn (Free)', 'https://nvcr.sparkcr.cn', 'nvcr.sparkcr.cn', '["ESA"]'::jsonb, false, false, true, 2),
  ('nvcr', 'nvcr.1ms.run (Paid)', 'https://nvcr.1ms.run', 'nvcr.1ms.run', '["CDN"]'::jsonb, false, false, false, 3),
  ('nvcr', 'xxx-nvcr.xuanyuan.run (Pro)', 'https://xxx-nvcr.xuanyuan.run', 'xxx-nvcr.xuanyuan.run', '["mirror"]'::jsonb, false, false, false, 4),
  ('nvcr', 'DaoCloud', 'https://nvcr.m.daocloud.io', 'nvcr.m.daocloud.io', '["DaoCloud"]'::jsonb, false, false, true, 5)
)
INSERT INTO registry_sources (
  category_id, name, base_url, display_url, registry_host, description, provider,
  country, region, operator, tags, is_official, is_cloudflare, is_recommended,
  is_enabled, priority, sort_order, maintenance, probe_mode, test_repository,
  test_tag, test_digest, request_timeout_seconds, download_test_bytes
)
SELECT
  s.category_id, s.name, s.base_url, s.base_url, s.registry_host, '', '',
  '', '', '', s.tags, s.is_official, s.is_cloudflare, s.is_recommended,
  true, 0, s.sort_order, false, 'registry', 'library/alpine',
  'latest', '', 15, 2097152
FROM seed s
WHERE NOT EXISTS (
  SELECT 1
  FROM registry_sources existing
  WHERE existing.category_id = s.category_id
    AND existing.name = s.name
    AND existing.base_url = s.base_url
);
