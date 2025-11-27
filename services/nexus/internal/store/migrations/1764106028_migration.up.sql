CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE OR REPLACE function set_updated_at() RETURNS trigger
  LANGUAGE plpgsql
  AS $$
BEGIN
  new.updated_at := current_timestamp;
  return new;
END;
$$;

-- Table Definition
CREATE TABLE "access_tokens" (
    "id" uuid NOT NULL DEFAULT uuid_generate_v4(),
    "admin_id" uuid NOT NULL,
    "token" text NOT NULL,
    "revoked" bool DEFAULT false,
    "expires_at" timestamptz,
    "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "ip" varchar(255),
    "user_agent" varchar(255),
    PRIMARY KEY ("id")
);

-- Table Definition
CREATE TABLE "audits" (
    "id" uuid NOT NULL DEFAULT uuid_generate_v4(),
    "project_id" uuid NOT NULL,
    "admin_id" uuid,
    "item_type" varchar(50) NOT NULL,
    "item_id" uuid NOT NULL,
    "event" varchar(50) NOT NULL,
    "object" jsonb,
    "object_changes" jsonb,
    "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY ("id")
);

-- Table Definition
CREATE TABLE "job_locks" (
    "id" uuid NOT NULL DEFAULT uuid_generate_v4(),
    "key" varchar(255),
    "owner" varchar(255),
    "expiration" timestamptz,
    "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY ("id")
);

-- Table Definition
CREATE TABLE "campaigns" (
    "id" uuid NOT NULL DEFAULT uuid_generate_v4(),
    "type" varchar(255),
    "project_id" uuid NOT NULL,
    "list_ids" jsonb,
    "exclusion_list_ids" jsonb,
    "name" varchar(255) DEFAULT ''::character varying,
    "channel" varchar(255) NOT NULL,
    "provider_id" uuid,
    "subscription_id" uuid,
    "state" varchar(20),
    "delivery" jsonb NOT NULL DEFAULT '{}'::jsonb,
    "send_at" timestamptz,
    "send_in_user_timezone" bool DEFAULT false,
    "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "deleted_at" timestamptz,
    PRIMARY KEY ("id")
);

-- Table Definition
CREATE TABLE "campaign_sends" (
    "id" uuid DEFAULT uuid_generate_v4(),
    "campaign_id" uuid NOT NULL,
    "user_id" uuid NOT NULL,
    "state" varchar(50),
    "send_at" timestamptz,
    "opened_at" timestamptz,
    "clicks" int4 DEFAULT 0,
    "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "reference_type" varchar(255),
    "reference_id" varchar(255) NOT NULL DEFAULT '0'::character varying,
    PRIMARY KEY ("campaign_id","user_id","reference_id")
);

-- Table Definition
CREATE TABLE "devices" (
    "id" uuid NOT NULL DEFAULT uuid_generate_v4(),
    "project_id" uuid NOT NULL,
    "user_id" uuid NOT NULL,
    "device_id" text NOT NULL,
    "token" varchar(255),
    "os" varchar(255),
    "os_version" varchar(255),
    "model" varchar(255),
    "app_version" varchar(255),
    "app_build" varchar(255),
    "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY ("id")
);

-- Table Definition
CREATE TABLE "entity_tags" (
    "id" uuid NOT NULL DEFAULT uuid_generate_v4(),
    "tag_id" uuid NOT NULL,
    "entity" varchar(255),
    "entity_id" uuid NOT NULL,
    "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY ("id")
);

-- Table Definition
CREATE TABLE "images" (
    "id" uuid NOT NULL DEFAULT uuid_generate_v4(),
    "project_id" uuid NOT NULL,
    "name" varchar(255) DEFAULT ''::character varying,
    "original_name" varchar(255),
    "extension" varchar(255),
    "alt" varchar(255),
    "file_size" int4,
    "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY ("id")
);

-- Table Definition
CREATE TABLE "journey_steps" (
    "id" uuid NOT NULL DEFAULT uuid_generate_v4(),
    "type" varchar(255) DEFAULT ''::character varying,
    "journey_id" uuid,
    "child_id" uuid,
    "data" jsonb,
    "x" float8 NOT NULL DEFAULT 0,
    "y" float8 NOT NULL DEFAULT 0,
    "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "external_id" varchar(128),
    "data_key" varchar(255),
    "stats" jsonb,
    "stats_at" timestamptz,
    "name" varchar(128),
    "next_scheduled_at" timestamptz,
    PRIMARY KEY ("id")
);

-- Table Definition
CREATE TABLE "journey_step_child" (
    "id" uuid NOT NULL DEFAULT uuid_generate_v4(),
    "step_id" uuid NOT NULL,
    "child_id" uuid NOT NULL,
    "data" jsonb,
    "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "priority" int4 NOT NULL DEFAULT 0,
    "path" varchar(128),
    PRIMARY KEY ("id")
);

-- Table Definition
CREATE TABLE "journey_user_step" (
    "id" uuid NOT NULL DEFAULT uuid_generate_v4(),
    "user_id" uuid,
    "journey_id" uuid,
    "step_id" uuid,
    "type" varchar(255),
    "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "delay_until" timestamptz,
    "entrance_id" uuid,
    "ended_at" timestamptz,
    "data" jsonb,
    "ref" varchar(64),
    PRIMARY KEY ("id")
);

-- Table Definition
CREATE TABLE "journeys" (
    "id" uuid NOT NULL DEFAULT uuid_generate_v4(),
    "project_id" uuid NOT NULL,
    "name" varchar(255) DEFAULT ''::character varying,
    "description" varchar(2048) DEFAULT ''::character varying,
    "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "deleted_at" timestamptz,
    "stats" jsonb,
    "stats_at" timestamptz,
    "parent_id" uuid,
    "status" varchar(255),
    PRIMARY KEY ("id")
);

-- Table Definition
CREATE TABLE "lists" (
    "id" uuid NOT NULL DEFAULT uuid_generate_v4(),
    "project_id" uuid NOT NULL,
    "name" varchar(255) DEFAULT ''::character varying,
    "type" varchar(25),
    "state" varchar(25),
    "rule" jsonb,
    "rule_id" uuid,
    "users_count" int4,
    "version" int4 NOT NULL DEFAULT 0,
    "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "deleted_at" timestamptz,
    "is_visible" bool DEFAULT true,
    "refreshed_at" timestamptz,
    PRIMARY KEY ("id")
);

-- Table Definition
CREATE TABLE "locales" (
    "id" uuid NOT NULL DEFAULT uuid_generate_v4(),
    "project_id" uuid NOT NULL,
    "key" varchar(255),
    "label" varchar(255),
    "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY ("id")
);

-- Table Definition
CREATE TABLE "notifications" (
    "id" uuid NOT NULL DEFAULT uuid_generate_v4(),
    "project_id" uuid NOT NULL,
    "user_id" uuid NOT NULL,
    "content_type" varchar(255),
    "content" jsonb,
    "read_at" timestamptz,
    "expires_at" timestamptz,
    "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY ("id")
);

-- Table Definition
CREATE TABLE "project_admins" (
    "id" uuid NOT NULL DEFAULT uuid_generate_v4(),
    "project_id" uuid NOT NULL,
    "admin_id" uuid NOT NULL,
    "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "deleted_at" timestamptz,
    "role" varchar(64) NOT NULL DEFAULT 'support'::character varying,
    PRIMARY KEY ("id")
);

-- Table Definition
CREATE TABLE "project_api_keys" (
    "id" uuid NOT NULL DEFAULT uuid_generate_v4(),
    "project_id" uuid NOT NULL,
    "value" varchar(255) NOT NULL,
    "scope" varchar(20),
    "name" varchar(255) NOT NULL,
    "description" varchar(2048),
    "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "deleted_at" timestamptz,
    "role" varchar(64) NOT NULL DEFAULT 'support'::character varying,
    PRIMARY KEY ("id")
);

-- Table Definition
CREATE TABLE "users" (
    "id" uuid NOT NULL DEFAULT uuid_generate_v4(),
    "project_id" uuid NOT NULL,
    "anonymous_id" varchar(255),
    "external_id" varchar(255),
    "email" varchar(255),
    "phone" varchar(64),
    "data" jsonb,
    "devices" jsonb,
    "timezone" varchar(50),
    "locale" varchar(255),
    "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "unsubscribe_ids" _uuid NOT NULL DEFAULT '{}'::uuid[],
    "version" int4 NOT NULL DEFAULT 0,
    "has_push_device" bool DEFAULT false,
    PRIMARY KEY ("id")
);

-- Table Definition
CREATE TABLE "subscriptions" (
    "id" uuid NOT NULL DEFAULT uuid_generate_v4(),
    "project_id" uuid NOT NULL,
    "name" varchar(255) DEFAULT ''::character varying,
    "channel" varchar(255) NOT NULL,
    "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "is_public" bool NOT NULL DEFAULT true,
    PRIMARY KEY ("id")
);

-- Table Definition
CREATE TABLE "tags" (
    "id" uuid NOT NULL DEFAULT uuid_generate_v4(),
    "project_id" uuid NOT NULL,
    "name" varchar(255) DEFAULT ''::character varying,
    "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY ("id")
);

-- Table Definition
CREATE TABLE "rules" (
    "id" uuid NOT NULL DEFAULT uuid_generate_v4(),
    "project_id" uuid NOT NULL,
    "root_id" uuid,
    "parent_id" uuid,
    "type" varchar(255),
    "group" varchar(255),
    "path" varchar(255),
    "operator" varchar(255),
    "value" varchar(255),
    "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY ("id")
);

DROP TYPE IF EXISTS "project_rule_paths_visibility";
CREATE TYPE "project_rule_paths_visibility" AS ENUM ('public', 'hidden', 'classified');

-- Table Definition
CREATE TABLE "project_rule_paths" (
    "id" uuid NOT NULL DEFAULT uuid_generate_v4(),
    "project_id" uuid NOT NULL,
    "path" varchar(255) NOT NULL,
    "name" varchar(255),
    "type" varchar(50) NOT NULL,
    "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "data_type" varchar(255) DEFAULT 'string'::character varying,
    "visibility" "project_rule_paths_visibility" NOT NULL DEFAULT 'public'::project_rule_paths_visibility,
    PRIMARY KEY ("id")
);

-- Table Definition
CREATE TABLE "resources" (
    "id" uuid NOT NULL DEFAULT uuid_generate_v4(),
    "project_id" uuid NOT NULL,
    "type" varchar(255),
    "name" varchar(255),
    "value" jsonb,
    "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY ("id")
);

-- Table Definition
CREATE TABLE "rule_evaluations" (
    "id" uuid NOT NULL DEFAULT uuid_generate_v4(),
    "rule_id" uuid NOT NULL,
    "user_id" uuid NOT NULL,
    "result" bool,
    "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY ("id")
);

-- Table Definition
CREATE TABLE "templates" (
    "id" uuid NOT NULL DEFAULT uuid_generate_v4(),
    "project_id" uuid NOT NULL,
    "type" varchar(50),
    "data" jsonb NOT NULL DEFAULT '{}'::jsonb,
    "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "locale" varchar(50),
    "campaign_id" uuid NOT NULL,
    PRIMARY KEY ("id")
);

-- Table Definition
CREATE TABLE "user_events" (
    "id" uuid NOT NULL DEFAULT uuid_generate_v4(),
    "name" varchar(255) DEFAULT ''::character varying,
    "project_id" uuid NOT NULL,
    "user_id" uuid NOT NULL,
    "data" jsonb,
    "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY ("id")
);

-- Table Definition
CREATE TABLE "user_list" (
    "id" uuid NOT NULL DEFAULT uuid_generate_v4(),
    "user_id" uuid NOT NULL,
    "list_id" uuid NOT NULL,
    "event_id" uuid,
    "version" int4 NOT NULL DEFAULT 0,
    "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "deleted_at" timestamptz,
    PRIMARY KEY ("id")
);

-- Table Definition
CREATE TABLE "user_subscription" (
    "id" uuid NOT NULL DEFAULT uuid_generate_v4(),
    "subscription_id" uuid NOT NULL,
    "user_id" uuid NOT NULL,
    "state" int2,
    "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY ("id")
);

-- Table Definition
CREATE TABLE "organizations" (
    "id" uuid NOT NULL DEFAULT uuid_generate_v4(),
    "auth" jsonb,
    "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "tracking_deeplink_mirror_url" varchar(255),
    "notification_provider_id" uuid,
    "name" varchar(255) NOT NULL DEFAULT ''::character varying,
    PRIMARY KEY ("id")
);

-- Table Definition
CREATE TABLE "admins" (
    "id" uuid NOT NULL DEFAULT uuid_generate_v4(),
    "organization_id" uuid,
    "first_name" varchar(255),
    "last_name" varchar(255),
    "email" varchar(255) NOT NULL,
    "image_url" varchar(255),
    "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "deleted_at" timestamptz,
    "role" varchar(64) NOT NULL DEFAULT 'member'::character varying,
    "external_id" varchar(255),
    PRIMARY KEY ("id")
);

-- Table Definition
CREATE TABLE "providers" (
    "id" uuid NOT NULL DEFAULT uuid_generate_v4(),
    "project_id" uuid NOT NULL,
    "type" varchar(255) NOT NULL,
    "group" varchar(255) NOT NULL,
    "data" jsonb,
    "is_default" bool DEFAULT false,
    "rate_limit" int4,
    "rate_interval" varchar(12) DEFAULT 'second'::character varying,
    "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "name" varchar(255) NOT NULL,
    "deleted_at" timestamptz,
    "external_id" varchar(255),
    PRIMARY KEY ("id")
);

-- Table Definition
CREATE TABLE "projects" (
    "id" uuid NOT NULL DEFAULT uuid_generate_v4(),
    "organization_id" uuid,
    "name" varchar(255) DEFAULT ''::character varying,
    "description" varchar(2048) DEFAULT ''::character varying,
    "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "deleted_at" timestamptz,
    "timezone" varchar(50),
    "text_opt_out_message" varchar(255),
    "link_wrap_email" bool DEFAULT false,
    "text_help_message" varchar(255),
    "link_wrap_push" bool DEFAULT false,
    "tools" _text,
    "locale" text,
    PRIMARY KEY ("id")
);

ALTER TABLE "access_tokens" ADD FOREIGN KEY ("admin_id") REFERENCES "admins"("id") ON DELETE CASCADE;


-- Indices
CREATE INDEX access_tokens_token_idx ON public.access_tokens USING btree (token);
CREATE INDEX access_tokens_admin_id_idx ON public.access_tokens USING btree (admin_id);
ALTER TABLE "audits" ADD FOREIGN KEY ("admin_id") REFERENCES "admins"("id") ON DELETE CASCADE;
ALTER TABLE "audits" ADD FOREIGN KEY ("project_id") REFERENCES "projects"("id") ON DELETE CASCADE;


-- Indices
CREATE INDEX audits_project_id_idx ON public.audits USING btree (project_id);
CREATE INDEX audits_admin_id_idx ON public.audits USING btree (admin_id);
CREATE INDEX audits_event_idx ON public.audits USING btree (event);
CREATE INDEX audits_item_type_item_id_idx ON public.audits USING btree (item_type, item_id);
CREATE INDEX audits_created_at_idx ON public.audits USING btree (created_at);


-- Indices
CREATE UNIQUE INDEX job_locks_key_uniq ON public.job_locks USING btree (key);
ALTER TABLE "campaigns" ADD FOREIGN KEY ("provider_id") REFERENCES "providers"("id") ON DELETE CASCADE;
ALTER TABLE "campaigns" ADD FOREIGN KEY ("project_id") REFERENCES "projects"("id") ON DELETE CASCADE;
ALTER TABLE "campaigns" ADD FOREIGN KEY ("subscription_id") REFERENCES "subscriptions"("id") ON DELETE CASCADE;


-- Indices
CREATE INDEX campaigns_project_id_idx ON public.campaigns USING btree (project_id);
CREATE INDEX campaigns_subscription_id_idx ON public.campaigns USING btree (subscription_id);
CREATE INDEX campaigns_send_at_idx ON public.campaigns USING btree (send_at);
CREATE INDEX campaigns_project_state_idx ON public.campaigns USING btree (project_id, state);
CREATE INDEX campaigns_provider_id_idx ON public.campaigns USING btree (provider_id);
ALTER TABLE "campaign_sends" ADD FOREIGN KEY ("campaign_id") REFERENCES "campaigns"("id") ON DELETE CASCADE;
ALTER TABLE "campaign_sends" ADD FOREIGN KEY ("user_id") REFERENCES "users"("id") ON DELETE CASCADE;


-- Indices
CREATE INDEX campaign_sends_user_id_idx ON public.campaign_sends USING btree (user_id);
CREATE INDEX campaign_sends_send_at_idx ON public.campaign_sends USING btree (send_at);
CREATE INDEX campaign_sends_state_idx ON public.campaign_sends USING btree (state);
CREATE INDEX campaign_sends_campaign_state_idx ON public.campaign_sends USING btree (campaign_id, state);
ALTER TABLE "devices" ADD FOREIGN KEY ("project_id") REFERENCES "projects"("id") ON DELETE CASCADE;
ALTER TABLE "devices" ADD FOREIGN KEY ("user_id") REFERENCES "users"("id") ON DELETE CASCADE;


-- Indices
CREATE UNIQUE INDEX devices_project_device_uniq ON public.devices USING btree (project_id, device_id);
CREATE UNIQUE INDEX devices_project_token_uniq ON public.devices USING btree (project_id, token);
CREATE INDEX devices_user_id_idx ON public.devices USING btree (user_id);
ALTER TABLE "entity_tags" ADD FOREIGN KEY ("tag_id") REFERENCES "tags"("id") ON DELETE CASCADE;


-- Indices
CREATE INDEX entity_tags_tag_id_idx ON public.entity_tags USING btree (tag_id);
CREATE INDEX entity_tags_entity_entity_id_idx ON public.entity_tags USING btree (entity, entity_id);
ALTER TABLE "images" ADD FOREIGN KEY ("project_id") REFERENCES "projects"("id") ON DELETE CASCADE;


-- Indices
CREATE INDEX images_project_id_idx ON public.images USING btree (project_id);
CREATE UNIQUE INDEX images_project_uuid_uniq ON public.images USING btree (project_id, id);
ALTER TABLE "journey_steps" ADD FOREIGN KEY ("child_id") REFERENCES "journey_steps"("id") ON DELETE SET NULL;
ALTER TABLE "journey_steps" ADD FOREIGN KEY ("journey_id") REFERENCES "journeys"("id") ON DELETE CASCADE;


-- Indices
CREATE INDEX journey_steps_child_id_idx ON public.journey_steps USING btree (child_id);
CREATE UNIQUE INDEX journey_steps_journey_external_uniq ON public.journey_steps USING btree (journey_id, external_id);
ALTER TABLE "journey_step_child" ADD FOREIGN KEY ("step_id") REFERENCES "journey_steps"("id") ON DELETE CASCADE;
ALTER TABLE "journey_step_child" ADD FOREIGN KEY ("child_id") REFERENCES "journey_steps"("id") ON DELETE CASCADE;


-- Indices
CREATE INDEX journey_step_child_child_id_idx ON public.journey_step_child USING btree (child_id);
CREATE UNIQUE INDEX journey_step_child_step_child_uniq ON public.journey_step_child USING btree (step_id, child_id);
ALTER TABLE "journey_user_step" ADD FOREIGN KEY ("journey_id") REFERENCES "journeys"("id") ON DELETE CASCADE;
ALTER TABLE "journey_user_step" ADD FOREIGN KEY ("entrance_id") REFERENCES "journey_user_step"("id") ON DELETE CASCADE;
ALTER TABLE "journey_user_step" ADD FOREIGN KEY ("step_id") REFERENCES "journey_steps"("id") ON DELETE CASCADE;
ALTER TABLE "journey_user_step" ADD FOREIGN KEY ("user_id") REFERENCES "users"("id") ON DELETE CASCADE;


-- Indices
CREATE INDEX journey_user_step_step_id_idx ON public.journey_user_step USING btree (step_id);
CREATE INDEX journey_user_step_entrance_id_idx ON public.journey_user_step USING btree (entrance_id);
CREATE INDEX journey_user_step_user_id_idx ON public.journey_user_step USING btree (user_id);
CREATE INDEX journey_user_step_journey_type_delay_idx ON public.journey_user_step USING btree (journey_id, type, delay_until);
CREATE INDEX journey_user_step_ref_idx ON public.journey_user_step USING btree (ref);
CREATE INDEX journey_user_step_delay_until_notnull_idx ON public.journey_user_step USING btree (delay_until) WHERE (delay_until IS NOT NULL);
CREATE INDEX journey_user_step_user_journey_idx ON public.journey_user_step USING btree (user_id, journey_id);
CREATE INDEX user_journey_step_entrance_id_index ON public.journey_user_step USING btree (entrance_id);
CREATE INDEX user_journey_step_journey_id_index ON public.journey_user_step USING btree (journey_id);
ALTER TABLE "journeys" ADD FOREIGN KEY ("parent_id") REFERENCES "journeys"("id") ON DELETE CASCADE;
ALTER TABLE "journeys" ADD FOREIGN KEY ("project_id") REFERENCES "projects"("id") ON DELETE CASCADE;


-- Indices
CREATE INDEX journeys_project_id_idx ON public.journeys USING btree (project_id);
CREATE INDEX journeys_parent_id_idx ON public.journeys USING btree (parent_id);
ALTER TABLE "lists" ADD FOREIGN KEY ("project_id") REFERENCES "projects"("id") ON DELETE CASCADE;
ALTER TABLE "lists" ADD FOREIGN KEY ("rule_id") REFERENCES "rules"("id") ON DELETE SET NULL;


-- Indices
CREATE INDEX lists_project_id_idx ON public.lists USING btree (project_id);
CREATE INDEX lists_rule_id_idx ON public.lists USING btree (rule_id);
CREATE INDEX lists_project_active_idx ON public.lists USING btree (project_id) WHERE (deleted_at IS NULL);
ALTER TABLE "locales" ADD FOREIGN KEY ("project_id") REFERENCES "projects"("id") ON DELETE CASCADE;


-- Indices
CREATE INDEX locales_project_id_idx ON public.locales USING btree (project_id);
ALTER TABLE "notifications" ADD FOREIGN KEY ("project_id") REFERENCES "projects"("id") ON DELETE CASCADE;
ALTER TABLE "notifications" ADD FOREIGN KEY ("user_id") REFERENCES "users"("id") ON DELETE CASCADE;


-- Indices
CREATE INDEX notifications_user_id_idx ON public.notifications USING btree (user_id);
CREATE INDEX notifications_project_id_idx ON public.notifications USING btree (project_id);
CREATE INDEX notifications_read_at_idx ON public.notifications USING btree (read_at);
CREATE INDEX notifications_expires_at_idx ON public.notifications USING btree (expires_at);
CREATE INDEX notifications_user_unread_idx ON public.notifications USING btree (user_id) WHERE (read_at IS NULL);
ALTER TABLE "project_admins" ADD FOREIGN KEY ("admin_id") REFERENCES "admins"("id") ON DELETE CASCADE;
ALTER TABLE "project_admins" ADD FOREIGN KEY ("project_id") REFERENCES "projects"("id") ON DELETE CASCADE;


-- Indices
CREATE INDEX project_admins_project_id_idx ON public.project_admins USING btree (project_id);
CREATE INDEX project_admins_admin_id_idx ON public.project_admins USING btree (admin_id);
ALTER TABLE "project_api_keys" ADD FOREIGN KEY ("project_id") REFERENCES "projects"("id") ON DELETE CASCADE;


-- Indices
CREATE UNIQUE INDEX project_api_keys_value_uniq ON public.project_api_keys USING btree (value);
CREATE INDEX project_api_keys_project_id_idx ON public.project_api_keys USING btree (project_id);
ALTER TABLE "users" ADD FOREIGN KEY ("project_id") REFERENCES "projects"("id") ON DELETE CASCADE;


-- Indices
CREATE UNIQUE INDEX users_project_anonymous_uniq ON public.users USING btree (project_id, anonymous_id);
CREATE UNIQUE INDEX users_project_external_uniq ON public.users USING btree (project_id, external_id);
CREATE INDEX users_email_idx ON public.users USING btree (email);
CREATE INDEX users_phone_idx ON public.users USING btree (phone);
CREATE INDEX users_external_id_idx ON public.users USING btree (external_id);
CREATE INDEX users_updated_at_idx ON public.users USING btree (updated_at);
CREATE INDEX users_anonymous_id_idx ON public.users USING btree (anonymous_id);
CREATE INDEX users_data_idx ON public.users USING gin (data);
ALTER TABLE "subscriptions" ADD FOREIGN KEY ("project_id") REFERENCES "projects"("id") ON DELETE CASCADE;


-- Indices
CREATE INDEX subscriptions_project_id_idx ON public.subscriptions USING btree (project_id);
ALTER TABLE "tags" ADD FOREIGN KEY ("project_id") REFERENCES "projects"("id") ON DELETE CASCADE;


-- Indices
CREATE INDEX tags_project_id_idx ON public.tags USING btree (project_id);
ALTER TABLE "rules" ADD FOREIGN KEY ("project_id") REFERENCES "projects"("id") ON DELETE CASCADE;
ALTER TABLE "rules" ADD FOREIGN KEY ("root_id") REFERENCES "rules"("id") ON DELETE CASCADE;
ALTER TABLE "rules" ADD FOREIGN KEY ("parent_id") REFERENCES "rules"("id") ON DELETE CASCADE;


-- Indices
CREATE UNIQUE INDEX rules_uuid_key ON public.rules USING btree (uuid);
CREATE INDEX rules_project_id_idx ON public.rules USING btree (project_id);
CREATE INDEX rules_parent_id_idx ON public.rules USING btree (parent_id);
CREATE INDEX rules_root_id_idx ON public.rules USING btree (root_id);
CREATE INDEX rules_group_idx ON public.rules USING btree ("group");
CREATE INDEX rules_type_idx ON public.rules USING btree (type);
CREATE INDEX rules_value_idx ON public.rules USING btree (value);
ALTER TABLE "project_rule_paths" ADD FOREIGN KEY ("project_id") REFERENCES "projects"("id") ON DELETE CASCADE;


-- Indices
CREATE INDEX project_rule_paths_project_id_idx ON public.project_rule_paths USING btree (project_id);


-- Indices
CREATE INDEX resources_project_id_idx ON public.resources USING btree (project_id);
ALTER TABLE "rule_evaluations" ADD FOREIGN KEY ("rule_id") REFERENCES "rules"("id") ON DELETE CASCADE;
ALTER TABLE "rule_evaluations" ADD FOREIGN KEY ("user_id") REFERENCES "users"("id") ON DELETE CASCADE;


-- Indices
CREATE INDEX rule_evaluations_rule_id_idx ON public.rule_evaluations USING btree (rule_id);
CREATE UNIQUE INDEX rule_evaluations_user_rule_uniq ON public.rule_evaluations USING btree (user_id, rule_id);
ALTER TABLE "templates" ADD FOREIGN KEY ("project_id") REFERENCES "projects"("id") ON DELETE CASCADE;
ALTER TABLE "templates" ADD FOREIGN KEY ("campaign_id") REFERENCES "campaigns"("id") ON DELETE CASCADE;


-- Indices
CREATE INDEX templates_campaign_id_idx ON public.templates USING btree (campaign_id);
CREATE INDEX templates_project_id_idx ON public.templates USING btree (project_id);
ALTER TABLE "user_events" ADD FOREIGN KEY ("user_id") REFERENCES "users"("id") ON DELETE CASCADE;
ALTER TABLE "user_events" ADD FOREIGN KEY ("project_id") REFERENCES "projects"("id") ON DELETE CASCADE;


-- Indices
CREATE INDEX user_events_project_id_idx ON public.user_events USING btree (project_id);
CREATE INDEX user_events_user_id_idx ON public.user_events USING btree (user_id);
CREATE INDEX user_events_created_at_idx ON public.user_events USING btree (created_at);
CREATE INDEX user_events_name_user_idx ON public.user_events USING btree (name, user_id);
CREATE INDEX user_events_data_idx ON public.user_events USING gin (data);
ALTER TABLE "user_list" ADD FOREIGN KEY ("list_id") REFERENCES "lists"("id") ON DELETE CASCADE;
ALTER TABLE "user_list" ADD FOREIGN KEY ("event_id") REFERENCES "user_events"("id") ON DELETE CASCADE;
ALTER TABLE "user_list" ADD FOREIGN KEY ("user_id") REFERENCES "users"("id") ON DELETE CASCADE;


-- Indices
CREATE UNIQUE INDEX user_list_user_list_uniq ON public.user_list USING btree (user_id, list_id);
CREATE INDEX user_list_version_idx ON public.user_list USING btree (version);
CREATE INDEX user_list_list_id_idx ON public.user_list USING btree (list_id);
CREATE INDEX user_list_event_id_idx ON public.user_list USING btree (event_id);
CREATE INDEX user_list_created_at_idx ON public.user_list USING btree (created_at);
ALTER TABLE "user_subscription" ADD FOREIGN KEY ("user_id") REFERENCES "users"("id") ON DELETE CASCADE;
ALTER TABLE "user_subscription" ADD FOREIGN KEY ("subscription_id") REFERENCES "subscriptions"("id") ON DELETE CASCADE;


-- Indices
CREATE INDEX user_subscription_user_id_idx ON public.user_subscription USING btree (user_id);
CREATE INDEX user_subscription_subscription_id_idx ON public.user_subscription USING btree (subscription_id);
ALTER TABLE "organizations" ADD FOREIGN KEY ("notification_provider_id") REFERENCES "providers"("id") ON DELETE SET NULL;


-- Indices
CREATE INDEX organizations_notification_provider_id_idx ON public.organizations USING btree (notification_provider_id);
ALTER TABLE "admins" ADD FOREIGN KEY ("organization_id") REFERENCES "organizations"("id") ON DELETE CASCADE;


-- Indices
CREATE INDEX admins_organization_id_idx ON public.admins USING btree (organization_id);
ALTER TABLE "providers" ADD FOREIGN KEY ("project_id") REFERENCES "projects"("id") ON DELETE CASCADE;


-- Indices
CREATE INDEX providers_project_id_idx ON public.providers USING btree (project_id);
ALTER TABLE "projects" ADD FOREIGN KEY ("organization_id") REFERENCES "organizations"("id") ON DELETE CASCADE;


-- Indices
CREATE INDEX projects_organization_id_idx ON public.projects USING btree (organization_id);
CREATE INDEX projects_name_idx ON public.projects USING btree (name);

-- Triggers for updated_at
CREATE TRIGGER set_updated_at_access_tokens BEFORE UPDATE ON access_tokens FOR EACH ROW EXECUTE PROCEDURE set_updated_at();
CREATE TRIGGER set_updated_at_admins BEFORE UPDATE ON admins FOR EACH ROW EXECUTE PROCEDURE set_updated_at();
CREATE TRIGGER set_updated_at_audits BEFORE UPDATE ON audits FOR EACH ROW EXECUTE PROCEDURE set_updated_at();
CREATE TRIGGER set_updated_at_campaign_sends BEFORE UPDATE ON campaign_sends FOR EACH ROW EXECUTE PROCEDURE set_updated_at();
CREATE TRIGGER set_updated_at_campaigns BEFORE UPDATE ON campaigns FOR EACH ROW EXECUTE PROCEDURE set_updated_at();
CREATE TRIGGER set_updated_at_devices BEFORE UPDATE ON devices FOR EACH ROW EXECUTE PROCEDURE set_updated_at();
CREATE TRIGGER set_updated_at_entity_tags BEFORE UPDATE ON entity_tags FOR EACH ROW EXECUTE PROCEDURE set_updated_at();
CREATE TRIGGER set_updated_at_images BEFORE UPDATE ON images FOR EACH ROW EXECUTE PROCEDURE set_updated_at();
CREATE TRIGGER set_updated_at_job_locks BEFORE UPDATE ON job_locks FOR EACH ROW EXECUTE PROCEDURE set_updated_at();
CREATE TRIGGER set_updated_at_journey_step_child BEFORE UPDATE ON journey_step_child FOR EACH ROW EXECUTE PROCEDURE set_updated_at();
CREATE TRIGGER set_updated_at_journey_steps BEFORE UPDATE ON journey_steps FOR EACH ROW EXECUTE PROCEDURE set_updated_at();
CREATE TRIGGER set_updated_at_journey_user_step BEFORE UPDATE ON journey_user_step FOR EACH ROW EXECUTE PROCEDURE set_updated_at();
CREATE TRIGGER set_updated_at_journeys BEFORE UPDATE ON journeys FOR EACH ROW EXECUTE PROCEDURE set_updated_at();
CREATE TRIGGER set_updated_at_lists BEFORE UPDATE ON lists FOR EACH ROW EXECUTE PROCEDURE set_updated_at();
CREATE TRIGGER set_updated_at_locales BEFORE UPDATE ON locales FOR EACH ROW EXECUTE PROCEDURE set_updated_at();
CREATE TRIGGER set_updated_at_notifications BEFORE UPDATE ON notifications FOR EACH ROW EXECUTE PROCEDURE set_updated_at();
CREATE TRIGGER set_updated_at_organizations BEFORE UPDATE ON organizations FOR EACH ROW EXECUTE PROCEDURE set_updated_at();
CREATE TRIGGER set_updated_at_project_admins BEFORE UPDATE ON project_admins FOR EACH ROW EXECUTE PROCEDURE set_updated_at();
CREATE TRIGGER set_updated_at_project_api_keys BEFORE UPDATE ON project_api_keys FOR EACH ROW EXECUTE PROCEDURE set_updated_at();
CREATE TRIGGER set_updated_at_project_rule_paths BEFORE UPDATE ON project_rule_paths FOR EACH ROW EXECUTE PROCEDURE set_updated_at();
CREATE TRIGGER set_updated_at_projects BEFORE UPDATE ON projects FOR EACH ROW EXECUTE PROCEDURE set_updated_at();
CREATE TRIGGER set_updated_at_providers BEFORE UPDATE ON providers FOR EACH ROW EXECUTE PROCEDURE set_updated_at();
CREATE TRIGGER set_updated_at_resources BEFORE UPDATE ON resources FOR EACH ROW EXECUTE PROCEDURE set_updated_at();
CREATE TRIGGER set_updated_at_rule_evaluations BEFORE UPDATE ON rule_evaluations FOR EACH ROW EXECUTE PROCEDURE set_updated_at();
CREATE TRIGGER set_updated_at_rules BEFORE UPDATE ON rules FOR EACH ROW EXECUTE PROCEDURE set_updated_at();
CREATE TRIGGER set_updated_at_subscriptions BEFORE UPDATE ON subscriptions FOR EACH ROW EXECUTE PROCEDURE set_updated_at();
CREATE TRIGGER set_updated_at_tags BEFORE UPDATE ON tags FOR EACH ROW EXECUTE PROCEDURE set_updated_at();
CREATE TRIGGER set_updated_at_templates BEFORE UPDATE ON templates FOR EACH ROW EXECUTE PROCEDURE set_updated_at();
CREATE TRIGGER set_updated_at_user_events BEFORE UPDATE ON user_events FOR EACH ROW EXECUTE PROCEDURE set_updated_at();
CREATE TRIGGER set_updated_at_user_list BEFORE UPDATE ON user_list FOR EACH ROW EXECUTE PROCEDURE set_updated_at();
CREATE TRIGGER set_updated_at_user_subscription BEFORE UPDATE ON user_subscription FOR EACH ROW EXECUTE PROCEDURE set_updated_at();
CREATE TRIGGER set_updated_at_users BEFORE UPDATE ON users FOR EACH ROW EXECUTE PROCEDURE set_updated_at();
