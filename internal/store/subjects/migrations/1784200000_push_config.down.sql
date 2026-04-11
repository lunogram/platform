ALTER TABLE devices ADD COLUMN token VARCHAR(255);
ALTER TABLE devices ADD COLUMN device_credentials JSONB;

UPDATE devices
SET token = push_config->>'token'
WHERE push_config->>'type' IN ('fcm', 'apns');

UPDATE devices
SET device_credentials = jsonb_build_object(
    'endpoint', push_config->>'endpoint',
    'keys',     push_config->'keys'
)
WHERE push_config->>'type' = 'webpush';

DROP INDEX IF EXISTS devices_project_push_token_uniq;

CREATE UNIQUE INDEX devices_project_token_uniq ON devices(project_id, token);

ALTER TABLE devices DROP COLUMN push_config;
