CREATE UNIQUE INDEX admins_email_unique ON admins (email) WHERE deleted_at IS NULL;
