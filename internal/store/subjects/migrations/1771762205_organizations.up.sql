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

-- Organization schemas table
-- Stores schema paths extracted from organization data JSONB for autocomplete
CREATE TABLE organization_schemas (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id UUID NOT NULL,
    path VARCHAR(255) NOT NULL,
    data_type data_type NOT NULL,
    visibility project_rule_paths_visibility NOT NULL DEFAULT 'hidden',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX idx_organization_schemas_project_path ON organization_schemas(project_id, path, data_type);

CREATE TRIGGER set_updated_at_organization_schemas BEFORE UPDATE ON organization_schemas FOR EACH ROW EXECUTE PROCEDURE set_updated_at();

-- Organization user schemas table
-- Stores schema paths extracted from organization_users data JSONB for autocomplete
CREATE TABLE organization_user_schemas (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id UUID NOT NULL,
    path VARCHAR(255) NOT NULL,
    data_type data_type NOT NULL,
    visibility project_rule_paths_visibility NOT NULL DEFAULT 'hidden',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX idx_organization_user_schemas_project_path ON organization_user_schemas(project_id, path, data_type);

CREATE TRIGGER set_updated_at_organization_user_schemas BEFORE UPDATE ON organization_user_schemas FOR EACH ROW EXECUTE PROCEDURE set_updated_at();

-- Add organization dependency columns to rules table
ALTER TABLE rules ADD COLUMN depends_on_organizations BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE rules ADD COLUMN depends_on_organization_users BOOLEAN NOT NULL DEFAULT FALSE;

-- Organization events table (event occurrences for organizations)
-- Tracks events that happen to/within organizations (not users)
CREATE TABLE organization_events (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    event_id UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    data JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_organization_events_organization_id ON organization_events(organization_id);
CREATE INDEX idx_organization_events_event_id ON organization_events(event_id);
CREATE INDEX idx_organization_events_created_at ON organization_events(created_at);
CREATE INDEX idx_organization_events_org_event ON organization_events(organization_id, event_id);
CREATE INDEX idx_organization_events_data ON organization_events USING GIN(data);
