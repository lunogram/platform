-- Replace the full unique indexes on scheduled events tables with partial
-- unique indexes that only cover unfired (pending) events.
--
-- The original indexes prevented ANY duplicate (schedule, offset, subject)
-- combination, even across different occurrences. This meant that once an
-- event had fired (fired_at IS NOT NULL), subsequent occurrences — or a
-- resume — could not insert a new event for the same offset because the
-- ON CONFLICT DO NOTHING silently skipped the insert.
--
-- The new partial indexes (WHERE fired_at IS NULL) ensure we still prevent
-- duplicate *pending* events for the same offset, but fired events no
-- longer block new inserts.

DROP INDEX IF EXISTS idx_user_scheduled_events_unique;
DROP INDEX IF EXISTS idx_organization_scheduled_events_unique;

CREATE UNIQUE INDEX idx_user_scheduled_events_unique
    ON user_scheduled_events (user_schedule_id, schedule_offset_id, user_id)
    WHERE fired_at IS NULL;

CREATE UNIQUE INDEX idx_organization_scheduled_events_unique
    ON organization_scheduled_events (organization_schedule_id, schedule_offset_id, organization_id)
    WHERE fired_at IS NULL;
