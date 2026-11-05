-- Rename the JSONB key "replyTo" to "reply_to" in email template data.
-- The frontend previously saved the reply-to address under data->'replyTo',
-- but the backend and the console read data->'reply_to', so the address was
-- never sent and never shown again after saving.

UPDATE templates
SET data = (data - 'replyTo') || jsonb_build_object('reply_to', data->'replyTo')
WHERE type = 'email'
  AND data ? 'replyTo'
  AND COALESCE(data->>'replyTo', '') <> ''
  AND COALESCE(data->>'reply_to', '') = '';

-- Drop the remaining stale keys, which are empty or shadowed by reply_to.
UPDATE templates
SET data = data - 'replyTo'
WHERE type = 'email'
  AND data ? 'replyTo';
