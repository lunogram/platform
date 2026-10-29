-- Revert: copy the JSONB key "reply_to" back to "replyTo" in email template data.
-- reply_to is kept, since the backend reads it.

UPDATE templates
SET data = data || jsonb_build_object('replyTo', data->'reply_to')
WHERE type = 'email'
  AND data ? 'reply_to'
  AND NOT data ? 'replyTo';
