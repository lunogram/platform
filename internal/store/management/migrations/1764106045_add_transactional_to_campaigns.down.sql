-- Remove transactional flag from campaigns
-- If you added a CHECK constraint, drop it first
-- ALTER TABLE campaigns
--     DROP CONSTRAINT IF EXISTS campaigns_subscription_or_transactional;

ALTER TABLE campaigns
    DROP COLUMN IF EXISTS transactional;
