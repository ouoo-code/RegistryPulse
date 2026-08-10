-- Registry proxy traffic handling. Forward keeps bytes on this server;
-- redirect returns a 307 so the client downloads directly from the selected source.
INSERT INTO system_settings(key, value)
VALUES ('proxy_transport_mode', '"forward"'::jsonb)
ON CONFLICT (key) DO NOTHING;
