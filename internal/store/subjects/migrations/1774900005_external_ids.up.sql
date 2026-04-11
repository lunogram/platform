-- External IDs migration
-- Replace anonymous_id/external_id columns on users and organizations with
-- mapping tables that allow multiple external identifiers per entity.

-- Enable pg_trgm for ILIKE search indexes
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- ============================================================================
-- 1. Create user_external_ids table
-- ============================================================================
CREATE TABLE user_external_ids (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id UUID NOT NULL,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    source VARCHAR(255) NOT NULL,
    external_id VARCHAR(255) NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- A given source+external_id pair can only belong to one user within a project
CREATE UNIQUE INDEX user_external_ids_project_source_eid_uniq
    ON user_external_ids(project_id, source, external_id);

-- Allow multiple IDs per source per user, but prevent exact duplicates
CREATE UNIQUE INDEX user_external_ids_user_source_eid_uniq
    ON user_external_ids(user_id, source, external_id);

-- Fast lookup by user
CREATE INDEX user_external_ids_user_id_idx ON user_external_ids(user_id);

-- Fast lookup by project + source
CREATE INDEX user_external_ids_project_source_idx ON user_external_ids(project_id, source);

-- Trigram index for ILIKE search on external_id
CREATE INDEX user_external_ids_external_id_trgm_idx
    ON user_external_ids USING gin (external_id gin_trgm_ops);

CREATE TRIGGER set_updated_at_user_external_ids
    BEFORE UPDATE ON user_external_ids
    FOR EACH ROW EXECUTE PROCEDURE set_updated_at();

-- ============================================================================
-- 2. Create organization_external_ids table
-- ============================================================================
CREATE TABLE organization_external_ids (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id UUID NOT NULL,
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    source VARCHAR(255) NOT NULL,
    external_id VARCHAR(255) NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- A given source+external_id pair can only belong to one org within a project
CREATE UNIQUE INDEX org_external_ids_project_source_eid_uniq
    ON organization_external_ids(project_id, source, external_id);

-- Allow multiple IDs per source per org, but prevent exact duplicates
CREATE UNIQUE INDEX org_external_ids_org_source_eid_uniq
    ON organization_external_ids(organization_id, source, external_id);

-- Fast lookup by organization
CREATE INDEX org_external_ids_org_id_idx ON organization_external_ids(organization_id);

-- Fast lookup by project + source
CREATE INDEX org_external_ids_project_source_idx ON organization_external_ids(project_id, source);

-- Trigram index for ILIKE search on external_id
CREATE INDEX org_external_ids_external_id_trgm_idx
    ON organization_external_ids USING gin (external_id gin_trgm_ops);

CREATE TRIGGER set_updated_at_organization_external_ids
    BEFORE UPDATE ON organization_external_ids
    FOR EACH ROW EXECUTE PROCEDURE set_updated_at();

-- ============================================================================
-- 3. Migrate existing user data into user_external_ids
-- ============================================================================

-- Migrate external_id values (source='default')
INSERT INTO user_external_ids (project_id, user_id, source, external_id)
SELECT project_id, id, 'default', external_id
FROM users
WHERE external_id IS NOT NULL;

-- Migrate anonymous_id values (source='anonymous')
INSERT INTO user_external_ids (project_id, user_id, source, external_id)
SELECT project_id, id, 'anonymous', anonymous_id
FROM users
WHERE anonymous_id IS NOT NULL;

-- ============================================================================
-- 4. Migrate existing organization data into organization_external_ids
-- ============================================================================

-- Migrate external_id values (source='default')
INSERT INTO organization_external_ids (project_id, organization_id, source, external_id)
SELECT project_id, id, 'default', external_id
FROM organizations
WHERE external_id IS NOT NULL;

-- ============================================================================
-- 5. Verify data migration before dropping old columns
-- ============================================================================

-- Ensure every user with an external_id has been migrated to user_external_ids.
-- If this query finds any missing rows, the DO block raises an exception and
-- the entire migration is rolled back.
DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM users u
    WHERE u.external_id IS NOT NULL
    AND NOT EXISTS (
      SELECT 1 FROM user_external_ids uei
      WHERE uei.user_id = u.id AND uei.source = 'default' AND uei.external_id = u.external_id
    )
  ) THEN
    RAISE EXCEPTION 'Data migration verification failed: some user external_ids were not migrated';
  END IF;

  IF EXISTS (
    SELECT 1 FROM users u
    WHERE u.anonymous_id IS NOT NULL
    AND NOT EXISTS (
      SELECT 1 FROM user_external_ids uei
      WHERE uei.user_id = u.id AND uei.source = 'anonymous' AND uei.external_id = u.anonymous_id
    )
  ) THEN
    RAISE EXCEPTION 'Data migration verification failed: some user anonymous_ids were not migrated';
  END IF;

  IF EXISTS (
    SELECT 1 FROM organizations o
    WHERE o.external_id IS NOT NULL
    AND NOT EXISTS (
      SELECT 1 FROM organization_external_ids oei
      WHERE oei.organization_id = o.id AND oei.source = 'default' AND oei.external_id = o.external_id
    )
  ) THEN
    RAISE EXCEPTION 'Data migration verification failed: some organization external_ids were not migrated';
  END IF;
END
$$;

-- ============================================================================
-- 6. Drop old columns and indexes from users table
-- ============================================================================

-- Drop indexes that reference old columns
DROP INDEX IF EXISTS users_project_anonymous_uniq;
DROP INDEX IF EXISTS users_project_external_uniq;
DROP INDEX IF EXISTS users_external_id_idx;
DROP INDEX IF EXISTS users_anonymous_id_idx;

-- Drop old columns
ALTER TABLE users DROP COLUMN anonymous_id;
ALTER TABLE users DROP COLUMN external_id;

-- ============================================================================
-- 7. Drop old columns and indexes from organizations table
-- ============================================================================

-- Drop indexes that reference old columns
DROP INDEX IF EXISTS organizations_project_external_uniq;

-- Drop old column
ALTER TABLE organizations DROP COLUMN external_id;

-- ============================================================================
-- 8. Aggregation views for external IDs
-- ============================================================================
-- These views encapsulate the jsonb_agg logic so that Go queries can LEFT JOIN
-- them instead of embedding identical correlated subqueries everywhere.

CREATE VIEW user_external_ids_agg AS
SELECT
    uei.user_id,
    jsonb_agg(jsonb_build_object(
        'id', uei.id,
        'project_id', uei.project_id,
        'subject_id', uei.user_id,
        'source', uei.source,
        'external_id', uei.external_id,
        'metadata', uei.metadata,
        'created_at', uei.created_at,
        'updated_at', uei.updated_at
    ) ORDER BY uei.created_at ASC) AS external_ids
FROM user_external_ids uei
GROUP BY uei.user_id;

CREATE VIEW organization_external_ids_agg AS
SELECT
    oei.organization_id,
    jsonb_agg(jsonb_build_object(
        'id', oei.id,
        'project_id', oei.project_id,
        'subject_id', oei.organization_id,
        'source', oei.source,
        'external_id', oei.external_id,
        'metadata', oei.metadata,
        'created_at', oei.created_at,
        'updated_at', oei.updated_at
    ) ORDER BY oei.created_at ASC) AS external_ids
FROM organization_external_ids oei
GROUP BY oei.organization_id;
