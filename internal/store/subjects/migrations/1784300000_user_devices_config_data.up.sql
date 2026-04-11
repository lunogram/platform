ALTER TABLE devices RENAME TO user_devices;

ALTER TABLE user_devices RENAME COLUMN push_config TO config;

ALTER TABLE user_devices ADD COLUMN IF NOT EXISTS data JSONB NOT NULL DEFAULT '{}'::jsonb;

UPDATE user_devices SET data = '{}'::jsonb WHERE data IS NULL;
