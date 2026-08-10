-- Access-control hardening for existing installations.
-- TOTP secrets are encrypted with CREDENTIAL_ENCRYPTION_KEY by the API.
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS totp_secret_ciphertext bytea NOT NULL DEFAULT ''::bytea,
    ADD COLUMN IF NOT EXISTS totp_secret_nonce bytea NOT NULL DEFAULT ''::bytea;

-- Keep the actor name after a user is physically removed. Existing rows are
-- backfilled by the API when the user is deleted; new audit rows always write it.
ALTER TABLE audit_logs
    ADD COLUMN IF NOT EXISTS actor_username text NOT NULL DEFAULT '';
