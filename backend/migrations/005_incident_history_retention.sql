-- Incident history persistence and retention indexes.
ALTER TABLE incident_events ADD COLUMN IF NOT EXISTS source_id uuid REFERENCES registry_sources(id) ON DELETE CASCADE;
ALTER TABLE incident_events ADD COLUMN IF NOT EXISTS from_status text NOT NULL DEFAULT '';
ALTER TABLE incident_events ADD COLUMN IF NOT EXISTS to_status text NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS incident_events_incident_created_idx
    ON incident_events(incident_id, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS incident_events_source_created_idx
    ON incident_events(source_id, created_at DESC);
CREATE INDEX IF NOT EXISTS incidents_resolved_at_idx
    ON incidents(resolved_at)
    WHERE resolved_at IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS incidents_one_active_per_source_idx
    ON incidents(source_id)
    WHERE resolved_at IS NULL;

ALTER TABLE probe_results ADD COLUMN IF NOT EXISTS blob_duration_ms bigint NOT NULL DEFAULT 0;
ALTER TABLE probe_results ADD COLUMN IF NOT EXISTS blob_bytes bigint NOT NULL DEFAULT 0;
CREATE INDEX IF NOT EXISTS probe_results_checked_at_idx ON probe_results(checked_at);

-- Named permissions keep authorization auditable and allow roles to be
-- extended without changing HTTP handlers.
INSERT INTO permissions(name) VALUES
('source.read'),('source.write'),('probe.read'),('probe.write'),
('incident.read'),('settings.read'),('settings.write'),('audit.read'),
('agent.manage') ON CONFLICT(name) DO NOTHING;
INSERT INTO roles(name) VALUES('operator'),('viewer') ON CONFLICT(name) DO NOTHING;
INSERT INTO role_permissions(role_id, permission_id)
SELECT r.id, p.id FROM roles r CROSS JOIN permissions p
WHERE r.name='operator' AND p.name IN ('source.read','source.write','probe.read','probe.write','incident.read','settings.read')
ON CONFLICT DO NOTHING;
INSERT INTO role_permissions(role_id, permission_id)
SELECT r.id, p.id FROM roles r CROSS JOIN permissions p
WHERE r.name='viewer' AND p.name IN ('source.read','probe.read','incident.read','settings.read')
ON CONFLICT DO NOTHING;
