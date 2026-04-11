-- Revert the primary key change that added broadcast_id.

-- 1. Drop the new composite primary key.
ALTER TABLE campaign_sends DROP CONSTRAINT campaign_sends_pkey;

-- 2. Restore the original primary key without broadcast_id.
ALTER TABLE campaign_sends ADD PRIMARY KEY (campaign_id, user_id, reference_id);

-- 3. Allow NULLs again in broadcast_id.
ALTER TABLE campaign_sends ALTER COLUMN broadcast_id DROP NOT NULL;

-- 4. Turn the zero-UUID placeholders back into NULLs.
UPDATE campaign_sends
SET broadcast_id = NULL
WHERE broadcast_id = '00000000-0000-0000-0000-000000000000';
