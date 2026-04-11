ALTER TABLE user_devices DROP COLUMN IF EXISTS data;

ALTER TABLE user_devices RENAME COLUMN config TO push_config;

ALTER TABLE user_devices RENAME TO devices;
