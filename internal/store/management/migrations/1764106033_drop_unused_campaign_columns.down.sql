-- Restore unused columns to campaigns table
ALTER TABLE campaigns ADD COLUMN list_ids JSONB;
ALTER TABLE campaigns ADD COLUMN exclusion_list_ids JSONB;
ALTER TABLE campaigns ADD COLUMN state VARCHAR(20);
ALTER TABLE campaigns ADD COLUMN send_at TIMESTAMPTZ;
ALTER TABLE campaigns ADD COLUMN send_in_user_timezone BOOLEAN DEFAULT false;
ALTER TABLE campaigns ADD COLUMN type VARCHAR(255);

-- Restore indexes
CREATE INDEX campaigns_send_at_idx ON campaigns(send_at);
CREATE INDEX campaigns_project_state_idx ON campaigns(project_id, state);
