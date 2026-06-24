CREATE TABLE project_invites (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    inviter_admin_id UUID REFERENCES admins(id) ON DELETE SET NULL,
    invitee_email VARCHAR(255) NOT NULL,
    invitee_admin_id UUID REFERENCES admins(id) ON DELETE SET NULL,

    role VARCHAR(50) NOT NULL,
    CHECK (role IN ('support', 'client', 'editor', 'admin')),

    expires_at TIMESTAMPTZ NOT NULL,
    accepted_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Invites are resolved by the invitee's email (case-insensitive), so index on the
-- lowered value to keep "my invites" and duplicate lookups fast.
CREATE INDEX project_invites_invitee_email_idx ON project_invites (lower(invitee_email));

-- At most one pending invite per (project, email). Accepted/revoked invites are
-- excluded so history is preserved and re-inviting after revoke/accept still works.
CREATE UNIQUE INDEX project_invites_pending_uniq
    ON project_invites (project_id, lower(invitee_email))
    WHERE accepted_at IS NULL AND revoked_at IS NULL;
