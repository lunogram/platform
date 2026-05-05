-- rename the old table to the new name
ALTER TABLE project_push_providers RENAME TO project_providers;

-- drop the old trigger
DROP TRIGGER set_updated_at_project_push_providers ON project_providers;

DROP INDEX project_push_providers_project_platform_uniq;

-- recreate trigger with new name
CREATE TRIGGER set_updated_at_project_providers 
BEFORE UPDATE ON project_providers 
FOR EACH ROW EXECUTE PROCEDURE set_updated_at();

-- preserve existing platform values
ALTER TABLE project_providers ADD COLUMN platform_new VARCHAR(50) CHECK (platform_new IN ('ios', 'android', 'web', 'mail'));
UPDATE project_providers SET platform_new = platform;
ALTER TABLE project_providers DROP COLUMN platform;
ALTER TABLE project_providers RENAME COLUMN platform_new TO platform;
ALTER TABLE project_providers ALTER COLUMN platform SET NOT NULL;

ALTER TABLE project_providers ADD CONSTRAINT project_providers_project_platform_uniq UNIQUE (project_id, platform);