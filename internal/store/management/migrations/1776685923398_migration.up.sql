CREATE TABLE project_invites (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id UUID REFERENCES projects(id) ON DELETE CASCADE,
    inviter_admin_id UUID NOT NULL REFERENCES admins(id) ON DELETE SET NULL,
    invitee_email VARCHAR(255) NOT NULL,
    token VARCHAR(50) NOT NULL,

    role VARCHAR(50) NOT NULL,
    CHECK (role IN ('support', 'client', 'editor', 'admin')),
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    accepted_at TIMESTAMPTZ
)
