-- Drop existing journey tables
DROP TABLE IF EXISTS journey_step_events CASCADE;
DROP TABLE IF EXISTS journey_user_step CASCADE;
DROP TABLE IF EXISTS journey_step_child CASCADE;
DROP TABLE IF EXISTS journey_steps CASCADE;
DROP TABLE IF EXISTS journeys CASCADE;

-- Create journey version status enum
CREATE TYPE journey_version_status AS ENUM ('draft', 'published', 'archived');

-- Create journey user status enum
CREATE TYPE journey_user_status AS ENUM ('active', 'completed', 'exited');

-- Create journeys table (main journey entity)
CREATE TABLE journeys (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    description TEXT,
    version_id UUID, -- points to current published version
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX idx_journeys_project_id ON journeys(project_id);
CREATE INDEX idx_journeys_version_id ON journeys(version_id);
CREATE INDEX idx_journeys_deleted_at ON journeys(deleted_at) WHERE deleted_at IS NULL;

-- Create journey versions table
CREATE TABLE journey_versions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    journey_id UUID NOT NULL REFERENCES journeys(id) ON DELETE CASCADE,
    version_number INT NOT NULL,
    status journey_version_status NOT NULL DEFAULT 'draft',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at TIMESTAMPTZ,
    UNIQUE (journey_id, version_number)
);

CREATE INDEX idx_journey_versions_journey_id ON journey_versions(journey_id);
CREATE INDEX idx_journey_versions_status ON journey_versions(status);

-- Create journey version steps table
CREATE TABLE journey_version_steps (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    version_id UUID NOT NULL REFERENCES journey_versions(id) ON DELETE CASCADE,
    external_id TEXT NOT NULL,
    type TEXT NOT NULL,
    name TEXT,
    data JSONB,
    data_key TEXT,
    x FLOAT NOT NULL DEFAULT 0,
    y FLOAT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (version_id, external_id)
);

CREATE INDEX idx_journey_version_steps_version_id ON journey_version_steps(version_id);
CREATE INDEX idx_journey_version_steps_external_id ON journey_version_steps(external_id);
CREATE INDEX idx_journey_version_steps_type ON journey_version_steps(type);

-- Create journey version step children table (edges)
CREATE TABLE journey_version_step_children (
    version_id UUID NOT NULL REFERENCES journey_versions(id) ON DELETE CASCADE,
    parent_external_id TEXT NOT NULL,
    child_external_id TEXT NOT NULL,
    path TEXT,
    data JSONB,
    priority INT NOT NULL DEFAULT 0,
    PRIMARY KEY (version_id, parent_external_id, child_external_id)
);

CREATE INDEX idx_journey_version_step_children_version_id ON journey_version_step_children(version_id);
CREATE INDEX idx_journey_version_step_children_parent ON journey_version_step_children(parent_external_id);
CREATE INDEX idx_journey_version_step_children_child ON journey_version_step_children(child_external_id);

-- Create journey version step events table (event dependencies)
CREATE TABLE journey_version_step_events (
    version_id UUID NOT NULL REFERENCES journey_versions(id) ON DELETE CASCADE,
    external_id TEXT NOT NULL,
    event_id UUID NOT NULL REFERENCES events(id) ON DELETE RESTRICT,
    PRIMARY KEY (version_id, external_id, event_id)
);

CREATE INDEX idx_journey_version_step_events_version_id ON journey_version_step_events(version_id);
CREATE INDEX idx_journey_version_step_events_event_id ON journey_version_step_events(event_id);
CREATE INDEX idx_journey_version_step_events_external_id ON journey_version_step_events(external_id);

-- Create journey user state table (current position per user)
CREATE TABLE journey_user_state (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    journey_id UUID NOT NULL REFERENCES journeys(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    pinned_version_id UUID REFERENCES journey_versions(id) ON DELETE SET NULL,
    external_id TEXT,
    type TEXT,
    entered_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    resume_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    data JSONB,
    status journey_user_status NOT NULL DEFAULT 'active',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (journey_id, user_id)
);

CREATE INDEX idx_journey_user_state_journey_id ON journey_user_state(journey_id);
CREATE INDEX idx_journey_user_state_user_id ON journey_user_state(user_id);
CREATE INDEX idx_journey_user_state_status ON journey_user_state(status);
CREATE INDEX idx_journey_user_state_resume_at ON journey_user_state(resume_at) WHERE resume_at IS NOT NULL;
CREATE INDEX idx_journey_user_state_external_id ON journey_user_state(external_id);

-- Create journey user steps table (historical log)
CREATE TABLE journey_user_steps (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    journey_id UUID NOT NULL REFERENCES journeys(id) ON DELETE CASCADE,
    step_id UUID NOT NULL REFERENCES journey_version_steps(id) ON DELETE RESTRICT,
    entrance_id UUID REFERENCES journey_user_steps(id) ON DELETE CASCADE,
    type TEXT NOT NULL,
    external_id TEXT NOT NULL,
    entered_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    exited_at TIMESTAMPTZ,
    delay_until TIMESTAMPTZ,
    data JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_journey_user_steps_user_id ON journey_user_steps(user_id);
CREATE INDEX idx_journey_user_steps_journey_id ON journey_user_steps(journey_id);
CREATE INDEX idx_journey_user_steps_step_id ON journey_user_steps(step_id);
CREATE INDEX idx_journey_user_steps_entrance_id ON journey_user_steps(entrance_id);
CREATE INDEX idx_journey_user_steps_external_id ON journey_user_steps(external_id);
CREATE INDEX idx_journey_user_steps_entered_at ON journey_user_steps(entered_at);

-- Add foreign key constraint from journeys to journey_versions
ALTER TABLE journeys
    ADD CONSTRAINT fk_journeys_version_id
    FOREIGN KEY (version_id)
    REFERENCES journey_versions(id)
    ON DELETE SET NULL;
