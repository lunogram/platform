DROP INDEX IF EXISTS idx_action_schemas_action_function_path;

-- Deduplicate rows before restoring the old unique index.
-- Keep the row with the latest updated_at (or lowest id as tiebreaker)
-- for each (action_id, path, data_type) group that would otherwise conflict.
DELETE FROM action_schemas
WHERE id NOT IN (
    SELECT MIN(id) FROM action_schemas
    GROUP BY action_id, path, data_type
);

CREATE UNIQUE INDEX idx_action_schemas_action_path ON action_schemas(action_id, path, data_type);
ALTER TABLE action_schemas DROP COLUMN function_id;
