ALTER TABLE lists DROP COLUMN refreshed_at;
ALTER TABLE lists DROP COLUMN is_visible;
ALTER TABLE lists DROP COLUMN rule;
ALTER TABLE lists DROP COLUMN rule_id;
ALTER TABLE lists DROP COLUMN state;
ALTER TABLE lists DROP COLUMN IF EXISTS users_count;

DROP TABLE IF EXISTS rule_evaluations;
DROP TABLE IF EXISTS rules;

CREATE TABLE rules (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    rule JSONB NOT NULL,
    depends_on_events BOOLEAN NOT NULL DEFAULT FALSE,
    depends_on_users BOOLEAN NOT NULL DEFAULT FALSE,
    version INT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_rules_project_id ON rules(project_id);
CREATE TABLE rules_events (
    rule_id UUID NOT NULL REFERENCES rules(id) ON DELETE RESTRICT,
    event_id UUID NOT NULL REFERENCES events(id) ON DELETE RESTRICT,
    PRIMARY KEY (rule_id, event_id)
);

CREATE INDEX idx_rules_events_rule_id ON rules_events(rule_id);
CREATE INDEX idx_rules_events_event_id ON rules_events(event_id);

CREATE TRIGGER increment_version_rules BEFORE UPDATE ON rules FOR EACH ROW EXECUTE PROCEDURE increment_version();
CREATE TRIGGER set_rules_updated_at BEFORE UPDATE ON rules FOR EACH ROW EXECUTE FUNCTION set_updated_at();

ALTER TABLE lists ADD COLUMN rule_id UUID REFERENCES rules(id) ON DELETE RESTRICT;
ALTER TABLE user_list RENAME TO lists_users;

DROP TABLE IF EXISTS lists_users;
DROP TABLE IF EXISTS user_events;

CREATE TABLE user_events (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    event_id UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    data JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_user_events_user_id ON user_events(user_id);
CREATE INDEX idx_user_events_event_id ON user_events(event_id);
CREATE INDEX idx_user_events_created_at ON user_events(created_at);
CREATE INDEX idx_user_events_user_event ON user_events(user_id, event_id);

CREATE TABLE list_users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    list_id UUID NOT NULL REFERENCES lists(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_list_users_list_id ON list_users(list_id);
CREATE INDEX idx_list_users_user_id ON list_users(user_id);
CREATE INDEX idx_list_users_list_user ON list_users(list_id, user_id);
CREATE UNIQUE INDEX idx_list_users_unique_active ON list_users(list_id, user_id);

CREATE INDEX idx_user_events_data ON user_events USING GIN (data);