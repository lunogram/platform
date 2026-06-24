-- Management Database Initial Migration
-- This database contains organization/project administration tables

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

-- Organizations table
CREATE TABLE organizations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL DEFAULT '',
    auth JSONB,
    tracking_deeplink_mirror_url VARCHAR(255),
    notification_provider_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE TRIGGER set_updated_at_organizations BEFORE UPDATE ON organizations FOR EACH ROW EXECUTE PROCEDURE set_updated_at();

-- Admins table
CREATE TABLE admins (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    first_name VARCHAR(255),
    last_name VARCHAR(255),
    email VARCHAR(255) NOT NULL,
    image_url VARCHAR(255),
    role VARCHAR(64) NOT NULL DEFAULT 'member',
    external_id VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX admins_organization_id_idx ON admins(organization_id);
CREATE UNIQUE INDEX admins_email_unique ON admins(email) WHERE deleted_at IS NULL;

CREATE TRIGGER set_updated_at_admins BEFORE UPDATE ON admins FOR EACH ROW EXECUTE PROCEDURE set_updated_at();

-- Projects table
CREATE TABLE projects (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
    name VARCHAR(255) DEFAULT '',
    description VARCHAR(2048) DEFAULT '',
    timezone VARCHAR(50),
    text_opt_out_message VARCHAR(255),
    text_help_message VARCHAR(255),
    link_wrap_email BOOLEAN DEFAULT false,
    link_wrap_push BOOLEAN DEFAULT false,
    tools TEXT[],
    locale TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX projects_organization_id_idx ON projects(organization_id);
CREATE INDEX projects_name_idx ON projects(name);

CREATE TRIGGER set_updated_at_projects BEFORE UPDATE ON projects FOR EACH ROW EXECUTE PROCEDURE set_updated_at();

-- Project admins table
CREATE TABLE project_admins (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    admin_id UUID NOT NULL REFERENCES admins(id) ON DELETE CASCADE,
    role VARCHAR(64) NOT NULL DEFAULT 'support',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    UNIQUE (project_id, admin_id)
);

CREATE INDEX project_admins_project_id_idx ON project_admins(project_id);
CREATE INDEX project_admins_admin_id_idx ON project_admins(admin_id);
CREATE UNIQUE INDEX project_admins_project_admin_uniq ON project_admins(project_id, admin_id) WHERE deleted_at IS NULL;

CREATE TRIGGER set_updated_at_project_admins BEFORE UPDATE ON project_admins FOR EACH ROW EXECUTE PROCEDURE set_updated_at();

-- Project API keys table
CREATE TABLE project_api_keys (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    value VARCHAR(255) NOT NULL,
    scope VARCHAR(20),
    name VARCHAR(255) NOT NULL,
    description VARCHAR(2048),
    role VARCHAR(64) NOT NULL DEFAULT 'support',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX project_api_keys_value_uniq ON project_api_keys(value);
CREATE INDEX project_api_keys_project_id_idx ON project_api_keys(project_id);

CREATE TRIGGER set_updated_at_project_api_keys BEFORE UPDATE ON project_api_keys FOR EACH ROW EXECUTE PROCEDURE set_updated_at();

-- Providers table
CREATE TABLE providers (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    module VARCHAR(255) NOT NULL,
    channel VARCHAR(255) NOT NULL,
    data JSONB,
    is_default BOOLEAN DEFAULT false,
    rate_limit INTEGER,
    rate_interval VARCHAR(12) DEFAULT 'second',
    external_id VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX providers_project_id_idx ON providers(project_id);

CREATE TRIGGER set_updated_at_providers BEFORE UPDATE ON providers FOR EACH ROW EXECUTE PROCEDURE set_updated_at();

-- Add foreign key for organizations.notification_provider_id after providers table exists
ALTER TABLE organizations
    ADD CONSTRAINT fk_organizations_notification_provider_id
    FOREIGN KEY (notification_provider_id)
    REFERENCES providers(id)
    ON DELETE SET NULL;

CREATE INDEX organizations_notification_provider_id_idx ON organizations(notification_provider_id);

-- Subscriptions table
CREATE TABLE subscriptions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name VARCHAR(255) DEFAULT '',
    channel VARCHAR(255) NOT NULL,
    is_public BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX subscriptions_project_id_idx ON subscriptions(project_id);

CREATE TRIGGER set_updated_at_subscriptions BEFORE UPDATE ON subscriptions FOR EACH ROW EXECUTE PROCEDURE set_updated_at();

-- User subscriptions table (tracks user opt-in/opt-out state)
CREATE TABLE user_subscription (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    subscription_id UUID NOT NULL REFERENCES subscriptions(id) ON DELETE CASCADE,
    user_id UUID NOT NULL,
    state SMALLINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX user_subscription_user_id_idx ON user_subscription(user_id);
CREATE INDEX user_subscription_subscription_id_idx ON user_subscription(subscription_id);

CREATE TRIGGER set_updated_at_user_subscription BEFORE UPDATE ON user_subscription FOR EACH ROW EXECUTE PROCEDURE set_updated_at();

-- Campaigns table
CREATE TABLE campaigns (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    type VARCHAR(255),
    name VARCHAR(255) DEFAULT '',
    channel VARCHAR(255) NOT NULL,
    provider_id UUID REFERENCES providers(id) ON DELETE CASCADE,
    subscription_id UUID REFERENCES subscriptions(id) ON DELETE CASCADE,
    list_ids JSONB,
    exclusion_list_ids JSONB,
    state VARCHAR(20),
    delivery JSONB NOT NULL DEFAULT '{}'::jsonb,
    send_at TIMESTAMPTZ,
    send_in_user_timezone BOOLEAN DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX campaigns_project_id_idx ON campaigns(project_id);
CREATE INDEX campaigns_subscription_id_idx ON campaigns(subscription_id);
CREATE INDEX campaigns_send_at_idx ON campaigns(send_at);
CREATE INDEX campaigns_project_state_idx ON campaigns(project_id, state);
CREATE INDEX campaigns_provider_id_idx ON campaigns(provider_id);

CREATE TRIGGER set_updated_at_campaigns BEFORE UPDATE ON campaigns FOR EACH ROW EXECUTE PROCEDURE set_updated_at();

-- Templates table
CREATE TABLE templates (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    campaign_id UUID NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    type VARCHAR(50),
    locale VARCHAR(50),
    data JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX templates_campaign_id_idx ON templates(campaign_id);
CREATE INDEX templates_project_id_idx ON templates(project_id);

CREATE TRIGGER set_updated_at_templates BEFORE UPDATE ON templates FOR EACH ROW EXECUTE PROCEDURE set_updated_at();

-- Tags table
CREATE TABLE tags (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name VARCHAR(255) DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX tags_project_id_idx ON tags(project_id);

CREATE TRIGGER set_updated_at_tags BEFORE UPDATE ON tags FOR EACH ROW EXECUTE PROCEDURE set_updated_at();

-- Locales table
CREATE TABLE locales (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    key VARCHAR(255),
    label VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX locales_project_id_idx ON locales(project_id);

CREATE TRIGGER set_updated_at_locales BEFORE UPDATE ON locales FOR EACH ROW EXECUTE PROCEDURE set_updated_at();

-- Documents table (formerly images)
CREATE TABLE documents (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name VARCHAR(255) DEFAULT '',
    filename VARCHAR(255) NOT NULL,
    content_type VARCHAR(100) NOT NULL,
    size_bytes BIGINT NOT NULL,
    key VARCHAR(255) DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX documents_project_id_idx ON documents(project_id);
CREATE INDEX documents_key_idx ON documents(key);
CREATE INDEX documents_deleted_at_idx ON documents(deleted_at) WHERE deleted_at IS NULL;
CREATE INDEX documents_created_at_idx ON documents(created_at);

CREATE TRIGGER set_updated_at_documents BEFORE UPDATE ON documents FOR EACH ROW EXECUTE PROCEDURE set_updated_at();

-- Audits table
CREATE TABLE audits (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    admin_id UUID REFERENCES admins(id) ON DELETE CASCADE,
    item_type VARCHAR(50) NOT NULL,
    item_id UUID NOT NULL,
    event VARCHAR(50) NOT NULL,
    object JSONB,
    object_changes JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX audits_project_id_idx ON audits(project_id);
CREATE INDEX audits_admin_id_idx ON audits(admin_id);
CREATE INDEX audits_event_idx ON audits(event);
CREATE INDEX audits_item_type_item_id_idx ON audits(item_type, item_id);
CREATE INDEX audits_created_at_idx ON audits(created_at);

CREATE TRIGGER set_updated_at_audits BEFORE UPDATE ON audits FOR EACH ROW EXECUTE PROCEDURE set_updated_at();
