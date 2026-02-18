-- Remove 'null' from data_type enum
-- Note: PostgreSQL doesn't support directly removing enum values
-- We need to recreate the type without the 'null' value

-- First, update any existing records that use 'null' to a default value
UPDATE event_schemas SET data_type = 'string' WHERE data_type = 'null';
UPDATE user_schemas SET data_type = 'string' WHERE data_type = 'null';

-- Rename the current enum type
ALTER TYPE data_type RENAME TO data_type_old;

-- Create a new enum type without 'null'
CREATE TYPE data_type AS ENUM('string', 'number', 'boolean', 'object', 'array');

-- Update the columns to use the new type
ALTER TABLE event_schemas ALTER COLUMN data_type TYPE data_type USING data_type::text::data_type;
ALTER TABLE user_schemas ALTER COLUMN data_type TYPE data_type USING data_type::text::data_type;

-- Drop the old enum type
DROP TYPE data_type_old;