CREATE TYPE list_version_status AS ENUM ('draft', 'published', 'archived');

CREATE TABLE list_versions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    list_id UUID NOT NULL REFERENCES lists(id) ON DELETE CASCADE,
    version_number INTEGER NOT NULL,
    status list_version_status NOT NULL DEFAULT 'draft',
    rule_id UUID REFERENCES rules(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at TIMESTAMPTZ,
    UNIQUE (list_id, version_number)
);

-- Add version_id pointer to lists (same pattern as journeys)
ALTER TABLE lists ADD COLUMN version_id UUID;

-- Migrate existing lists: create a version row for each list that has a rule
INSERT INTO list_versions (list_id, version_number, status, rule_id, created_at, published_at)
SELECT id, 1, 'published', rule_id, created_at, updated_at
FROM lists
WHERE rule_id IS NOT NULL AND deleted_at IS NULL;

-- Point each list to its newly created version
UPDATE lists l
SET version_id = lv.id
FROM list_versions lv
WHERE lv.list_id = l.id;

-- Add FK constraint after data migration
ALTER TABLE lists ADD CONSTRAINT fk_lists_version_id
    FOREIGN KEY (version_id) REFERENCES list_versions(id) ON DELETE SET NULL;

-- Now remove rule_id from lists
ALTER TABLE lists DROP COLUMN rule_id;

CREATE INDEX idx_list_versions_list_id ON list_versions(list_id);
CREATE INDEX idx_list_versions_status ON list_versions(list_id, status);
