-- Complete the diagnostic fields required to explain each probe stage.
ALTER TABLE probe_results ADD COLUMN IF NOT EXISTS certificate_not_before timestamptz;
ALTER TABLE probe_results ADD COLUMN IF NOT EXISTS certificate_not_after timestamptz;
ALTER TABLE probe_results ADD COLUMN IF NOT EXISTS registry_api_version text NOT NULL DEFAULT '';
ALTER TABLE probe_results ADD COLUMN IF NOT EXISTS manifest_size bigint NOT NULL DEFAULT 0;
ALTER TABLE probe_results ADD COLUMN IF NOT EXISTS blob_range_supported boolean NOT NULL DEFAULT false;
ALTER TABLE probe_results ADD COLUMN IF NOT EXISTS dns_error text NOT NULL DEFAULT '';
ALTER TABLE probe_results ADD COLUMN IF NOT EXISTS tcp_error text NOT NULL DEFAULT '';
ALTER TABLE probe_results ADD COLUMN IF NOT EXISTS tls_error text NOT NULL DEFAULT '';
ALTER TABLE probe_results ADD COLUMN IF NOT EXISTS registry_api_error text NOT NULL DEFAULT '';
ALTER TABLE probe_results ADD COLUMN IF NOT EXISTS manifest_error text NOT NULL DEFAULT '';
ALTER TABLE probe_results ADD COLUMN IF NOT EXISTS blob_error text NOT NULL DEFAULT '';
