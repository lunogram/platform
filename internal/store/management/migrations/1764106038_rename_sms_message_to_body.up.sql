-- Rename the JSONB key "message" to "body" in SMS template data.
-- The frontend previously saved the SMS message body under data->'message',
-- but the OAPI spec and backend expect data->'body'.

UPDATE templates
SET data = (data - 'message') || jsonb_build_object('body', data->'message')
WHERE type = 'sms'
  AND data ? 'message'
  AND NOT data ? 'body';
