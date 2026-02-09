-- Make admins.organization_id NOT NULL
ALTER TABLE admins ALTER COLUMN organization_id SET NOT NULL;
