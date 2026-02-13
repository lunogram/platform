-- Add deleted_at column to organizations if it doesn't exist
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'organizations' AND column_name = 'deleted_at') THEN
        ALTER TABLE organizations ADD COLUMN deleted_at timestamptz;
    END IF;
END $$;
