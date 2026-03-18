-- Reverse: restore identity UUIDs back into template JSONB data column.

-- For email templates: put the UUID back into data.from.email
UPDATE templates
SET data = CASE
    WHEN data->'from' IS NOT NULL
    THEN jsonb_set(data, '{from,email}', to_jsonb(sender_identity_id::text))
    ELSE jsonb_set(data, '{from}', jsonb_build_object('email', sender_identity_id::text))
END
WHERE sender_identity_id IS NOT NULL
    AND type = 'email';

-- For SMS templates: put the UUID back into data.from as a string
UPDATE templates
SET data = jsonb_set(data, '{from}', to_jsonb(sender_identity_id::text))
WHERE sender_identity_id IS NOT NULL
    AND type IN ('text', 'sms');

DROP INDEX IF EXISTS templates_sender_identity_id_idx;
ALTER TABLE templates DROP COLUMN sender_identity_id;

-- Restore default_from_name into provider data from identity traits.
WITH identity_names AS (
    SELECT
        si.id AS identity_id,
        si.traits->>'name' AS from_name
    FROM sender_identities si
    WHERE si.traits->>'name' IS NOT NULL
      AND si.traits->>'name' != ''
)
UPDATE providers p
SET data = data || jsonb_build_object('default_from_name', inn.from_name)
FROM identity_names inn
WHERE (p.data->>'default_from')::UUID = inn.identity_id
  AND p.data->>'default_from' IS NOT NULL
  AND p.data->>'default_from' ~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$';

-- Reverse: restore provider default_from from UUID → address
UPDATE providers p
SET data = jsonb_set(p.data, '{default_from}', to_jsonb(si.traits->>'address'))
FROM sender_identities si
WHERE p.data->>'default_from' IS NOT NULL
  AND p.data->>'default_from' ~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
  AND si.id = (p.data->>'default_from')::uuid;

-- Reverse: restore email template from.email from UUID → address
UPDATE templates t
SET data = jsonb_set(t.data, '{from,email}', to_jsonb(si.traits->>'address'))
FROM sender_identities si
WHERE t.data->'from'->>'email' IS NOT NULL
  AND t.data->'from'->>'email' ~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
  AND si.id = (t.data->'from'->>'email')::uuid;

-- Reverse: restore SMS template from from UUID → address
UPDATE templates t
SET data = jsonb_set(t.data, '{from}', to_jsonb(si.traits->>'address'))
FROM sender_identities si
WHERE t.data->>'from' IS NOT NULL
  AND t.data->>'from' ~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
  AND si.id = (t.data->>'from')::uuid;

-- Drop the table (cascades trigger and indexes).
DROP TRIGGER IF EXISTS set_updated_at_sender_identities ON sender_identities;
DROP TABLE IF EXISTS sender_identities;
