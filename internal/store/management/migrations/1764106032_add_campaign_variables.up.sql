ALTER TABLE campaigns ADD COLUMN variables jsonb NOT NULL DEFAULT '[]'::jsonb;
