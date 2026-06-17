-- Migration: Hash access-policy secrets at rest
-- Purpose: Stop storing plaintext API secrets. Secrets are now identified by a
-- SHA-256 hash (the lookup key) plus a short display prefix; the plaintext value
-- column is dropped. Existing secrets are backfilled from their plaintext before
-- it is removed, so existing keys keep authenticating.

CREATE EXTENSION IF NOT EXISTS pgcrypto;

ALTER TABLE access_policies ADD COLUMN secret_hash TEXT;
ALTER TABLE access_policies ADD COLUMN secret_prefix TEXT;

-- Backfill from the existing plaintext, matching management.hashSecret
-- (hex-encoded SHA-256).
UPDATE access_policies
SET secret_hash   = encode(digest(value, 'sha256'), 'hex'),
    secret_prefix = left(value, 11)
WHERE value IS NOT NULL;

-- Recreate the compatibility view without the plaintext column. Dropping the
-- column also drops the old project_api_keys_value_uniq index.
DROP VIEW IF EXISTS project_api_keys;

ALTER TABLE access_policies DROP COLUMN value;

CREATE VIEW project_api_keys AS
    SELECT id, project_id, secret_hash, secret_prefix, scope, name, description, role,
           created_at, updated_at, deleted_at
    FROM access_policies
    WHERE type = 'api_key';

CREATE UNIQUE INDEX access_policies_secret_hash_uniq
    ON access_policies(secret_hash)
    WHERE secret_hash IS NOT NULL;
