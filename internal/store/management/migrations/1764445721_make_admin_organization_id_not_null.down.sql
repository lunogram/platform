-- Revert admins.organization_id to allow NULL
ALTER TABLE admins ALTER COLUMN organization_id DROP NOT NULL;
