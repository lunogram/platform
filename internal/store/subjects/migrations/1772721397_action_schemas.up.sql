-- Action schemas table
-- Stores inferred schema paths from action execution results (Metadata field).
-- References action_id from the management database (cross-database, no FK constraint).
CREATE TABLE action_schemas (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    action_id UUID NOT NULL,
    path VARCHAR(255) NOT NULL,
    data_type data_type NOT NULL,
    visibility project_rule_paths_visibility NOT NULL DEFAULT 'hidden',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_action_schemas_action_id ON action_schemas(action_id);
CREATE UNIQUE INDEX idx_action_schemas_action_path ON action_schemas(action_id, path, data_type);

CREATE TRIGGER set_updated_at_action_schemas BEFORE UPDATE ON action_schemas FOR EACH ROW EXECUTE PROCEDURE set_updated_at();
