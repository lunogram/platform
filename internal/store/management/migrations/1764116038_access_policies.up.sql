-- Migration: Generalize project_api_keys into access_policies
-- Purpose: An access policy is a project-scoped configuration of *how* a client
-- authenticates to the API. The API key is the first policy type; trusted-issuer
-- (JWKS / public cert) and short-term session policies are layered on top via the
-- type discriminator and the *_config columns below.
--
-- The original project_api_keys table is renamed (so its data, indexes and
-- triggers are preserved) and re-exposed as an auto-updatable view, letting the
-- existing API-key store and auth lookups keep working unchanged during the
-- transition.

ALTER TABLE project_api_keys RENAME TO access_policies;

-- Policy type discriminator. Every existing row is an API key.
ALTER TABLE access_policies
    ADD COLUMN type VARCHAR(32) NOT NULL DEFAULT 'api_key';

-- The secret (value) only applies to API-key and session policies; trusted-issuer
-- policies have no Lunogram-held secret.
ALTER TABLE access_policies
    ALTER COLUMN value DROP NOT NULL;

-- Custom permission set: a JSON array of {"resource","verb"} grants. NULL means
-- the policy uses its role preset (support/client/editor/admin) rather than a
-- custom scope.
ALTER TABLE access_policies
    ADD COLUMN grants JSONB;

-- Trusted-issuer configuration (JWKS url / PEM cert, iss, aud, subject claim),
-- populated for type = 'trusted_issuer'.
ALTER TABLE access_policies
    ADD COLUMN issuer_config JSONB;

-- Session-signing configuration (TTL, granted role/scope), populated for
-- type = 'session'.
ALTER TABLE access_policies
    ADD COLUMN session_config JSONB;

-- Backward-compatible view over the API-key rows. Postgres keeps this view
-- auto-updatable (single source table, plain columns, type defaults to
-- 'api_key' so inserts land as API keys), so existing project_api_keys
-- queries — including INSERT ... RETURNING and soft-delete UPDATEs — keep
-- working without code changes.
CREATE VIEW project_api_keys AS
    SELECT id, project_id, value, scope, name, description, role,
           created_at, updated_at, deleted_at
    FROM access_policies
    WHERE type = 'api_key';
