-- The probe worker can be paused from the admin system settings without
-- stopping the API, frontend, or registry proxy services.
INSERT INTO system_settings(key, value)
VALUES ('probe_service_enabled', 'true'::jsonb)
ON CONFLICT (key) DO NOTHING;
