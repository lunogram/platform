-- Revert: rename the JSONB key "body" back to "message" in SMS template data.

UPDATE templates
SET data = (data - 'body') || jsonb_build_object('message', data->'body')
WHERE type = 'sms'
  AND data ? 'body'
  AND NOT data ? 'message';
