-- Reverse secret hashing. The plaintext cannot be recovered from the hash, so
-- the restored value column is left NULL; existing keys must be reissued.

DROP VIEW IF EXISTS project_api_keys;
DROP INDEX IF EXISTS access_policies_secret_hash_uniq;

ALTER TABLE access_policies ADD COLUMN value VARCHAR(255);

ALTER TABLE access_policies DROP COLUMN secret_prefix;
ALTER TABLE access_policies DROP COLUMN secret_hash;

CREATE VIEW project_api_keys AS
    SELECT id, project_id, value, scope, name, description, role,
           created_at, updated_at, deleted_at
    FROM access_policies
    WHERE type = 'api_key';
