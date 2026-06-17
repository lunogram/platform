-- Admin ↔ organization many-to-many membership. Until now every admin belonged
-- to exactly one organization via admins.organization_id; this table lets an
-- admin be a member of several organizations (e.g. after accepting a project
-- invite into another org). admins.organization_id is kept as the admin's home
-- organization for backward compatibility and is dual-written for now.
CREATE TABLE organization_members (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    admin_id UUID NOT NULL REFERENCES admins(id) ON DELETE CASCADE,
    role VARCHAR(64) NOT NULL DEFAULT 'member',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    UNIQUE (organization_id, admin_id)
);

CREATE INDEX organization_members_organization_id_idx ON organization_members(organization_id);
CREATE INDEX organization_members_admin_id_idx ON organization_members(admin_id);
CREATE UNIQUE INDEX organization_members_org_admin_uniq ON organization_members(organization_id, admin_id) WHERE deleted_at IS NULL;

CREATE TRIGGER set_updated_at_organization_members BEFORE UPDATE ON organization_members FOR EACH ROW EXECUTE PROCEDURE set_updated_at();

-- Every existing admin becomes a member of their current home organization,
-- preserving their global role as the membership role.
INSERT INTO organization_members (organization_id, admin_id, role)
SELECT organization_id, id, role FROM admins WHERE deleted_at IS NULL;

-- The active organization scopes an admin's session. It defaults to the home
-- organization and is changed via the organization switcher.
ALTER TABLE admins ADD COLUMN active_organization_id UUID REFERENCES organizations(id) ON DELETE SET NULL;
UPDATE admins SET active_organization_id = organization_id;
