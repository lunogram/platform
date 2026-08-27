-- Every console credential now terminates in a row here: the session is the
-- revocation handle the old stateless admin JWT never had. The token carries
-- only its id (sid); everything that could go stale -- role, organization,
-- email -- stays out of the bearer credential and is re-read per request.
--
-- There is deliberately no organization_id: the active organization lives on
-- admins and is re-validated against current membership on every request.
-- Copying it here would create the second source of truth this refactor exists
-- to remove.
--
-- There is also no deleted_at. A session is an event, not an entity: it ends by
-- expiring or by being revoked, and both are already modelled. A third liveness
-- predicate on the hottest lookup in the system is not worth it; retention is a
-- purge job.
CREATE TABLE admin_sessions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    admin_id UUID NOT NULL REFERENCES admins(id) ON DELETE CASCADE,
    admin_identity_id UUID REFERENCES admin_identities(id) ON DELETE SET NULL,
    impersonated BOOLEAN NOT NULL DEFAULT FALSE,
    -- Nullable because an upstream impersonator (e.g. a Clerk dashboard user)
    -- usually maps to no admin row of ours. The raw subject is always recorded;
    -- the admin id only when it resolves.
    impersonator_admin_id UUID REFERENCES admins(id) ON DELETE SET NULL,
    impersonator_subject VARCHAR(255),
    upstream_expires_at TIMESTAMPTZ,
    issued_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    absolute_expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    refreshable BOOLEAN NOT NULL DEFAULT TRUE,
    user_agent VARCHAR(512),
    ip INET,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (expires_at <= absolute_expires_at),
    -- The impersonation invariants are database CHECKs on purpose: a future code
    -- path that forgets to clamp the lifetime, or to mark the session
    -- non-refreshable, cannot write the row at all. An impersonated session may
    -- never outlive the upstream session that authorised it.
    CHECK (NOT impersonated OR (
        impersonator_subject IS NOT NULL
        AND refreshable = FALSE
        AND upstream_expires_at IS NOT NULL
        AND absolute_expires_at <= upstream_expires_at
    )),
    CHECK (impersonated OR (impersonator_admin_id IS NULL AND impersonator_subject IS NULL)),
    CHECK (impersonator_admin_id IS NULL OR impersonator_admin_id <> admin_id)
);

CREATE INDEX admin_sessions_admin_id_idx ON admin_sessions (admin_id);
CREATE INDEX admin_sessions_active_idx ON admin_sessions (expires_at) WHERE revoked_at IS NULL;

CREATE TRIGGER set_updated_at_admin_sessions BEFORE UPDATE ON admin_sessions
    FOR EACH ROW EXECUTE PROCEDURE set_updated_at();
