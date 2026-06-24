-- Add function_id column to action_schemas for multi-function action modules.
ALTER TABLE action_schemas ADD COLUMN function_id VARCHAR(255) NOT NULL DEFAULT '';

-- Drop old unique index and create new one that includes function_id.
DROP INDEX IF EXISTS idx_action_schemas_action_path;
CREATE UNIQUE INDEX idx_action_schemas_action_function_path ON action_schemas(action_id, function_id, path, data_type);
