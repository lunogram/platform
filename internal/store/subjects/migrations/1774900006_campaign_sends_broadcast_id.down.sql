DROP INDEX IF EXISTS campaign_sends_broadcast_id_idx;
ALTER TABLE campaign_sends DROP COLUMN IF EXISTS broadcast_id;
