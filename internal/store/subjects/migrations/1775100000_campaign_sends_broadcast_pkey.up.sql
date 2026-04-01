-- Replace the primary key on campaign_sends to include broadcast_id so that
-- the same campaign+user combination can appear in multiple broadcasts.
--
-- Existing rows with a NULL broadcast_id get a deterministic placeholder so
-- they satisfy the NOT NULL constraint required by the composite PK.

-- 1. Back-fill NULL broadcast_ids with a zero UUID so every row has a value.
UPDATE campaign_sends
SET broadcast_id = '00000000-0000-0000-0000-000000000000'
WHERE broadcast_id IS NULL;

-- 2. Make the column NOT NULL now that all rows have a value.
ALTER TABLE campaign_sends ALTER COLUMN broadcast_id SET NOT NULL;

-- 3. Drop the old primary key and create the new one that includes broadcast_id.
ALTER TABLE campaign_sends DROP CONSTRAINT campaign_sends_pkey;
ALTER TABLE campaign_sends ADD PRIMARY KEY (campaign_id, user_id, broadcast_id, reference_id);
