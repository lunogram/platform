-- Journey Database Initial Migration
-- This database contains journey/automation tables

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Helper function for updating timestamps
CREATE OR REPLACE FUNCTION set_updated_at() RETURNS trigger
  LANGUAGE plpgsql
  AS $$
BEGIN
  new.updated_at := current_timestamp;
  return new;
END;
$$;

-- Enum types
CREATE TYPE journey_version_status AS ENUM ('draft', 'published', 'archived');

-- Journeys table (main journey entity)
CREATE TABLE journeys (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id UUID NOT NULL,
    name TEXT NOT NULL,
    description TEXT,
    version_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX idx_journeys_project_id ON journeys(project_id);
CREATE INDEX idx_journeys_version_id ON journeys(version_id);
CREATE INDEX idx_journeys_deleted_at ON journeys(deleted_at) WHERE deleted_at IS NULL;

CREATE TRIGGER set_updated_at_journeys BEFORE UPDATE ON journeys FOR EACH ROW EXECUTE PROCEDURE set_updated_at();

-- Journey versions table
CREATE TABLE journey_versions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    journey_id UUID NOT NULL REFERENCES journeys(id) ON DELETE CASCADE,
    version_number INTEGER NOT NULL,
    status journey_version_status NOT NULL DEFAULT 'draft',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at TIMESTAMPTZ,
    UNIQUE (journey_id, version_number)
);

CREATE INDEX idx_journey_versions_journey_id ON journey_versions(journey_id);
CREATE INDEX idx_journey_versions_status ON journey_versions(status);

-- Add foreign key constraint from journeys to journey_versions (circular reference)
ALTER TABLE journeys
    ADD CONSTRAINT fk_journeys_version_id
    FOREIGN KEY (version_id)
    REFERENCES journey_versions(id)
    ON DELETE SET NULL;

-- Journey version steps table
CREATE TABLE journey_version_steps (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    version_id UUID NOT NULL REFERENCES journey_versions(id) ON DELETE CASCADE,
    external_id TEXT NOT NULL,
    type TEXT NOT NULL,
    name TEXT,
    data JSONB NOT NULL DEFAULT '{}'::jsonb,
    data_key TEXT,
    x DOUBLE PRECISION NOT NULL DEFAULT 0,
    y DOUBLE PRECISION NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (version_id, external_id)
);

CREATE INDEX idx_journey_version_steps_version_id ON journey_version_steps(version_id);
CREATE INDEX idx_journey_version_steps_external_id ON journey_version_steps(external_id);
CREATE INDEX idx_journey_version_steps_type ON journey_version_steps(type);

-- Journey version step children table (edges)
CREATE TABLE journey_version_step_children (
    version_id UUID NOT NULL REFERENCES journey_versions(id) ON DELETE CASCADE,
    parent_external_id TEXT NOT NULL,
    child_external_id TEXT NOT NULL,
    path TEXT,
    data JSONB,
    PRIMARY KEY (version_id, parent_external_id, child_external_id)
);

CREATE INDEX idx_journey_version_step_children_version_id ON journey_version_step_children(version_id);
CREATE INDEX idx_journey_version_step_children_parent ON journey_version_step_children(parent_external_id);
CREATE INDEX idx_journey_version_step_children_child ON journey_version_step_children(child_external_id);

-- Journey version step events table (event dependencies)
-- Note: event_id references events table in users database, enforced at application level
CREATE TABLE journey_version_step_events (
    version_id UUID NOT NULL REFERENCES journey_versions(id) ON DELETE CASCADE,
    external_id TEXT NOT NULL,
    event_id UUID NOT NULL,
    PRIMARY KEY (version_id, external_id, event_id)
);

CREATE INDEX idx_journey_version_step_events_version_id ON journey_version_step_events(version_id);
CREATE INDEX idx_journey_version_step_events_event_id ON journey_version_step_events(event_id);
CREATE INDEX idx_journey_version_step_events_external_id ON journey_version_step_events(external_id);

-- Journey user state table (current position per user per journey entry)
-- Note: user_id references users table in users database, enforced at application level
CREATE TABLE journey_user_state (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    journey_id UUID NOT NULL REFERENCES journeys(id) ON DELETE CASCADE,
    user_id UUID NOT NULL,
    pinned_version_id UUID REFERENCES journey_versions(id) ON DELETE SET NULL,
    entered_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    resume_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    data JSONB NOT NULL DEFAULT '{}'::jsonb,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    journey_entry_id UUID NOT NULL DEFAULT uuid_generate_v4(),
    external_step_id TEXT NOT NULL DEFAULT '',
    occurrence INTEGER NOT NULL DEFAULT 1
);

CREATE INDEX idx_journey_user_state_journey_id ON journey_user_state(journey_id);
CREATE INDEX idx_journey_user_state_user_id ON journey_user_state(user_id);
CREATE INDEX idx_journey_user_state_resume_at ON journey_user_state(resume_at) WHERE resume_at IS NOT NULL;
CREATE INDEX idx_journey_user_state_entry_id ON journey_user_state(journey_entry_id);
CREATE INDEX idx_journey_user_state_external_step_id ON journey_user_state(external_step_id);
CREATE UNIQUE INDEX journey_user_state_unique ON journey_user_state(journey_entry_id, external_step_id, occurrence);
CREATE INDEX journey_user_state_latest ON journey_user_state(journey_entry_id, external_step_id, occurrence DESC);

CREATE TRIGGER set_updated_at_journey_user_state BEFORE UPDATE ON journey_user_state FOR EACH ROW EXECUTE PROCEDURE set_updated_at();
