-- Remove duplicate locale keys per project (keep the earliest created)
DELETE FROM locales a
USING locales b
WHERE a.project_id = b.project_id
  AND a.key = b.key
  AND a.created_at > b.created_at;

-- Add unique constraint on (project_id, key) to prevent duplicate locale keys per project
ALTER TABLE locales ADD CONSTRAINT locales_project_id_key_unique UNIQUE (project_id, key);
