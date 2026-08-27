DROP INDEX IF EXISTS admins_email_lower_unique;

CREATE UNIQUE INDEX admins_email_unique ON admins(email) WHERE deleted_at IS NULL;

ALTER TABLE admins DROP COLUMN IF EXISTS email_conflict_at;
