ALTER TABLE registry_sources ADD COLUMN IF NOT EXISTS test_image_id uuid REFERENCES test_images(id) ON DELETE SET NULL;

INSERT INTO test_images(reference, enabled, max_bytes)
VALUES ('library/alpine:latest', true, 2097152)
ON CONFLICT (reference) DO NOTHING;
