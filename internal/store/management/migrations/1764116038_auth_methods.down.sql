-- Reverse the normalized authentication model, restoring project_api_keys.
-- The plaintext secret cannot be recovered from its hash, so the restored value
-- column holds the hash (unique, non-null) and keys must be reissued.

CREATE TABLE project_api_keys (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    value VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    description VARCHAR(2048),
    role VARCHAR(64) NOT NULL DEFAULT 'support',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

INSERT INTO project_api_keys (id, project_id, value, name, description, role, created_at, updated_at, deleted_at)
SELECT m.id, m.project_id, k.secret_hash, m.name, m.description, m.role, m.created_at, m.updated_at, m.deleted_at
FROM auth_methods m
JOIN auth_method_api_keys k ON k.auth_method_id = m.id
WHERE m.type = 'api_key';

CREATE UNIQUE INDEX project_api_keys_value_uniq ON project_api_keys(value);
CREATE INDEX project_api_keys_project_id_idx ON project_api_keys(project_id);
CREATE TRIGGER set_updated_at_project_api_keys BEFORE UPDATE ON project_api_keys
    FOR EACH ROW EXECUTE PROCEDURE set_updated_at();

DROP TABLE auth_method_sessions;
DROP TABLE auth_method_trusted_issuers;
DROP TABLE auth_method_api_keys;
DROP TABLE auth_method_grants;
DROP TABLE auth_methods;
