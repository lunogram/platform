-- Drop new versioned journey tables
DROP TABLE IF EXISTS journey_user_steps CASCADE;
DROP TABLE IF EXISTS journey_user_state CASCADE;
DROP TABLE IF EXISTS journey_version_step_events CASCADE;
DROP TABLE IF EXISTS journey_version_step_children CASCADE;
DROP TABLE IF EXISTS journey_version_steps CASCADE;
DROP TABLE IF EXISTS journey_versions CASCADE;
DROP TABLE IF EXISTS journeys CASCADE;

-- Drop enums
DROP TYPE IF EXISTS journey_user_status;
DROP TYPE IF EXISTS journey_version_status;

-- Recreate old tables (for rollback)
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

CREATE TABLE journey_step_events (
    journey_step_id UUID NOT NULL REFERENCES journey_steps(id) ON DELETE CASCADE,
    event_id UUID NOT NULL REFERENCES events(id) ON DELETE RESTRICT,
    PRIMARY KEY (journey_step_id, event_id)
);

CREATE INDEX idx_journey_step_events_journey_step_id ON journey_step_events(journey_step_id);
CREATE INDEX idx_journey_step_events_event_id ON journey_step_events(event_id);
