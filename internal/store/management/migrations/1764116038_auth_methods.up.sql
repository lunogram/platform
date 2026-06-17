-- Migration: normalized authentication model (auth_methods + per-type tables)
--
-- Replaces the single project_api_keys table with a supertype/subtype model that
-- separates authentication (how a client proves identity) from authorization
-- (what it may do):
--
--   auth_methods                 supertype: a configured way to authenticate;
--                                its id is the RBAC subject. Carries the authz
--                                role preset.
--   auth_method_grants           authorization: custom permission set (resource, verb)
--   auth_method_api_keys         credential: a Lunogram-issued key (secret hashed)
--   auth_method_trusted_issuers  credential validation: external JWT (JWKS/PEM)
--   auth_method_sessions         credential minting: short-lived session signing
--
-- Existing API keys are migrated into auth_methods + auth_method_api_keys, with
-- their plaintext hashed in the process (the plaintext is then dropped).

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE auth_methods (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    type VARCHAR(32) NOT NULL,
    name VARCHAR(255) NOT NULL,
    description VARCHAR(2048),
    role VARCHAR(64) NOT NULL DEFAULT 'support',
    -- Data boundary: 'all' acts across every subject's records (backend keys),
    -- 'own' confines a verified end-user to their own records. Only meaningful
    -- for verified-subject types (trusted_issuer, session); api_key is 'all'.
    subject_scope VARCHAR(8) NOT NULL DEFAULT 'all',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);
CREATE INDEX auth_methods_project_id_idx ON auth_methods(project_id);
CREATE TRIGGER set_updated_at_auth_methods BEFORE UPDATE ON auth_methods
    FOR EACH ROW EXECUTE PROCEDURE set_updated_at();

-- Custom permission set. A method either uses its role preset or these grants.
CREATE TABLE auth_method_grants (
    auth_method_id UUID NOT NULL REFERENCES auth_methods(id) ON DELETE CASCADE,
    resource VARCHAR(64) NOT NULL,
    verb VARCHAR(16) NOT NULL,
    PRIMARY KEY (auth_method_id, resource, verb)
);

-- API key credential. The plaintext secret is never stored: only its SHA-256
-- hash (the lookup key) and a short display prefix.
CREATE TABLE auth_method_api_keys (
    auth_method_id UUID PRIMARY KEY REFERENCES auth_methods(id) ON DELETE CASCADE,
    secret_hash TEXT NOT NULL,
    secret_prefix TEXT NOT NULL,
    scope VARCHAR(20) NOT NULL DEFAULT 'secret'
);
CREATE UNIQUE INDEX auth_method_api_keys_secret_hash_uniq ON auth_method_api_keys(secret_hash);

-- Trusted-issuer (BYO-IdP) validation config. Exactly one of jwks_url /
-- public_cert is set.
CREATE TABLE auth_method_trusted_issuers (
    auth_method_id UUID PRIMARY KEY REFERENCES auth_methods(id) ON DELETE CASCADE,
    jwks_url TEXT,
    public_cert TEXT,
    issuer TEXT NOT NULL,
    audience TEXT,
    subject_claim VARCHAR(64) NOT NULL DEFAULT 'sub',
    CHECK ((jwks_url IS NOT NULL) <> (public_cert IS NOT NULL))
);
CREATE INDEX auth_method_trusted_issuers_issuer_idx ON auth_method_trusted_issuers(issuer);

-- Short-lived session config. Tokens are signed with a server key (configured,
-- not per-policy), so only the minted-token lifetime lives here; the sessions
-- PR adds anything more it needs.
CREATE TABLE auth_method_sessions (
    auth_method_id UUID PRIMARY KEY REFERENCES auth_methods(id) ON DELETE CASCADE,
    ttl_seconds INTEGER NOT NULL DEFAULT 900
);

-- Migrate existing API keys into the normalized model, hashing the plaintext.
INSERT INTO auth_methods (id, project_id, type, name, description, role, created_at, updated_at, deleted_at)
SELECT id, project_id, 'api_key', name, description, role, created_at, updated_at, deleted_at
FROM project_api_keys;

INSERT INTO auth_method_api_keys (auth_method_id, secret_hash, secret_prefix, scope)
SELECT id, encode(digest(value, 'sha256'), 'hex'), left(value, 11), COALESCE(scope, 'secret')
FROM project_api_keys;

DROP TABLE project_api_keys;
