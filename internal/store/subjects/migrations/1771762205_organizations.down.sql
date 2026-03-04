-- Drop organization events table
DROP TABLE IF EXISTS organization_events;

-- Drop organization dependency columns from rules table
ALTER TABLE rules DROP COLUMN IF EXISTS depends_on_organization_users;
ALTER TABLE rules DROP COLUMN IF EXISTS depends_on_organizations;

-- Drop schema tables first
DROP TABLE IF EXISTS organization_user_schemas;
DROP TABLE IF EXISTS organization_schemas;

-- Drop organization users table (depends on organizations)
DROP TABLE IF EXISTS organization_users;

-- Drop organizations table
DROP TABLE IF EXISTS organizations;
