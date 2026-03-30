-- Down migration for external IDs
--
-- WARNING: This migration is destructive. Running it in production will lose
-- all multi-identifier data that cannot be represented by the original
-- single-column schema (e.g. multiple sources per user, metadata).
-- It is provided for development/testing convenience only.

-- ============================================================================
-- 1. Drop aggregation views
-- ============================================================================
DROP VIEW IF EXISTS organization_external_ids_agg;
DROP VIEW IF EXISTS user_external_ids_agg;

-- ============================================================================
-- 2. Restore old columns on users table
-- ============================================================================
ALTER TABLE users ADD COLUMN anonymous_id VARCHAR(255) DEFAULT (uuid_generate_v4())::text;
ALTER TABLE users ADD COLUMN external_id VARCHAR(255);

-- Migrate data back: pick the 'default' source identifier as external_id
UPDATE users u
SET external_id = uei.external_id
FROM user_external_ids uei
WHERE uei.user_id = u.id AND uei.source = 'default';

-- Migrate data back: pick the 'anonymous' source identifier as anonymous_id
UPDATE users u
SET anonymous_id = uei.external_id
FROM user_external_ids uei
WHERE uei.user_id = u.id AND uei.source = 'anonymous';

-- Restore original indexes
CREATE UNIQUE INDEX users_project_anonymous_uniq ON users(project_id, anonymous_id);
CREATE UNIQUE INDEX users_project_external_uniq ON users(project_id, external_id);
CREATE INDEX users_external_id_idx ON users(external_id);
CREATE INDEX users_anonymous_id_idx ON users(anonymous_id);

-- ============================================================================
-- 3. Restore old column on organizations table
-- ============================================================================
ALTER TABLE organizations ADD COLUMN external_id VARCHAR(255);

-- Migrate data back: pick the 'default' source identifier as external_id
UPDATE organizations o
SET external_id = oei.external_id
FROM organization_external_ids oei
WHERE oei.organization_id = o.id AND oei.source = 'default';

-- Make external_id NOT NULL (original schema)
ALTER TABLE organizations ALTER COLUMN external_id SET NOT NULL;

-- Restore original index
CREATE UNIQUE INDEX organizations_project_external_uniq ON organizations(project_id, external_id);

-- ============================================================================
-- 4. Drop new tables (cascades triggers and indexes)
-- ============================================================================
DROP TABLE IF EXISTS organization_external_ids CASCADE;
DROP TABLE IF EXISTS user_external_ids CASCADE;

-- ============================================================================
-- 5. Drop pg_trgm extension (only if no other objects depend on it)
-- ============================================================================
-- Not dropping pg_trgm as other schemas may depend on it.
