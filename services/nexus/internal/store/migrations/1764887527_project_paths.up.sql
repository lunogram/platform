CREATE TYPE data_type AS ENUM('string', 'number', 'boolean', 'object', 'array');

CREATE TABLE IF NOT EXISTS events (
    "id" uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    "project_id" uuid REFERENCES projects(id) ON DELETE CASCADE,
    "name" VARCHAR(255) NOT NULL,
    "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "deleted_at" timestamptz
);

CREATE UNIQUE INDEX idx_events_project_name ON events(project_id, name);
CREATE TRIGGER set_updated_at_events BEFORE UPDATE ON events FOR EACH ROW EXECUTE PROCEDURE set_updated_at();

CREATE TABLE IF NOT EXISTS event_schemas (
    "id" uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    "event_id" uuid REFERENCES events(id) ON DELETE CASCADE,
    "path" VARCHAR(255) NOT NULL,
    "data_type" data_type NOT NULL,
    "visibility" project_rule_paths_visibility NOT NULL DEFAULT 'hidden',
    "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_event_schemas_event_id ON event_schemas(event_id);
CREATE UNIQUE INDEX idx_event_schemas_event_path ON event_schemas(event_id, path, data_type);
CREATE TRIGGER set_updated_at_event_schemas BEFORE UPDATE ON event_schemas FOR EACH ROW EXECUTE PROCEDURE set_updated_at();

CREATE TABLE IF NOT EXISTS user_schemas (
    "id" uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    "project_id" uuid REFERENCES projects(id) ON DELETE CASCADE,
    "path" VARCHAR(255) NOT NULL,
    "data_type" data_type NOT NULL,
    "visibility" project_rule_paths_visibility NOT NULL DEFAULT 'hidden',
    "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX idx_user_schemas_project_path ON user_schemas(project_id, path, data_type);
CREATE TRIGGER set_updated_at_user_schemas BEFORE UPDATE ON user_schemas FOR EACH ROW EXECUTE PROCEDURE set_updated_at();

DROP TABLE project_rule_paths;