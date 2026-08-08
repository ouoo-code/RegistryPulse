CREATE TABLE IF NOT EXISTS notification_rules (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    channel_id uuid NOT NULL REFERENCES notification_channels(id) ON DELETE CASCADE,
    event_type text NOT NULL,
    enabled boolean NOT NULL DEFAULT true,
    cooldown_seconds integer NOT NULL DEFAULT 300,
    template text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(channel_id, event_type)
);
CREATE TABLE IF NOT EXISTS notification_logs (
    id bigserial PRIMARY KEY,
    channel_id uuid REFERENCES notification_channels(id) ON DELETE SET NULL,
    event_type text NOT NULL,
    status text NOT NULL,
    attempts integer NOT NULL DEFAULT 1,
    error text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS notification_logs_created_idx ON notification_logs(created_at DESC);
