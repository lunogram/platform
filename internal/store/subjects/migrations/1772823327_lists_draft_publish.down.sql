-- Re-add rule_id to lists from the published version
ALTER TABLE lists ADD COLUMN rule_id UUID REFERENCES rules(id);

UPDATE lists l
SET rule_id = lv.rule_id
FROM list_versions lv
WHERE lv.list_id = l.id AND lv.status = 'published';

-- Remove version_id FK and column
ALTER TABLE lists DROP CONSTRAINT IF EXISTS fk_lists_version_id;
ALTER TABLE lists DROP COLUMN version_id;

-- Drop list_versions table
DROP TABLE list_versions;

-- Drop enum type
DROP TYPE list_version_status;
