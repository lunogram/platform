-- Sender identities table
-- Stores registered sender addresses (email, SMS, etc.) scoped to a provider.
-- Channel-specific metadata lives in the traits JSONB column; the address is
-- always stored as traits->>'address'.

CREATE TABLE sender_identities (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    provider_id UUID NOT NULL REFERENCES providers(id) ON DELETE CASCADE,
    channel VARCHAR(255) NOT NULL,
    traits JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX sender_identities_project_id_idx ON sender_identities(project_id);
CREATE INDEX sender_identities_provider_id_idx ON sender_identities(provider_id);
CREATE UNIQUE INDEX sender_identities_provider_channel_address_uniq
    ON sender_identities (provider_id, channel, (traits->>'address'));

CREATE TRIGGER set_updated_at_sender_identities BEFORE UPDATE ON sender_identities FOR EACH ROW EXECUTE PROCEDURE set_updated_at();

--------------------------------------------------------------------------------
-- Data migration: provider default_from → sender_identities
--------------------------------------------------------------------------------
-- For every provider whose default_from is a raw address (not a UUID), create
-- a sender identity and replace the address string with the new identity UUID.

WITH provider_inserts AS (
    INSERT INTO sender_identities (project_id, provider_id, channel, traits)
    SELECT
        p.project_id,
        p.id,
        p.channel,
        jsonb_build_object('address', p.data->>'default_from')
    FROM providers p
    WHERE p.data->>'default_from' IS NOT NULL
      AND p.data->>'default_from' != ''
      AND p.data->>'default_from' !~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
    ON CONFLICT (provider_id, channel, (traits->>'address')) DO NOTHING
    RETURNING id, provider_id, channel, traits->>'address' AS address
)
UPDATE providers p
SET data = jsonb_set(p.data, '{default_from}', to_jsonb(pi.id::text))
FROM provider_inserts pi
WHERE p.id = pi.provider_id;

--------------------------------------------------------------------------------
-- Data migration: email template from.email → sender_identities
--------------------------------------------------------------------------------

WITH email_template_data AS (
    SELECT
        t.id AS template_id,
        t.project_id,
        COALESCE(c.provider_id, dp.id) AS provider_id,
        'email' AS channel,
        t.data->'from'->>'email' AS address
    FROM templates t
    JOIN campaigns c ON t.campaign_id = c.id
    LEFT JOIN providers dp
        ON dp.project_id = t.project_id
       AND dp.channel = 'email'
       AND dp.is_default = true
    WHERE c.channel = 'email'
      AND t.data->'from'->>'email' IS NOT NULL
      AND t.data->'from'->>'email' != ''
      AND t.data->'from'->>'email' !~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
      AND COALESCE(c.provider_id, dp.id) IS NOT NULL
),
email_inserts AS (
    INSERT INTO sender_identities (project_id, provider_id, channel, traits)
    SELECT DISTINCT
        etd.project_id,
        etd.provider_id,
        etd.channel,
        jsonb_build_object('address', etd.address)
    FROM email_template_data etd
    ON CONFLICT (provider_id, channel, (traits->>'address')) DO NOTHING
    RETURNING id, provider_id, channel, traits->>'address' AS address
)
UPDATE templates t
SET data = jsonb_set(t.data, '{from,email}', to_jsonb(si.id::text))
FROM email_template_data etd
JOIN sender_identities si
    ON si.provider_id = etd.provider_id
   AND si.channel = etd.channel
   AND si.traits->>'address' = etd.address
WHERE t.id = etd.template_id;

--------------------------------------------------------------------------------
-- Data migration: SMS template from → sender_identities
--------------------------------------------------------------------------------

WITH sms_template_data AS (
    SELECT
        t.id AS template_id,
        t.project_id,
        COALESCE(c.provider_id, dp.id) AS provider_id,
        'sms' AS channel,
        t.data->>'from' AS address
    FROM templates t
    JOIN campaigns c ON t.campaign_id = c.id
    LEFT JOIN providers dp
        ON dp.project_id = t.project_id
       AND dp.channel = 'sms'
       AND dp.is_default = true
    WHERE c.channel = 'sms'
      AND t.data->>'from' IS NOT NULL
      AND t.data->>'from' != ''
      AND t.data->>'from' !~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
      AND COALESCE(c.provider_id, dp.id) IS NOT NULL
),
sms_inserts AS (
    INSERT INTO sender_identities (project_id, provider_id, channel, traits)
    SELECT DISTINCT
        std.project_id,
        std.provider_id,
        std.channel,
        jsonb_build_object('address', std.address)
    FROM sms_template_data std
    ON CONFLICT (provider_id, channel, (traits->>'address')) DO NOTHING
    RETURNING id, provider_id, channel, traits->>'address' AS address
)
UPDATE templates t
SET data = jsonb_set(t.data, '{from}', to_jsonb(si.id::text))
FROM sms_template_data std
JOIN sender_identities si
    ON si.provider_id = std.provider_id
   AND si.channel = std.channel
   AND si.traits->>'address' = std.address
WHERE t.id = std.template_id;

--------------------------------------------------------------------------------
-- Data migration: copy default_from_name into identity traits as "name"
--------------------------------------------------------------------------------

WITH provider_names AS (
    SELECT
        id AS provider_id,
        (data->>'default_from')::UUID AS identity_id,
        data->>'default_from_name' AS from_name
    FROM providers
    WHERE data->>'default_from' IS NOT NULL
      AND data->>'default_from_name' IS NOT NULL
      AND data->>'default_from_name' != ''
      AND data->>'default_from' ~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
)
UPDATE sender_identities si
SET traits = si.traits || jsonb_build_object('name', pn.from_name)
FROM provider_names pn
WHERE si.id = pn.identity_id;

-- Strip default_from_name and default_from_locked from provider data.
UPDATE providers
SET data = data - 'default_from_name' - 'default_from_locked'
WHERE data ? 'default_from_name' OR data ? 'default_from_locked';

--------------------------------------------------------------------------------
-- Add sender_identity_id column to templates
--------------------------------------------------------------------------------

ALTER TABLE templates
    ADD COLUMN sender_identity_id UUID REFERENCES sender_identities(id) ON DELETE SET NULL;

-- Backfill from existing JSONB data:
-- Email templates store the identity UUID in data->'from'->>'email'
-- SMS templates store it in data->>'from'
UPDATE templates
SET sender_identity_id = CASE
    WHEN data->'from'->>'email' IS NOT NULL
         AND data->'from'->>'email' ~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
    THEN (data->'from'->>'email')::uuid
    WHEN data->>'from' IS NOT NULL
         AND jsonb_typeof(data->'from') = 'string'
         AND data->>'from' ~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
    THEN (data->>'from')::uuid
    ELSE NULL
END
WHERE (
    data->'from'->>'email' ~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
    OR (
        jsonb_typeof(data->'from') = 'string'
        AND data->>'from' ~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
    )
);

-- Strip the identity UUID from the JSONB data now that it lives in the column.
-- For email: remove data.from.email (keep data.from.name if present)
UPDATE templates
SET data = CASE
    WHEN data->'from'->'name' IS NOT NULL AND data->'from'->>'name' != ''
    THEN jsonb_set(data, '{from}', jsonb_build_object('name', data->'from'->'name'))
    ELSE data - 'from'
END
WHERE data->'from'->>'email' ~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$';

-- For SMS: remove data.from entirely (it was just the UUID string)
UPDATE templates
SET data = data - 'from'
WHERE jsonb_typeof(data->'from') = 'string'
    AND data->>'from' ~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$';

CREATE INDEX templates_sender_identity_id_idx ON templates(sender_identity_id);
