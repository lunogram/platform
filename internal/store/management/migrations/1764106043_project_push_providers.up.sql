-- Add a table to store per-platform default push providers at the project level.
-- Each project can have one push provider per platform (ios, android, web).

CREATE TABLE project_push_providers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id),
    provider_id UUID NOT NULL REFERENCES providers(id),
    platform VARCHAR(50) NOT NULL CHECK (platform IN ('ios', 'android', 'web')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX project_push_providers_project_platform_uniq
    ON project_push_providers (project_id, platform);

CREATE TRIGGER set_updated_at_project_push_providers BEFORE UPDATE ON project_push_providers FOR EACH ROW EXECUTE PROCEDURE set_updated_at();
