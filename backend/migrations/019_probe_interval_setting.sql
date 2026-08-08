INSERT INTO system_settings(key, value)
VALUES ('probe_interval_minutes', '{"value":30}'::jsonb)
ON CONFLICT (key) DO NOTHING;

INSERT INTO system_settings(key, value)
VALUES ('probe_retry_interval_minutes', '{"value":3}'::jsonb)
ON CONFLICT (key) DO NOTHING;
