-- Organizations table
-- Stores organization/company data that can be used as event subjects
CREATE TABLE organizations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id UUID NOT NULL,
    external_id VARCHAR(255) NOT NULL,
    name VARCHAR(255),
    data JSONB NOT NULL DEFAULT '{}'::jsonb,
    version INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX organizations_project_external_uniq ON organizations(project_id, external_id);
CREATE INDEX organizations_project_id_idx ON organizations(project_id);
CREATE INDEX organizations_data_idx ON organizations USING gin(data);

CREATE TRIGGER set_updated_at_organizations BEFORE UPDATE ON organizations FOR EACH ROW EXECUTE PROCEDURE set_updated_at();
CREATE TRIGGER increment_version_organizations BEFORE UPDATE ON organizations FOR EACH ROW EXECUTE PROCEDURE increment_version();

-- Organization users junction table
-- Links users to organizations with optional org-specific user data
CREATE TABLE organization_users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    data JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX organization_users_org_user_uniq ON organization_users(organization_id, user_id);
CREATE INDEX organization_users_organization_id_idx ON organization_users(organization_id);
CREATE INDEX organization_users_user_id_idx ON organization_users(user_id);
CREATE INDEX organization_users_data_idx ON organization_users USING gin(data);

CREATE TRIGGER set_updated_at_organization_users BEFORE UPDATE ON organization_users FOR EACH ROW EXECUTE PROCEDURE set_updated_at();
