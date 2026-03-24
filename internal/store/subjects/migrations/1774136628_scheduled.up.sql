-- Scheduling feature: tables, indexes, triggers, and enum types.
-- Merged from multiple incremental migrations into a single initial migration.

-- ============================================================
-- Enum types
-- ============================================================

CREATE TYPE offset_direction AS ENUM ('before', 'after');


-- ============================================================
-- Schedule definitions
-- ============================================================

CREATE TABLE IF NOT EXISTS schedules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    type VARCHAR(20) NOT NULL CHECK (type IN ('single', 'recurring')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_schedules_project_name
    ON schedules (project_id, name)
    WHERE deleted_at IS NULL;


-- ============================================================
-- Schedule offsets
-- ============================================================
-- Every schedule auto-gets a zero offset on creation.
-- The offset column stores a positive INTERVAL; the direction
-- column encodes whether it fires before or after the anchor.

CREATE TABLE IF NOT EXISTS schedule_offsets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    schedule_id UUID NOT NULL REFERENCES schedules(id),
    "offset" INTERVAL NOT NULL DEFAULT '0 minutes'::interval,
    direction offset_direction NOT NULL DEFAULT 'after',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_schedule_offsets_schedule
    ON schedule_offsets (schedule_id);

CREATE UNIQUE INDEX IF NOT EXISTS idx_schedule_offsets_unique
    ON schedule_offsets (schedule_id, "offset", direction);


-- ============================================================
-- Per-user schedule assignments
-- ============================================================

CREATE TABLE IF NOT EXISTS user_schedules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    schedule_id UUID NOT NULL REFERENCES schedules(id),
    scheduled_at TIMESTAMPTZ,              -- fire time for single-type schedules
    start_at TIMESTAMPTZ,                  -- start of interval for recurring (supports future dates)
    interval INTERVAL,                     -- recurrence interval, NULL for single
    occurrence INTEGER NOT NULL DEFAULT 0, -- number of intervals advanced from start_at (scheduled_at = start_at + occurrence * interval)
    data JSONB DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_user_schedules_user
    ON user_schedules (user_id, schedule_id);

CREATE INDEX IF NOT EXISTS idx_user_schedules_schedule
    ON user_schedules (schedule_id);

CREATE UNIQUE INDEX IF NOT EXISTS idx_user_schedules_user_schedule_unique
    ON user_schedules (user_id, schedule_id);


-- ============================================================
-- Pre-computed scheduled events for users
-- ============================================================
-- Rows are created when a user_schedule is created/updated.
-- The scheduler queries for unfired rows where fire_at <= NOW().

CREATE TABLE IF NOT EXISTS user_scheduled_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_schedule_id UUID NOT NULL REFERENCES user_schedules(id) ON DELETE CASCADE,
    schedule_offset_id UUID NOT NULL REFERENCES schedule_offsets(id),
    user_id UUID NOT NULL,
    schedule_id UUID NOT NULL REFERENCES schedules(id),
    fire_at TIMESTAMPTZ NOT NULL,          -- computed: scheduled_at +/- offset
    fired_at TIMESTAMPTZ,                  -- NULL = pending, non-NULL = fired
    data JSONB DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_user_scheduled_events_due
    ON user_scheduled_events (fire_at)
    WHERE fired_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_user_scheduled_events_user_schedule
    ON user_scheduled_events (user_schedule_id)
    WHERE fired_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_user_scheduled_events_user
    ON user_scheduled_events (user_id, schedule_id)
    WHERE fired_at IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_user_scheduled_events_unique
    ON user_scheduled_events (user_schedule_id, schedule_offset_id, user_id);


-- ============================================================
-- Schema tracking for scheduled data
-- ============================================================

CREATE TABLE IF NOT EXISTS scheduled_schemas (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    schedule_id UUID NOT NULL REFERENCES schedules(id),
    path VARCHAR(255) NOT NULL,
    data_type VARCHAR(50) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_scheduled_schemas_unique
    ON scheduled_schemas (schedule_id, path, data_type);


-- ============================================================
-- Per-organization schedule assignments
-- ============================================================

CREATE TABLE IF NOT EXISTS organization_schedules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id),
    schedule_id UUID NOT NULL REFERENCES schedules(id),
    scheduled_at TIMESTAMPTZ,
    start_at TIMESTAMPTZ,
    interval INTERVAL,
    occurrence INTEGER NOT NULL DEFAULT 0, -- number of intervals advanced from start_at (scheduled_at = start_at + occurrence * interval)
    data JSONB DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_organization_schedules_org
    ON organization_schedules (organization_id, schedule_id);

CREATE INDEX IF NOT EXISTS idx_organization_schedules_schedule
    ON organization_schedules (schedule_id);

CREATE UNIQUE INDEX IF NOT EXISTS idx_organization_schedules_org_schedule_unique
    ON organization_schedules (organization_id, schedule_id);


-- ============================================================
-- Pre-computed scheduled events for organizations
-- ============================================================

CREATE TABLE IF NOT EXISTS organization_scheduled_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_schedule_id UUID NOT NULL REFERENCES organization_schedules(id) ON DELETE CASCADE,
    schedule_offset_id UUID NOT NULL REFERENCES schedule_offsets(id),
    organization_id UUID NOT NULL,
    schedule_id UUID NOT NULL REFERENCES schedules(id),
    fire_at TIMESTAMPTZ NOT NULL,
    fired_at TIMESTAMPTZ,
    data JSONB DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_organization_scheduled_events_due
    ON organization_scheduled_events (fire_at)
    WHERE fired_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_organization_scheduled_events_org_schedule
    ON organization_scheduled_events (organization_schedule_id)
    WHERE fired_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_organization_scheduled_events_org
    ON organization_scheduled_events (organization_id, schedule_id)
    WHERE fired_at IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_organization_scheduled_events_unique
    ON organization_scheduled_events (organization_schedule_id, schedule_offset_id, organization_id);


-- ============================================================
-- updated_at triggers
-- ============================================================

CREATE TRIGGER set_updated_at_schedules BEFORE UPDATE ON schedules FOR EACH ROW EXECUTE PROCEDURE set_updated_at();
CREATE TRIGGER set_updated_at_schedule_offsets BEFORE UPDATE ON schedule_offsets FOR EACH ROW EXECUTE PROCEDURE set_updated_at();
CREATE TRIGGER set_updated_at_user_schedules BEFORE UPDATE ON user_schedules FOR EACH ROW EXECUTE PROCEDURE set_updated_at();
CREATE TRIGGER set_updated_at_organization_schedules BEFORE UPDATE ON organization_schedules FOR EACH ROW EXECUTE PROCEDURE set_updated_at();
