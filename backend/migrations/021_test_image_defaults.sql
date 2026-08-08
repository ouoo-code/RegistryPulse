ALTER TABLE test_images ADD COLUMN IF NOT EXISTS is_default boolean NOT NULL DEFAULT false;

UPDATE test_images SET is_default=false;
UPDATE test_images SET is_default=true
WHERE id = (SELECT id FROM test_images WHERE enabled ORDER BY reference, id LIMIT 1);

CREATE UNIQUE INDEX IF NOT EXISTS test_images_single_default_idx
ON test_images (is_default) WHERE is_default;
