ALTER TABLE campaign_broadcasts ADD COLUMN scheduled_at TIMESTAMPTZ;

CREATE INDEX idx_campaign_broadcasts_scheduled
    ON campaign_broadcasts (scheduled_at)
    WHERE state = 'scheduled' AND scheduled_at IS NOT NULL;
