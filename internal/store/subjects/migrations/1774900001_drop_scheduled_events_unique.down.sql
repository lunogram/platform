-- Restore the original (non-partial) unique indexes on scheduled events tables.

DROP INDEX IF EXISTS idx_user_scheduled_events_unique;
DROP INDEX IF EXISTS idx_organization_scheduled_events_unique;

CREATE UNIQUE INDEX IF NOT EXISTS idx_user_scheduled_events_unique
    ON user_scheduled_events (user_schedule_id, schedule_offset_id, user_id);

CREATE UNIQUE INDEX IF NOT EXISTS idx_organization_scheduled_events_unique
    ON organization_scheduled_events (organization_schedule_id, schedule_offset_id, organization_id);
