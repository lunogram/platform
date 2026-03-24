-- Remove paused_at column from user_schedules and organization_schedules.

ALTER TABLE user_schedules DROP COLUMN IF EXISTS paused_at;
ALTER TABLE organization_schedules DROP COLUMN IF EXISTS paused_at;
