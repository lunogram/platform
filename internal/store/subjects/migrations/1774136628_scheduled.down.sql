-- Reverse the entire scheduling feature migration.

-- Drop triggers first.
DROP TRIGGER IF EXISTS set_updated_at_organization_schedules ON organization_schedules;
DROP TRIGGER IF EXISTS set_updated_at_user_schedules ON user_schedules;
DROP TRIGGER IF EXISTS set_updated_at_schedule_offsets ON schedule_offsets;
DROP TRIGGER IF EXISTS set_updated_at_schedules ON schedules;

-- Drop tables in reverse dependency order.
DROP TABLE IF EXISTS organization_scheduled_events;
DROP TABLE IF EXISTS organization_schedules;
DROP TABLE IF EXISTS scheduled_schemas;
DROP TABLE IF EXISTS user_scheduled_events;
DROP TABLE IF EXISTS user_schedules;
DROP TABLE IF EXISTS schedule_offsets;
DROP TABLE IF EXISTS schedules;

-- Drop enum types.
DROP TYPE IF EXISTS offset_direction;
