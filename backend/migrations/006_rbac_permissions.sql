-- Idempotent RBAC seed for databases that already applied migration 005.
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
