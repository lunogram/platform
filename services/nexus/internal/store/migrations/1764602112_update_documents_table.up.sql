-- Rename images table to documents
ALTER TABLE images RENAME TO documents;

-- Rename original_filename to filename
ALTER TABLE documents RENAME COLUMN original_filename TO filename;

-- Remove storage_type, storage_path, and storage_url columns
ALTER TABLE documents DROP COLUMN storage_type;
ALTER TABLE documents DROP COLUMN storage_path;
ALTER TABLE documents DROP COLUMN storage_url;

-- Drop the index on storage_type since it no longer exists
DROP INDEX IF EXISTS images_storage_type_idx;

-- Add key column to store the storage key/path
ALTER TABLE documents ADD COLUMN key VARCHAR(255) DEFAULT '';

-- Create index on key for faster lookups
CREATE INDEX documents_key_idx ON documents(key) WHERE key != '';
