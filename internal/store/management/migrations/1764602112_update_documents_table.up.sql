-- Rename images table to documents (idempotent)
DO $$
BEGIN
    -- Rename images table to documents if images exists
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'images') THEN
        ALTER TABLE images RENAME TO documents;
    END IF;
    
    -- Rename original_filename to filename if original_filename exists
    IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'documents' AND column_name = 'original_filename') THEN
        ALTER TABLE documents RENAME COLUMN original_filename TO filename;
    END IF;
    
    -- Remove storage_type column if it exists
    IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'documents' AND column_name = 'storage_type') THEN
        ALTER TABLE documents DROP COLUMN storage_type;
    END IF;
    
    -- Remove storage_path column if it exists
    IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'documents' AND column_name = 'storage_path') THEN
        ALTER TABLE documents DROP COLUMN storage_path;
    END IF;
    
    -- Remove storage_url column if it exists
    IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'documents' AND column_name = 'storage_url') THEN
        ALTER TABLE documents DROP COLUMN storage_url;
    END IF;
    
    -- Add key column if it doesn't exist
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'documents' AND column_name = 'key') THEN
        ALTER TABLE documents ADD COLUMN key VARCHAR(255) DEFAULT '';
    END IF;
END $$;

-- Drop the index on storage_type since it no longer exists
DROP INDEX IF EXISTS images_storage_type_idx;

-- Create index on key for faster lookups (idempotent)
CREATE INDEX IF NOT EXISTS documents_key_idx ON documents(key);
