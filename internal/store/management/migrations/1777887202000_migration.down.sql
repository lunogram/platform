-- down migration

-- drop new trigger
DROP TRIGGER set_updated_at_project_providers ON project_providers;

-- remove mail rows, these are gone for good
DELETE FROM project_providers WHERE platform = 'mail';

-- preserve platform values while swapping the check constraint
ALTER TABLE project_providers ADD COLUMN platform_old VARCHAR(50);
UPDATE project_providers SET platform_old = platform;
ALTER TABLE project_providers DROP COLUMN platform;
ALTER TABLE project_providers ADD COLUMN platform VARCHAR(50) NOT NULL CHECK (platform IN ('ios', 'android', 'web'));
UPDATE project_providers SET platform = platform_old;
ALTER TABLE project_providers DROP COLUMN platform_old;

-- rename back
ALTER TABLE project_providers RENAME TO project_push_providers;

-- recreate original unique index
CREATE UNIQUE INDEX project_push_providers_project_platform_uniq
ON project_push_providers (project_id, platform);

-- recreate original trigger
CREATE TRIGGER set_updated_at_project_push_providers 
BEFORE UPDATE ON project_push_providers 
FOR EACH ROW EXECUTE PROCEDURE set_updated_at();