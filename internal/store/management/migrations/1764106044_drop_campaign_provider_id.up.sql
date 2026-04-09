-- Remove the provider_id column from campaigns.
-- For email/SMS, the provider is now derived from the sender identity.
-- For push, the provider is derived from project-level push provider defaults.

ALTER TABLE campaigns DROP COLUMN IF EXISTS provider_id;
