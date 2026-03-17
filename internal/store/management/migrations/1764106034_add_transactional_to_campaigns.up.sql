-- Add transactional flag to campaigns
ALTER TABLE campaigns
    ADD COLUMN IF NOT EXISTS transactional BOOLEAN NOT NULL DEFAULT FALSE;
