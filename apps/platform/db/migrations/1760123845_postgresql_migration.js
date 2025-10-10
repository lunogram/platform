exports.up = async function (knex) {
    await knex.raw(`
CREATE OR REPLACE FUNCTION set_updated_at () RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  NEW.updated_at := current_timestamp;
  RETURN NEW;
END; $$;

CREATE OR REPLACE FUNCTION increment_version () RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  NEW.version := OLD.version + 1;
  RETURN NEW;
END; $$;

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

/* -------------------- admins -------------------- */
CREATE TABLE admins (
  id uuid PRIMARY KEY DEFAULT uuid_generate_v4 (),
  organization_id uuid,
  first_name VARCHAR(255),
  last_name VARCHAR(255),
  email VARCHAR(255) NOT NULL,
  image_url VARCHAR(255),
  created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  deleted_at TIMESTAMPTZ,
  role VARCHAR(64) NOT NULL DEFAULT 'member'
);

CREATE TRIGGER admins_set_updated_at BEFORE
UPDATE ON admins FOR EACH ROW
EXECUTE FUNCTION set_updated_at ();

-- Indexes
CREATE INDEX admins_organization_id_idx ON admins (organization_id);

/* -------------------- access_tokens -------------------- */
CREATE TABLE access_tokens (
  id uuid PRIMARY KEY DEFAULT uuid_generate_v4 (),
  admin_id uuid NOT NULL,
  token TEXT NOT NULL,
  revoked BOOLEAN DEFAULT FALSE,
  expires_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  ip VARCHAR(255),
  user_agent VARCHAR(255)
);

CREATE TRIGGER access_tokens_set_updated_at BEFORE
UPDATE ON access_tokens FOR EACH ROW
EXECUTE FUNCTION set_updated_at ();

-- Indexes
CREATE INDEX access_tokens_token_idx ON access_tokens (token);

CREATE INDEX access_tokens_admin_id_idx ON access_tokens (admin_id);

/* -------------------- audits -------------------- */
CREATE TABLE audits (
  id uuid PRIMARY KEY DEFAULT uuid_generate_v4 (),
  project_id uuid NOT NULL,
  admin_id uuid,
  item_type VARCHAR(50) NOT NULL,
  item_id uuid NOT NULL,
  event VARCHAR(50) NOT NULL,
  object JSONB,
  object_changes JSONB,
  created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TRIGGER audits_set_updated_at BEFORE
UPDATE ON audits FOR EACH ROW
EXECUTE FUNCTION set_updated_at ();

-- Indexes
CREATE INDEX audits_project_id_idx ON audits (project_id);

CREATE INDEX audits_admin_id_idx ON audits (admin_id);

CREATE INDEX audits_event_idx ON audits (event);

CREATE INDEX audits_item_type_item_id_idx ON audits (item_type, item_id);

CREATE INDEX audits_created_at_idx ON audits (created_at);

/* -------------------- job_locks -------------------- */
CREATE TABLE job_locks (
  id uuid PRIMARY KEY DEFAULT uuid_generate_v4 (),
  key VARCHAR(255),
  owner VARCHAR(255),
  expiration TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TRIGGER job_locks_set_updated_at BEFORE
UPDATE ON job_locks FOR EACH ROW
EXECUTE FUNCTION set_updated_at ();

-- Indexes
CREATE UNIQUE INDEX job_locks_key_uniq ON job_locks (key);

/* -------------------- campaigns -------------------- */
CREATE TABLE campaigns (
  id uuid PRIMARY KEY DEFAULT uuid_generate_v4 (),
  type VARCHAR(255),
  project_id uuid NOT NULL,
  list_ids JSONB,
  exclusion_list_ids JSONB,
  name VARCHAR(255) DEFAULT '',
  channel VARCHAR(255) NOT NULL,
  provider_id uuid NOT NULL,
  subscription_id uuid NOT NULL,
  state VARCHAR(20),
  delivery JSONB,
  send_at TIMESTAMPTZ,
  send_in_user_timezone BOOLEAN DEFAULT FALSE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  deleted_at TIMESTAMPTZ
);

CREATE TRIGGER campaigns_set_updated_at BEFORE
UPDATE ON campaigns FOR EACH ROW
EXECUTE FUNCTION set_updated_at ();

-- Indexes
CREATE INDEX campaigns_project_id_idx ON campaigns (project_id);

CREATE INDEX campaigns_provider_id_idx ON campaigns (provider_id);

CREATE INDEX campaigns_subscription_id_idx ON campaigns (subscription_id);

CREATE INDEX campaigns_send_at_idx ON campaigns (send_at);

CREATE INDEX campaigns_project_state_idx ON campaigns (project_id, state);

/* -------------------- campaign_sends -------------------- */
CREATE TABLE campaign_sends (
  id uuid DEFAULT uuid_generate_v4 (),
  campaign_id uuid NOT NULL,
  user_id uuid NOT NULL,
  state VARCHAR(50),
  send_at TIMESTAMPTZ,
  opened_at TIMESTAMPTZ,
  clicks int4 DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  reference_type VARCHAR(255),
  reference_id VARCHAR(255) NOT NULL DEFAULT '0',
  PRIMARY KEY (campaign_id, user_id, reference_id)
);

CREATE TRIGGER campaign_sends_set_updated_at BEFORE
UPDATE ON campaign_sends FOR EACH ROW
EXECUTE FUNCTION set_updated_at ();

-- Indexes
CREATE INDEX campaign_sends_user_id_idx ON campaign_sends (user_id);

CREATE INDEX campaign_sends_send_at_idx ON campaign_sends (send_at);

CREATE INDEX campaign_sends_state_idx ON campaign_sends (state);

CREATE INDEX campaign_sends_campaign_state_idx ON campaign_sends (campaign_id, state);

/* -------------------- devices -------------------- */
CREATE TABLE devices (
  id uuid PRIMARY KEY DEFAULT uuid_generate_v4 (),
  project_id uuid NOT NULL,
  user_id uuid NOT NULL,
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

CREATE TRIGGER devices_set_updated_at BEFORE
UPDATE ON devices FOR EACH ROW
EXECUTE FUNCTION set_updated_at ();

-- Indexes
CREATE UNIQUE INDEX devices_project_device_uniq ON devices (project_id, device_id);

CREATE UNIQUE INDEX devices_project_token_uniq ON devices (project_id, token);

CREATE INDEX devices_user_id_idx ON devices (user_id);

/* -------------------- entity_tags -------------------- */
CREATE TABLE entity_tags (
  id uuid PRIMARY KEY DEFAULT uuid_generate_v4 (),
  tag_id uuid NOT NULL,
  entity VARCHAR(255),
  entity_id uuid NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TRIGGER entity_tags_set_updated_at BEFORE
UPDATE ON entity_tags FOR EACH ROW
EXECUTE FUNCTION set_updated_at ();

-- Indexes
CREATE INDEX entity_tags_tag_id_idx ON entity_tags (tag_id);

CREATE INDEX entity_tags_entity_entity_id_idx ON entity_tags (entity, entity_id);

/* -------------------- images -------------------- */
CREATE TABLE images (
  id uuid PRIMARY KEY DEFAULT uuid_generate_v4 (),
  project_id uuid NOT NULL,
  uuid VARCHAR(255) NOT NULL,
  name VARCHAR(255) DEFAULT '',
  original_name VARCHAR(255),
  extension VARCHAR(255),
  alt VARCHAR(255),
  file_size int4,
  created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TRIGGER images_set_updated_at BEFORE
UPDATE ON images FOR EACH ROW
EXECUTE FUNCTION set_updated_at ();

-- Indexes
CREATE INDEX images_project_id_idx ON images (project_id);

CREATE UNIQUE INDEX images_project_uuid_uniq ON images (project_id, uuid);

/* -------------------- journey_steps -------------------- */
CREATE TABLE journey_steps (
  id uuid PRIMARY KEY DEFAULT uuid_generate_v4 (),
  type VARCHAR(255) DEFAULT '',
  journey_id uuid,
  child_id uuid,
  data JSONB,
  x float8 NOT NULL DEFAULT 0,
  y float8 NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  external_id VARCHAR(128),
  data_key VARCHAR(255),
  stats JSONB,
  stats_at TIMESTAMPTZ,
  name VARCHAR(128),
  next_scheduled_at TIMESTAMPTZ
);

CREATE TRIGGER journey_steps_set_updated_at BEFORE
UPDATE ON journey_steps FOR EACH ROW
EXECUTE FUNCTION set_updated_at ();

-- Indexes
CREATE INDEX journey_steps_child_id_idx ON journey_steps (child_id);

CREATE UNIQUE INDEX journey_steps_journey_external_uniq ON journey_steps (journey_id, external_id);

/* -------------------- journey_step_child -------------------- */
CREATE TABLE journey_step_child (
  id uuid PRIMARY KEY DEFAULT uuid_generate_v4 (),
  step_id uuid NOT NULL,
  child_id uuid NOT NULL,
  data JSONB,
  created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  priority int4 NOT NULL DEFAULT 0,
  path VARCHAR(128)
);

CREATE TRIGGER journey_step_child_set_updated_at BEFORE
UPDATE ON journey_step_child FOR EACH ROW
EXECUTE FUNCTION set_updated_at ();

-- Indexes
CREATE INDEX journey_step_child_child_id_idx ON journey_step_child (child_id);

CREATE UNIQUE INDEX journey_step_child_step_child_uniq ON journey_step_child (step_id, child_id);

/* -------------------- journey_user_step -------------------- */
CREATE TABLE journey_user_step (
  id uuid PRIMARY KEY DEFAULT uuid_generate_v4 (),
  user_id uuid,
  journey_id uuid,
  step_id uuid,
  type VARCHAR(255),
  created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  delay_until TIMESTAMPTZ,
  entrance_id uuid,
  ended_at TIMESTAMPTZ,
  data JSONB,
  ref VARCHAR(64)
);

CREATE TRIGGER journey_user_step_set_updated_at BEFORE
UPDATE ON journey_user_step FOR EACH ROW
EXECUTE FUNCTION set_updated_at ();

-- Indexes
CREATE INDEX journey_user_step_step_id_idx ON journey_user_step (step_id);

CREATE INDEX journey_user_step_entrance_id_idx ON journey_user_step (entrance_id);

CREATE INDEX journey_user_step_user_id_idx ON journey_user_step (user_id);

CREATE INDEX journey_user_step_journey_type_delay_idx ON journey_user_step (journey_id, type, delay_until);

CREATE INDEX journey_user_step_ref_idx ON journey_user_step (ref);

-- Helpful extras:
CREATE INDEX journey_user_step_delay_until_notnull_idx ON journey_user_step (delay_until)
WHERE
  delay_until IS NOT NULL;

CREATE INDEX journey_user_step_user_journey_idx ON journey_user_step (user_id, journey_id);

/* -------------------- organizations -------------------- */
CREATE TABLE organizations (
  id uuid PRIMARY KEY DEFAULT uuid_generate_v4 (),
  username VARCHAR(255),
  domain VARCHAR(255),
  auth JSONB,
  created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  tracking_deeplink_mirror_url VARCHAR(255),
  notification_provider_id uuid
);

CREATE TRIGGER organizations_set_updated_at BEFORE
UPDATE ON organizations FOR EACH ROW
EXECUTE FUNCTION set_updated_at ();

-- Indexes
CREATE INDEX organizations_domain_idx ON organizations (domain);

CREATE UNIQUE INDEX organizations_username_uniq ON organizations (username);

CREATE INDEX organizations_notification_provider_id_idx ON organizations (notification_provider_id);

/* -------------------- projects -------------------- */
CREATE TABLE projects (
  id uuid PRIMARY KEY DEFAULT uuid_generate_v4 (),
  organization_id uuid,
  name VARCHAR(255) DEFAULT '',
  description VARCHAR(2048) DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  deleted_at TIMESTAMPTZ,
  locale VARCHAR(50),
  timezone VARCHAR(50),
  text_opt_out_message VARCHAR(255),
  link_wrap_email BOOLEAN DEFAULT FALSE,
  text_help_message VARCHAR(255),
  link_wrap_push BOOLEAN DEFAULT FALSE
);

CREATE TRIGGER projects_set_updated_at BEFORE
UPDATE ON projects FOR EACH ROW
EXECUTE FUNCTION set_updated_at ();

-- Indexes
CREATE INDEX projects_organization_id_idx ON projects (organization_id);

CREATE INDEX projects_name_idx ON projects (name);

/* -------------------- journeys -------------------- */
CREATE TABLE journeys (
  id uuid PRIMARY KEY DEFAULT uuid_generate_v4 (),
  project_id uuid NOT NULL,
  name VARCHAR(255) DEFAULT '',
  description VARCHAR(2048) DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  deleted_at TIMESTAMPTZ,
  stats JSONB,
  stats_at TIMESTAMPTZ,
  parent_id uuid,
  status VARCHAR(255)
);

CREATE TRIGGER journeys_set_updated_at BEFORE
UPDATE ON journeys FOR EACH ROW
EXECUTE FUNCTION set_updated_at ();

-- Indexes
CREATE INDEX journeys_project_id_idx ON journeys (project_id);

CREATE INDEX journeys_parent_id_idx ON journeys (parent_id);

/* -------------------- lists -------------------- */
CREATE TABLE lists (
  id uuid PRIMARY KEY DEFAULT uuid_generate_v4 (),
  project_id uuid NOT NULL,
  name VARCHAR(255) DEFAULT '',
  type VARCHAR(25),
  state VARCHAR(25),
  rule JSONB,
  rule_id uuid,
  users_count int4,
  version int4 NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  deleted_at TIMESTAMPTZ,
  is_visible BOOLEAN DEFAULT TRUE,
  refreshed_at TIMESTAMPTZ
);

CREATE TRIGGER lists_set_updated_at BEFORE
UPDATE ON lists FOR EACH ROW
EXECUTE FUNCTION set_updated_at ();

CREATE TRIGGER list_increment_version BEFORE
UPDATE ON lists FOR EACH ROW
EXECUTE FUNCTION increment_version ();

-- Indexes
CREATE INDEX lists_project_id_idx ON lists (project_id);

CREATE INDEX lists_rule_id_idx ON lists (rule_id);

CREATE INDEX lists_project_active_idx ON lists (project_id)
WHERE
  deleted_at IS NULL;

/* -------------------- locales -------------------- */
CREATE TABLE locales (
  id uuid PRIMARY KEY DEFAULT uuid_generate_v4 (),
  project_id uuid NOT NULL,
  key VARCHAR(255),
  label VARCHAR(255),
  created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TRIGGER locales_set_updated_at BEFORE
UPDATE ON locales FOR EACH ROW
EXECUTE FUNCTION set_updated_at ();

-- Indexes
CREATE INDEX locales_project_id_idx ON locales (project_id);

/* -------------------- notifications -------------------- */
CREATE TABLE notifications (
  id uuid PRIMARY KEY DEFAULT uuid_generate_v4 (),
  project_id uuid NOT NULL,
  user_id uuid NOT NULL,
  content_type VARCHAR(255),
  content JSONB,
  read_at TIMESTAMPTZ,
  expires_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TRIGGER notifications_set_updated_at BEFORE
UPDATE ON notifications FOR EACH ROW
EXECUTE FUNCTION set_updated_at ();

-- Indexes
CREATE INDEX notifications_user_id_idx ON notifications (user_id);

CREATE INDEX notifications_project_id_idx ON notifications (project_id);

CREATE INDEX notifications_read_at_idx ON notifications (read_at);

CREATE INDEX notifications_expires_at_idx ON notifications (expires_at);

CREATE INDEX notifications_user_unread_idx ON notifications (user_id)
WHERE
  read_at IS NULL;

/* -------------------- project_admins -------------------- */
CREATE TABLE project_admins (
  id uuid PRIMARY KEY DEFAULT uuid_generate_v4 (),
  project_id uuid NOT NULL,
  admin_id uuid NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  deleted_at TIMESTAMPTZ,
  role VARCHAR(64) NOT NULL DEFAULT 'support'
);

CREATE TRIGGER project_admins_set_updated_at BEFORE
UPDATE ON project_admins FOR EACH ROW
EXECUTE FUNCTION set_updated_at ();

-- Indexes
CREATE INDEX project_admins_project_id_idx ON project_admins (project_id);

CREATE INDEX project_admins_admin_id_idx ON project_admins (admin_id);

/* -------------------- project_api_keys -------------------- */
CREATE TABLE project_api_keys (
  id uuid PRIMARY KEY DEFAULT uuid_generate_v4 (),
  project_id uuid NOT NULL,
  value VARCHAR(255) NOT NULL,
  scope VARCHAR(20),
  name VARCHAR(255) NOT NULL,
  description VARCHAR(2048),
  created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  deleted_at TIMESTAMPTZ,
  role VARCHAR(64) NOT NULL DEFAULT 'support'
);

CREATE TRIGGER project_api_keys_set_updated_at BEFORE
UPDATE ON project_api_keys FOR EACH ROW
EXECUTE FUNCTION set_updated_at ();

-- Indexes
CREATE UNIQUE INDEX project_api_keys_value_uniq ON project_api_keys (value);

CREATE INDEX project_api_keys_project_id_idx ON project_api_keys (project_id);

/* -------------------- users -------------------- */
CREATE TABLE users (
  id uuid PRIMARY KEY DEFAULT uuid_generate_v4 (),
  project_id uuid NOT NULL,
  anonymous_id VARCHAR(255),
  external_id VARCHAR(255),
  email VARCHAR(255),
  phone VARCHAR(64),
  data JSONB,
  devices JSONB,
  timezone VARCHAR(50),
  locale VARCHAR(255),
  created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  unsubscribe_ids UUID[] NOT NULL DEFAULT '{}',
  version int4 NOT NULL DEFAULT 0,
  has_push_device BOOLEAN DEFAULT FALSE
);

CREATE TRIGGER users_set_updated_at BEFORE
UPDATE ON users FOR EACH ROW
EXECUTE FUNCTION set_updated_at ();

CREATE TRIGGER users_increment_version BEFORE
UPDATE ON users FOR EACH ROW
EXECUTE FUNCTION increment_version ();

-- Indexes
CREATE UNIQUE INDEX users_project_anonymous_uniq ON users (project_id, anonymous_id);

CREATE UNIQUE INDEX users_project_external_uniq ON users (project_id, external_id);

CREATE INDEX users_email_idx ON users (email);

CREATE INDEX users_phone_idx ON users (phone);

CREATE INDEX users_external_id_idx ON users (external_id);

CREATE INDEX users_updated_at_idx ON users (updated_at);

CREATE INDEX users_anonymous_id_idx ON users (anonymous_id);

CREATE INDEX users_data_idx ON users USING GIN (data);

/* If you always scope by project, the single-column indexes above can be dropped in favor of composites. */
/* -------------------- providers -------------------- */
CREATE TABLE providers (
  id uuid PRIMARY KEY DEFAULT uuid_generate_v4 (),
  project_id uuid NOT NULL,
  type VARCHAR(255) DEFAULT '',
  "group" VARCHAR(255) NOT NULL,
  data JSONB,
  is_default BOOLEAN DEFAULT FALSE,
  rate_limit int4,
  rate_interval VARCHAR(12) DEFAULT 'second',
  created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  name VARCHAR(255),
  deleted_at TIMESTAMPTZ
);

CREATE TRIGGER providers_set_updated_at BEFORE
UPDATE ON providers FOR EACH ROW
EXECUTE FUNCTION set_updated_at ();

-- Indexes
CREATE INDEX providers_project_id_idx ON providers (project_id);

/* -------------------- subscriptions -------------------- */
CREATE TABLE subscriptions (
  id uuid PRIMARY KEY DEFAULT uuid_generate_v4 (),
  project_id uuid NOT NULL,
  name VARCHAR(255) DEFAULT '',
  channel VARCHAR(255) NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  is_public BOOLEAN NOT NULL DEFAULT TRUE
);

CREATE TRIGGER subscriptions_set_updated_at BEFORE
UPDATE ON subscriptions FOR EACH ROW
EXECUTE FUNCTION set_updated_at ();

-- Indexes
CREATE INDEX subscriptions_project_id_idx ON subscriptions (project_id);

/* -------------------- tags -------------------- */
CREATE TABLE tags (
  id uuid PRIMARY KEY DEFAULT uuid_generate_v4 (),
  project_id uuid NOT NULL,
  name VARCHAR(255) DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TRIGGER tags_set_updated_at BEFORE
UPDATE ON tags FOR EACH ROW
EXECUTE FUNCTION set_updated_at ();

-- Indexes
CREATE INDEX tags_project_id_idx ON tags (project_id);

/* -------------------- rules -------------------- */
CREATE TABLE rules (
  id uuid PRIMARY KEY DEFAULT uuid_generate_v4 (),
  project_id uuid NOT NULL,
  uuid VARCHAR(255),
  root_uuid VARCHAR(255),
  parent_uuid VARCHAR(255),
  type VARCHAR(255),
  "group" VARCHAR(255),
  path VARCHAR(255),
  operator VARCHAR(255),
  value VARCHAR(255),
  created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE (uuid)
);

CREATE TRIGGER rules_set_updated_at BEFORE
UPDATE ON rules FOR EACH ROW
EXECUTE FUNCTION set_updated_at ();

-- Indexes
CREATE INDEX rules_project_id_idx ON rules (project_id);

CREATE INDEX rules_parent_uuid_idx ON rules (parent_uuid);

CREATE INDEX rules_root_uuid_idx ON rules (root_uuid);

CREATE INDEX rules_group_idx ON rules ("group");

CREATE INDEX rules_type_idx ON rules (type);

CREATE INDEX rules_value_idx ON rules (value);

/* -------------------- project_rule_paths -------------------- */
CREATE SEQUENCE IF NOT EXISTS project_rule_paths_id_seq;

DROP TYPE IF EXISTS project_rule_paths_visibility;

CREATE TYPE project_rule_paths_visibility AS ENUM('public', 'hidden', 'classified');

CREATE TABLE project_rule_paths (
  id uuid PRIMARY KEY DEFAULT uuid_generate_v4 (),
  project_id uuid NOT NULL,
  path VARCHAR(255) NOT NULL,
  name VARCHAR(255),
  type VARCHAR(50) NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  data_type VARCHAR(255) DEFAULT 'string',
  visibility project_rule_paths_visibility NOT NULL DEFAULT 'public'
);

CREATE TRIGGER project_rule_paths_set_updated_at BEFORE
UPDATE ON project_rule_paths FOR EACH ROW
EXECUTE FUNCTION set_updated_at ();

-- Indexes
CREATE INDEX project_rule_paths_project_id_idx ON project_rule_paths (project_id);

/* -------------------- resources -------------------- */
CREATE TABLE resources (
  id uuid PRIMARY KEY DEFAULT uuid_generate_v4 (),
  project_id uuid NOT NULL,
  type VARCHAR(255),
  name VARCHAR(255),
  value JSONB,
  created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TRIGGER resources_set_updated_at BEFORE
UPDATE ON resources FOR EACH ROW
EXECUTE FUNCTION set_updated_at ();

-- Indexes
CREATE INDEX resources_project_id_idx ON resources (project_id);

/* -------------------- rule_evaluations -------------------- */
CREATE TABLE rule_evaluations (
  id uuid PRIMARY KEY DEFAULT uuid_generate_v4 (),
  rule_id uuid NOT NULL,
  user_id uuid NOT NULL,
  result BOOLEAN,
  created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TRIGGER rule_evaluations_set_updated_at BEFORE
UPDATE ON rule_evaluations FOR EACH ROW
EXECUTE FUNCTION set_updated_at ();

-- Indexes
CREATE INDEX rule_evaluations_rule_id_idx ON rule_evaluations (rule_id);

CREATE UNIQUE INDEX rule_evaluations_user_rule_uniq ON rule_evaluations (user_id, rule_id);

/* -------------------- templates -------------------- */
CREATE TABLE templates (
  id uuid PRIMARY KEY DEFAULT uuid_generate_v4 (),
  project_id uuid NOT NULL,
  name VARCHAR(255),
  type VARCHAR(50),
  data JSONB,
  created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  locale VARCHAR(50),
  campaign_id uuid NOT NULL
);

CREATE TRIGGER templates_set_updated_at BEFORE
UPDATE ON templates FOR EACH ROW
EXECUTE FUNCTION set_updated_at ();

-- Indexes
CREATE INDEX templates_campaign_id_idx ON templates (campaign_id);

CREATE INDEX templates_project_id_idx ON templates (project_id);

/* -------------------- user_events -------------------- */
CREATE TABLE user_events (
  id uuid PRIMARY KEY DEFAULT uuid_generate_v4 (),
  name VARCHAR(255) DEFAULT '',
  project_id uuid NOT NULL,
  user_id uuid NOT NULL,
  data JSONB,
  created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TRIGGER user_events_set_updated_at BEFORE
UPDATE ON user_events FOR EACH ROW
EXECUTE FUNCTION set_updated_at ();

-- Indexes
CREATE INDEX user_events_project_id_idx ON user_events (project_id);

CREATE INDEX user_events_user_id_idx ON user_events (user_id);

CREATE INDEX user_events_created_at_idx ON user_events (created_at);

CREATE INDEX user_events_name_user_idx ON user_events (name, user_id);

CREATE INDEX user_events_data_idx ON user_events USING GIN (data);

/* -------------------- user_list -------------------- */
CREATE TABLE user_list (
  id uuid PRIMARY KEY DEFAULT uuid_generate_v4 (),
  user_id uuid NOT NULL,
  list_id uuid NOT NULL,
  event_id uuid,
  version int4 NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  deleted_at TIMESTAMPTZ
);

CREATE TRIGGER user_list_set_updated_at BEFORE
UPDATE ON user_list FOR EACH ROW
EXECUTE FUNCTION set_updated_at ();

CREATE TRIGGER user_list_increment_version BEFORE
UPDATE ON user_list FOR EACH ROW
EXECUTE FUNCTION increment_version ();

-- Indexes
CREATE UNIQUE INDEX user_list_user_list_uniq ON user_list (user_id, list_id);

CREATE INDEX user_list_version_idx ON user_list (version);

CREATE INDEX user_list_list_id_idx ON user_list (list_id);

CREATE INDEX user_list_event_id_idx ON user_list (event_id);

CREATE INDEX user_list_created_at_idx ON user_list (created_at);

/* -------------------- user_subscription -------------------- */
CREATE TABLE user_subscription (
  id uuid PRIMARY KEY DEFAULT uuid_generate_v4 (),
  subscription_id uuid NOT NULL,
  user_id uuid NOT NULL,
  state int2,
  created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TRIGGER user_subscription_set_updated_at BEFORE
UPDATE ON user_subscription FOR EACH ROW
EXECUTE FUNCTION set_updated_at ();

-- Indexes
CREATE INDEX user_subscription_user_id_idx ON user_subscription (user_id);

CREATE INDEX user_subscription_subscription_id_idx ON user_subscription (subscription_id);

/* -------------------- Foreign keys (unchanged order) -------------------- */
ALTER TABLE admins
ADD FOREIGN KEY (organization_id) REFERENCES organizations (id) ON DELETE CASCADE;

ALTER TABLE access_tokens
ADD FOREIGN KEY (admin_id) REFERENCES admins (id) ON DELETE CASCADE;

ALTER TABLE audits
ADD FOREIGN KEY (admin_id) REFERENCES admins (id) ON DELETE CASCADE;

ALTER TABLE audits
ADD FOREIGN KEY (project_id) REFERENCES projects (id) ON DELETE CASCADE;

ALTER TABLE campaigns
ADD FOREIGN KEY (provider_id) REFERENCES providers (id) ON DELETE CASCADE;

ALTER TABLE campaigns
ADD FOREIGN KEY (subscription_id) REFERENCES subscriptions (id) ON DELETE CASCADE;

ALTER TABLE campaigns
ADD FOREIGN KEY (project_id) REFERENCES projects (id) ON DELETE CASCADE;

ALTER TABLE campaign_sends
ADD FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE;

ALTER TABLE campaign_sends
ADD FOREIGN KEY (campaign_id) REFERENCES campaigns (id) ON DELETE CASCADE;

ALTER TABLE entity_tags
ADD FOREIGN KEY (tag_id) REFERENCES tags (id) ON DELETE CASCADE;

ALTER TABLE devices
ADD FOREIGN KEY (project_id) REFERENCES projects (id) ON DELETE CASCADE;

ALTER TABLE devices
ADD FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE;

ALTER TABLE images
ADD FOREIGN KEY (project_id) REFERENCES projects (id) ON DELETE CASCADE;

ALTER TABLE journey_step_child
ADD FOREIGN KEY (child_id) REFERENCES journey_steps (id) ON DELETE CASCADE;

ALTER TABLE journey_step_child
ADD FOREIGN KEY (step_id) REFERENCES journey_steps (id) ON DELETE CASCADE;

ALTER TABLE user_subscription
ADD FOREIGN KEY (subscription_id) REFERENCES subscriptions (id) ON DELETE CASCADE;

ALTER TABLE user_subscription
ADD FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE;

ALTER TABLE user_list
ADD FOREIGN KEY (list_id) REFERENCES lists (id) ON DELETE CASCADE;

ALTER TABLE user_list
ADD FOREIGN KEY (event_id) REFERENCES user_events (id) ON DELETE CASCADE;

ALTER TABLE user_list
ADD FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE;

ALTER TABLE user_events
ADD FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE;

ALTER TABLE user_events
ADD FOREIGN KEY (project_id) REFERENCES projects (id) ON DELETE CASCADE;

ALTER TABLE rule_evaluations
ADD FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE;

ALTER TABLE rule_evaluations
ADD FOREIGN KEY (rule_id) REFERENCES rules (id) ON DELETE CASCADE;

ALTER TABLE project_rule_paths
ADD FOREIGN KEY (project_id) REFERENCES projects (id) ON DELETE CASCADE;

ALTER TABLE journey_steps
ADD FOREIGN KEY (journey_id) REFERENCES journeys (id) ON DELETE CASCADE;

ALTER TABLE journey_steps
ADD FOREIGN KEY (child_id) REFERENCES journey_steps (id) ON DELETE SET NULL;

ALTER TABLE journey_user_step
ADD FOREIGN KEY (journey_id) REFERENCES journeys (id) ON DELETE CASCADE;

ALTER TABLE journey_user_step
ADD FOREIGN KEY (entrance_id) REFERENCES journey_user_step (id) ON DELETE CASCADE;

ALTER TABLE journey_user_step
ADD FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE;

ALTER TABLE journey_user_step
ADD FOREIGN KEY (step_id) REFERENCES journey_steps (id) ON DELETE CASCADE;

ALTER TABLE organizations
ADD FOREIGN KEY (notification_provider_id) REFERENCES providers (id) ON DELETE SET NULL;

ALTER TABLE projects
ADD FOREIGN KEY (organization_id) REFERENCES organizations (id) ON DELETE CASCADE;

ALTER TABLE journeys
ADD FOREIGN KEY (parent_id) REFERENCES journeys (id) ON DELETE CASCADE;

ALTER TABLE journeys
ADD FOREIGN KEY (project_id) REFERENCES projects (id) ON DELETE CASCADE;

ALTER TABLE lists
ADD FOREIGN KEY (rule_id) REFERENCES rules (id) ON DELETE SET NULL;

ALTER TABLE lists
ADD FOREIGN KEY (project_id) REFERENCES projects (id) ON DELETE CASCADE;

ALTER TABLE locales
ADD FOREIGN KEY (project_id) REFERENCES projects (id) ON DELETE CASCADE;

ALTER TABLE notifications
ADD FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE;

ALTER TABLE notifications
ADD FOREIGN KEY (project_id) REFERENCES projects (id) ON DELETE CASCADE;

ALTER TABLE project_admins
ADD FOREIGN KEY (admin_id) REFERENCES admins (id) ON DELETE CASCADE;

ALTER TABLE project_admins
ADD FOREIGN KEY (project_id) REFERENCES projects (id) ON DELETE CASCADE;

ALTER TABLE project_api_keys
ADD FOREIGN KEY (project_id) REFERENCES projects (id) ON DELETE CASCADE;

ALTER TABLE users
ADD FOREIGN KEY (project_id) REFERENCES projects (id) ON DELETE CASCADE;

ALTER TABLE providers
ADD FOREIGN KEY (project_id) REFERENCES projects (id) ON DELETE CASCADE;

ALTER TABLE subscriptions
ADD FOREIGN KEY (project_id) REFERENCES projects (id) ON DELETE CASCADE;

ALTER TABLE tags
ADD FOREIGN KEY (project_id) REFERENCES projects (id) ON DELETE CASCADE;

ALTER TABLE rules
ADD FOREIGN KEY (parent_uuid) REFERENCES rules (uuid) ON DELETE CASCADE;

ALTER TABLE rules
ADD FOREIGN KEY (root_uuid) REFERENCES rules (uuid) ON DELETE CASCADE;

ALTER TABLE rules
ADD FOREIGN KEY (project_id) REFERENCES projects (id) ON DELETE CASCADE;

ALTER TABLE templates
ADD FOREIGN KEY (project_id) REFERENCES projects (id) ON DELETE CASCADE;

ALTER TABLE templates
ADD FOREIGN KEY (campaign_id) REFERENCES campaigns (id) ON DELETE CASCADE;
    `)
}

exports.down = async function (knex) {
}
