ALTER TABLE devices ADD COLUMN push_config JSONB;

-- Migrate existing FCM tokens
UPDATE devices
SET push_config = jsonb_build_object('type', 'fcm', 'token', token)
WHERE token IS NOT NULL AND token != '';

-- Migrate existing Web Push subscriptions
UPDATE devices
SET push_config = jsonb_build_object(
    'type',     'webpush',
    'endpoint', device_credentials->>'endpoint',
    'keys',     jsonb_build_object(
                    'auth',   device_credentials->'keys'->>'auth',
                    'p256dh', device_credentials->'keys'->>'p256dh'
                )
)
WHERE push_config IS NULL
  AND device_credentials IS NOT NULL
  AND device_credentials->>'endpoint' IS NOT NULL;

-- Drop old unique index on (project_id, token)
DROP INDEX IF EXISTS devices_project_token_uniq;

-- New functional unique index covering FCM and APNs tokens
CREATE UNIQUE INDEX devices_project_push_token_uniq
    ON devices ((push_config->>'token'), project_id)
    WHERE deleted_at IS NULL
      AND push_config->>'token' IS NOT NULL;

ALTER TABLE devices DROP COLUMN token;
ALTER TABLE devices DROP COLUMN device_credentials;
