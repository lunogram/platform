-- Rename provider columns (idempotent)
DO $$
BEGIN
    -- Rename group to channel if group exists
    IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'providers' AND column_name = 'group') THEN
        ALTER TABLE providers RENAME COLUMN "group" TO channel;
    END IF;
    
    -- Rename type to module if type exists
    IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'providers' AND column_name = 'type') THEN
        ALTER TABLE providers RENAME COLUMN type TO module;
    END IF;
END $$;
