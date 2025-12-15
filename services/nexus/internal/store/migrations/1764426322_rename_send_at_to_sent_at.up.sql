-- Rename campaign_sends.send_at to campaign_sends.sent_at for consistency with API naming
ALTER TABLE campaign_sends RENAME COLUMN send_at TO sent_at;

-- Update index to reflect new column name
DROP INDEX IF EXISTS campaign_sends_send_at_idx;
CREATE INDEX campaign_sends_sent_at_idx ON campaign_sends USING btree (sent_at);
