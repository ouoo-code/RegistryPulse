ALTER TABLE registry_categories ADD COLUMN IF NOT EXISTS icon text NOT NULL DEFAULT 'container';
ALTER TABLE registry_categories ADD COLUMN IF NOT EXISTS official_url text NOT NULL DEFAULT '';
ALTER TABLE registry_categories ADD COLUMN IF NOT EXISTS default_test_repository text NOT NULL DEFAULT 'library/alpine';
ALTER TABLE registry_categories ADD COLUMN IF NOT EXISTS default_manifest_path text NOT NULL DEFAULT '/v2/{repository}/manifests/{reference}';
ALTER TABLE registry_categories ADD COLUMN IF NOT EXISTS auth_type text NOT NULL DEFAULT 'bearer_or_none';
INSERT INTO registry_categories (id,slug,name,description,enabled,sort_order,icon,official_url,default_test_repository,default_manifest_path,auth_type)
VALUES ('custom','custom','自定义 OCI Registry','自定义 OCI 镜像仓库',true,90,'registry','', 'library/alpine','/v2/{repository}/manifests/{reference}','bearer_or_none')
ON CONFLICT (id) DO NOTHING;
