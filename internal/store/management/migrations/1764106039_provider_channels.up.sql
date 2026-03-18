-- Add channels JSONB column, migrate existing data, drop old column
ALTER TABLE providers ADD COLUMN channels JSONB;
UPDATE providers SET channels = jsonb_build_array(channel);
ALTER TABLE providers ALTER COLUMN channels SET NOT NULL;
ALTER TABLE providers DROP COLUMN channel;

-- Drop the is_default column (no longer used)
ALTER TABLE providers DROP COLUMN is_default;
