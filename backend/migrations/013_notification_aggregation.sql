-- Extend notification rules and logs without changing the 007/008 contracts.
-- Zero keeps the existing cooldown-only behavior; positive values enable
-- channel/event coalescing within the requested window.
ALTER TABLE notification_rules
    ADD COLUMN IF NOT EXISTS aggregation_seconds integer NOT NULL DEFAULT 0;

ALTER TABLE notification_logs
    ADD COLUMN IF NOT EXISTS source_id uuid REFERENCES registry_sources(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS notification_logs_channel_event_window_idx
    ON notification_logs(channel_id, event_type, created_at DESC)
    WHERE status IN ('sent', 'coalesced');
