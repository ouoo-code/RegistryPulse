-- Registry proxy control-plane defaults. Image content is never stored here.
INSERT INTO system_settings(key, value)
VALUES
  ('proxy_enabled', 'true'::jsonb),
  ('proxy_listen_port', '10800'::jsonb),
  ('proxy_category_id', '"dockerhub"'::jsonb),
  ('proxy_route_max_age_minutes', '120'::jsonb),
  ('proxy_failure_cooldown_seconds', '30'::jsonb),
  ('proxy_max_concurrent', '64'::jsonb),
  ('proxy_max_range_mb', '256'::jsonb),
  ('proxy_max_manifest_mb', '8'::jsonb)
ON CONFLICT (key) DO NOTHING;
