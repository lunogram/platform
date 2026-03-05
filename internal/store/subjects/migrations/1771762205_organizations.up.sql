-- Subject type enum for events (user vs organization)
CREATE TYPE subject_type AS ENUM ('user', 'organization', 'organization_user');

-- Add subject_type column to events table
ALTER TABLE events ADD COLUMN subject_type subject_type NOT NULL DEFAULT 'user';

-- Drop the old unique index and create a new one that includes subject_type
DROP INDEX idx_events_project_name;
CREATE UNIQUE INDEX idx_events_project_name_subject ON events(project_id, name, subject_type);
CREATE INDEX idx_events_project_subject ON events(project_id, subject_type);

-- Rename user_event_schemas to event_schemas (now shared for both user and org events)
ALTER TABLE user_event_schemas RENAME TO event_schemas;
ALTER INDEX idx_user_event_schemas_event_id RENAME TO idx_event_schemas_event_id;
ALTER INDEX idx_user_event_schemas_event_path RENAME TO idx_event_schemas_event_path;

-- Rename user_schemas to subject_schemas and add subject_type column
ALTER TABLE user_schemas RENAME TO subject_schemas;
ALTER TABLE subject_schemas ADD COLUMN subject_type subject_type NOT NULL DEFAULT 'user';
DROP INDEX idx_user_schemas_project_path;
CREATE UNIQUE INDEX idx_subject_schemas_project_path_type ON subject_schemas(project_id, path, data_type, subject_type);

-- Organizations table
-- Stores organization/company data that can be used as event subjects
CREATE TABLE organizations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id UUID NOT NULL,
    external_id VARCHAR(255) NOT NULL,
    name VARCHAR(255),
    data JSONB NOT NULL DEFAULT '{}'::jsonb,
    version INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
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
    version INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX organization_users_org_user_uniq ON organization_users(organization_id, user_id);
CREATE INDEX organization_users_organization_id_idx ON organization_users(organization_id);
CREATE INDEX organization_users_user_id_idx ON organization_users(user_id);
CREATE INDEX organization_users_data_idx ON organization_users USING gin(data);

CREATE TRIGGER set_updated_at_organization_users BEFORE UPDATE ON organization_users FOR EACH ROW EXECUTE PROCEDURE set_updated_at();
CREATE TRIGGER increment_version_organization_users BEFORE UPDATE ON organization_users FOR EACH ROW EXECUTE PROCEDURE increment_version();

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
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_organization_events_organization_id ON organization_events(organization_id);
CREATE INDEX idx_organization_events_event_id ON organization_events(event_id);
CREATE INDEX idx_organization_events_created_at ON organization_events(created_at);
CREATE INDEX idx_organization_events_org_event ON organization_events(organization_id, event_id);
CREATE INDEX idx_organization_events_data ON organization_events USING GIN(data);
