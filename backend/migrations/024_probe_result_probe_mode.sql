ALTER TABLE probe_results
    ADD COLUMN IF NOT EXISTS probe_mode text NOT NULL DEFAULT 'unknown';

CREATE INDEX IF NOT EXISTS probe_results_probe_mode_checked_idx
    ON probe_results(probe_mode, checked_at DESC);
