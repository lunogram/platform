-- Create unique index on admins email (idempotent)
CREATE UNIQUE INDEX IF NOT EXISTS admins_email_unique ON admins (email) WHERE deleted_at IS NULL;
