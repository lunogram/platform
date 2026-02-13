-- Make admins.organization_id NOT NULL (idempotent)
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'admins' AND column_name = 'organization_id' AND is_nullable = 'YES') THEN
        ALTER TABLE admins ALTER COLUMN organization_id SET NOT NULL;
    END IF;
END $$;
