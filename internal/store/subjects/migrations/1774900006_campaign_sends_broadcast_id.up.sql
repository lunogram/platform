ALTER TABLE campaign_sends ADD COLUMN broadcast_id UUID;
CREATE INDEX campaign_sends_broadcast_id_idx ON campaign_sends(broadcast_id);
