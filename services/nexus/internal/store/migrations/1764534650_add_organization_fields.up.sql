ALTER TABLE organizations ADD COLUMN username varchar(255);
ALTER TABLE organizations ADD COLUMN domain varchar(255);
ALTER TABLE organizations ADD COLUMN deleted_at timestamptz;
