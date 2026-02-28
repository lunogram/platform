-- Users Database Initial Migration
-- This database contains user/customer data tables

-- Create extension in public schema so it's visible to all schemas
CREATE EXTENSION IF NOT EXISTS "uuid-ossp" SCHEMA public;

-- Helper function for updating timestamps
CREATE OR REPLACE FUNCTION set_updated_at() RETURNS trigger
  LANGUAGE plpgsql
  AS $$
BEGIN
  new.updated_at := current_timestamp;
  return new;
END;
$$;

-- Helper function for incrementing version
CREATE OR REPLACE FUNCTION increment_version() RETURNS trigger
  LANGUAGE plpgsql
  AS $$
BEGIN
  new.version := old.version + 1;
  return new;
END;
$$;

-- Enum types
CREATE TYPE data_type AS ENUM ('string', 'number', 'boolean', 'object', 'array', 'null');
CREATE TYPE project_rule_paths_visibility AS ENUM ('public', 'hidden', 'classified');

-- Users table
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id UUID NOT NULL,
    anonymous_id VARCHAR(255) DEFAULT (uuid_generate_v4())::text,
    external_id VARCHAR(255),
    email VARCHAR(255),
    phone VARCHAR(64),
    data JSONB NOT NULL DEFAULT '{}'::jsonb,
    devices JSONB,
    timezone VARCHAR(50),
    locale VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    unsubscribe_ids UUID[] NOT NULL DEFAULT '{}'::uuid[],
    version INTEGER NOT NULL DEFAULT 0
);

CREATE UNIQUE INDEX users_project_anonymous_uniq ON users(project_id, anonymous_id);
CREATE UNIQUE INDEX users_project_external_uniq ON users(project_id, external_id);
CREATE UNIQUE INDEX users_project_id_uniq ON users(project_id, id);
CREATE INDEX users_email_idx ON users(email);
CREATE INDEX users_phone_idx ON users(phone);
CREATE INDEX users_external_id_idx ON users(external_id);
CREATE INDEX users_updated_at_idx ON users(updated_at);
CREATE INDEX users_anonymous_id_idx ON users(anonymous_id);
CREATE INDEX users_data_idx ON users USING gin(data);

CREATE TRIGGER set_updated_at_users BEFORE UPDATE ON users FOR EACH ROW EXECUTE PROCEDURE set_updated_at();
CREATE TRIGGER increment_version_users BEFORE UPDATE ON users FOR EACH ROW EXECUTE PROCEDURE increment_version();

-- Devices table
CREATE TABLE devices (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id UUID NOT NULL,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    device_id TEXT NOT NULL,
    token VARCHAR(255),
    os VARCHAR(255),
    os_version VARCHAR(255),
    model VARCHAR(255),
    app_version VARCHAR(255),
    app_build VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX devices_project_device_uniq ON devices(project_id, device_id);
CREATE UNIQUE INDEX devices_project_token_uniq ON devices(project_id, token);
CREATE INDEX devices_user_id_idx ON devices(user_id);

CREATE TRIGGER set_updated_at_devices BEFORE UPDATE ON devices FOR EACH ROW EXECUTE PROCEDURE set_updated_at();

-- Events table (event definitions)
CREATE TABLE events (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX idx_events_project_name ON events(project_id, name);

CREATE TRIGGER set_updated_at_events BEFORE UPDATE ON events FOR EACH ROW EXECUTE PROCEDURE set_updated_at();

-- Event schemas table
CREATE TABLE user_event_schemas (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    event_id UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    path VARCHAR(255) NOT NULL,
    data_type data_type NOT NULL,
    visibility project_rule_paths_visibility NOT NULL DEFAULT 'hidden',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_user_event_schemas_event_id ON user_event_schemas(event_id);
CREATE UNIQUE INDEX idx_user_event_schemas_event_path ON user_event_schemas(event_id, path, data_type);

CREATE TRIGGER set_updated_at_user_event_schemas BEFORE UPDATE ON user_event_schemas FOR EACH ROW EXECUTE PROCEDURE set_updated_at();

-- User schemas table
CREATE TABLE user_schemas (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id UUID NOT NULL,
    path VARCHAR(255) NOT NULL,
    data_type data_type NOT NULL,
    visibility project_rule_paths_visibility NOT NULL DEFAULT 'hidden',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX idx_user_schemas_project_path ON user_schemas(project_id, path, data_type);

CREATE TRIGGER set_updated_at_user_schemas BEFORE UPDATE ON user_schemas FOR EACH ROW EXECUTE PROCEDURE set_updated_at();

-- User events table (event occurrences)
CREATE TABLE user_events (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    event_id UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    data JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_user_events_user_id ON user_events(user_id);
CREATE INDEX idx_user_events_event_id ON user_events(event_id);
CREATE INDEX idx_user_events_created_at ON user_events(created_at);
CREATE INDEX idx_user_events_user_event ON user_events(user_id, event_id);
CREATE INDEX idx_user_events_data ON user_events USING GIN(data);

-- Rules table
CREATE TABLE rules (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id UUID NOT NULL,
    rule JSONB NOT NULL,
    depends_on_events BOOLEAN NOT NULL DEFAULT FALSE,
    depends_on_users BOOLEAN NOT NULL DEFAULT FALSE,
    version INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_rules_project_id ON rules(project_id);

CREATE TRIGGER increment_version_rules BEFORE UPDATE ON rules FOR EACH ROW EXECUTE PROCEDURE increment_version();
CREATE TRIGGER set_rules_updated_at BEFORE UPDATE ON rules FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Rules events table
CREATE TABLE rules_events (
    rule_id UUID NOT NULL REFERENCES rules(id) ON DELETE RESTRICT,
    event_id UUID NOT NULL REFERENCES events(id) ON DELETE RESTRICT,
    PRIMARY KEY (rule_id, event_id)
);

CREATE INDEX idx_rules_events_rule_id ON rules_events(rule_id);
CREATE INDEX idx_rules_events_event_id ON rules_events(event_id);

-- Lists table
CREATE TABLE lists (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id UUID NOT NULL,
    name VARCHAR(255) DEFAULT '',
    type VARCHAR(25),
    version INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMPTZ,
    rule_id UUID REFERENCES rules(id) ON DELETE RESTRICT
);

CREATE INDEX lists_project_id_idx ON lists(project_id);
CREATE INDEX lists_project_active_idx ON lists(project_id) WHERE deleted_at IS NULL;

CREATE TRIGGER set_updated_at_lists BEFORE UPDATE ON lists FOR EACH ROW EXECUTE PROCEDURE set_updated_at();
CREATE TRIGGER increment_version_lists BEFORE UPDATE ON lists FOR EACH ROW EXECUTE PROCEDURE increment_version();

-- List users table
CREATE TABLE list_users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    list_id UUID NOT NULL REFERENCES lists(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_list_users_list_id ON list_users(list_id);
CREATE INDEX idx_list_users_user_id ON list_users(user_id);
CREATE INDEX idx_list_users_list_user ON list_users(list_id, user_id);
CREATE UNIQUE INDEX idx_list_users_unique_active ON list_users(list_id, user_id);

-- Campaign sends table (high-volume user delivery tracking)
CREATE TABLE campaign_sends (
    id UUID DEFAULT uuid_generate_v4(),
    campaign_id UUID NOT NULL,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    state VARCHAR(50),
    sent_at TIMESTAMPTZ,
    opened_at TIMESTAMPTZ,
    clicks INTEGER DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    reference_type VARCHAR(255),
    reference_id VARCHAR(255) NOT NULL DEFAULT '0',
    PRIMARY KEY (campaign_id, user_id, reference_id)
);

CREATE INDEX campaign_sends_user_id_idx ON campaign_sends(user_id);
CREATE INDEX campaign_sends_sent_at_idx ON campaign_sends(sent_at);
CREATE INDEX campaign_sends_state_idx ON campaign_sends(state);
CREATE INDEX campaign_sends_campaign_state_idx ON campaign_sends(campaign_id, state);

CREATE TRIGGER set_updated_at_campaign_sends BEFORE UPDATE ON campaign_sends FOR EACH ROW EXECUTE PROCEDURE set_updated_at();
