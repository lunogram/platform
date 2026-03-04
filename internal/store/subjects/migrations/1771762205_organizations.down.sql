-- Drop organization events table
DROP TABLE IF EXISTS organization_events;

-- Drop organization dependency columns from rules table
ALTER TABLE rules DROP COLUMN IF EXISTS depends_on_organization_users;
ALTER TABLE rules DROP COLUMN IF EXISTS depends_on_organizations;

-- Drop organization users table (depends on organizations)
DROP TABLE IF EXISTS organization_users;

-- Drop organizations table
DROP TABLE IF EXISTS organizations;

-- Rename subject_schemas back to user_schemas and remove subject_type column
DROP INDEX idx_subject_schemas_project_path_type;
ALTER TABLE subject_schemas DROP COLUMN IF EXISTS subject_type;
ALTER TABLE subject_schemas RENAME TO user_schemas;
CREATE UNIQUE INDEX idx_user_schemas_project_path ON user_schemas(project_id, path, data_type);

-- Rename event_schemas back to user_event_schemas
ALTER TABLE event_schemas RENAME TO user_event_schemas;
ALTER INDEX idx_event_schemas_event_id RENAME TO idx_user_event_schemas_event_id;
ALTER INDEX idx_event_schemas_event_path RENAME TO idx_user_event_schemas_event_path;

-- Restore the original unique index on events (without subject_type)
DROP INDEX idx_events_project_subject;
DROP INDEX idx_events_project_name_subject;
CREATE UNIQUE INDEX idx_events_project_name ON events(project_id, name);

-- Remove subject_type column from events
ALTER TABLE events DROP COLUMN IF EXISTS subject_type;

-- Drop enum
DROP TYPE IF EXISTS subject_type;
