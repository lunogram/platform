-- Add paused_at column to user_schedules and organization_schedules.
-- paused_at is NULL when the schedule is active. When set to a non-NULL
-- timestamp, the schedule is considered paused: the scheduler will not
-- advance it to the next occurrence and (depending on pause mode) pending
-- events may be deleted.

ALTER TABLE user_schedules
    ADD COLUMN IF NOT EXISTS paused_at TIMESTAMPTZ;

ALTER TABLE organization_schedules
    ADD COLUMN IF NOT EXISTS paused_at TIMESTAMPTZ;
