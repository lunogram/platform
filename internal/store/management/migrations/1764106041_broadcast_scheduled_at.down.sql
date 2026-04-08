DROP INDEX IF EXISTS idx_campaign_broadcasts_scheduled;
ALTER TABLE campaign_broadcasts DROP COLUMN IF EXISTS scheduled_at;
