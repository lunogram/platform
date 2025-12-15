-- Revert renaming of campaign_sends.sent_at back to send_at
ALTER TABLE campaign_sends RENAME COLUMN sent_at TO send_at;

-- Revert index name
DROP INDEX IF EXISTS campaign_sends_sent_at_idx;
CREATE INDEX campaign_sends_send_at_idx ON campaign_sends USING btree (send_at);
