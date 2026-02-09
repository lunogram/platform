-- Drop key column and index
DROP INDEX IF EXISTS documents_key_idx;
ALTER TABLE documents DROP COLUMN IF EXISTS key;

-- Restore storage columns
ALTER TABLE documents ADD COLUMN storage_url text;
ALTER TABLE documents ADD COLUMN storage_path text NOT NULL DEFAULT '';
ALTER TABLE documents ADD COLUMN storage_type varchar(50) NOT NULL DEFAULT 'local';

-- Restore the index on storage_type
CREATE INDEX images_storage_type_idx ON public.documents USING btree (storage_type);

-- Rename filename back to original_filename
ALTER TABLE documents RENAME COLUMN filename TO original_filename;

-- Rename documents table back to images
ALTER TABLE documents RENAME TO images;
