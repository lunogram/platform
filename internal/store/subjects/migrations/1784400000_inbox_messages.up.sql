CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE TABLE user_inbox_messages (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id UUID NOT NULL,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    external_id TEXT,
    channel TEXT NOT NULL,
    sender_identity_id UUID,
    campaign_id UUID,
    broadcast_id UUID,
    content JSONB NOT NULL DEFAULT '{}'::jsonb,
    data JSONB NOT NULL DEFAULT '{}'::jsonb,
    tags TEXT[] NOT NULL DEFAULT '{}',
    priority SMALLINT NOT NULL DEFAULT 3,
    source TEXT,
    scheduled_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    sent_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ,
    read_at TIMESTAMPTZ,
    archived_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    CONSTRAINT user_inbox_messages_channel_check CHECK (channel IN ('inbox', 'email', 'sms', 'push')),
    CONSTRAINT user_inbox_messages_priority_check CHECK (priority BETWEEN 1 AND 5),
    CONSTRAINT user_inbox_messages_source_check CHECK (source IS NULL OR source IN ('inbox', 'campaign', 'broadcast', 'journey')),
    CONSTRAINT user_inbox_messages_sender_identity_check CHECK (
        (channel IN ('email', 'sms') AND sender_identity_id IS NOT NULL)
        OR (channel IN ('inbox', 'push') AND sender_identity_id IS NULL)
    )
);

CREATE UNIQUE INDEX user_inbox_messages_external_id_idx
    ON user_inbox_messages(project_id, user_id, channel, external_id)
    WHERE external_id IS NOT NULL;

CREATE INDEX user_inbox_messages_feed_idx
    ON user_inbox_messages(project_id, user_id, scheduled_at DESC, created_at DESC)
    WHERE deleted_at IS NULL;

CREATE INDEX user_inbox_messages_due_idx
    ON user_inbox_messages(scheduled_at ASC)
    WHERE deleted_at IS NULL AND sent_at IS NULL;

CREATE INDEX user_inbox_messages_unread_idx
    ON user_inbox_messages(project_id, user_id, created_at DESC)
    WHERE read_at IS NULL AND archived_at IS NULL AND deleted_at IS NULL;

CREATE INDEX user_inbox_messages_channel_idx
    ON user_inbox_messages(project_id, user_id, channel)
    WHERE deleted_at IS NULL;

CREATE INDEX user_inbox_messages_source_idx
    ON user_inbox_messages(project_id, user_id, source)
    WHERE deleted_at IS NULL AND source IS NOT NULL;

CREATE INDEX user_inbox_messages_campaign_idx
    ON user_inbox_messages(project_id, campaign_id)
    WHERE deleted_at IS NULL AND campaign_id IS NOT NULL;

CREATE INDEX user_inbox_messages_broadcast_idx
    ON user_inbox_messages(project_id, broadcast_id)
    WHERE deleted_at IS NULL AND broadcast_id IS NOT NULL;

CREATE INDEX user_inbox_messages_tags_idx ON user_inbox_messages USING GIN(tags) WHERE deleted_at IS NULL;
CREATE INDEX user_inbox_messages_data_idx ON user_inbox_messages USING GIN(data) WHERE deleted_at IS NULL;
CREATE INDEX user_inbox_messages_content_title_trgm_idx ON user_inbox_messages USING GIN((content->>'title') gin_trgm_ops) WHERE deleted_at IS NULL;
CREATE INDEX user_inbox_messages_content_body_trgm_idx ON user_inbox_messages USING GIN((content->>'body') gin_trgm_ops) WHERE deleted_at IS NULL;
CREATE INDEX user_inbox_messages_content_subject_trgm_idx ON user_inbox_messages USING GIN((content->>'subject') gin_trgm_ops) WHERE deleted_at IS NULL;

CREATE TRIGGER set_updated_at_user_inbox_messages
    BEFORE UPDATE ON user_inbox_messages
    FOR EACH ROW EXECUTE PROCEDURE set_updated_at();

CREATE TABLE organization_inbox_messages (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id UUID NOT NULL,
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    external_id TEXT,
    channel TEXT NOT NULL,
    sender_identity_id UUID,
    campaign_id UUID,
    broadcast_id UUID,
    content JSONB NOT NULL DEFAULT '{}'::jsonb,
    data JSONB NOT NULL DEFAULT '{}'::jsonb,
    tags TEXT[] NOT NULL DEFAULT '{}',
    priority SMALLINT NOT NULL DEFAULT 3,
    source TEXT,
    scheduled_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    sent_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ,
    read_at TIMESTAMPTZ,
    archived_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    CONSTRAINT organization_inbox_messages_channel_check CHECK (channel IN ('inbox', 'email', 'sms', 'push')),
    CONSTRAINT organization_inbox_messages_priority_check CHECK (priority BETWEEN 1 AND 5),
    CONSTRAINT organization_inbox_messages_source_check CHECK (source IS NULL OR source IN ('inbox', 'campaign', 'broadcast', 'journey')),
    CONSTRAINT organization_inbox_messages_sender_identity_check CHECK (
        (channel IN ('email', 'sms') AND sender_identity_id IS NOT NULL)
        OR (channel IN ('inbox', 'push') AND sender_identity_id IS NULL)
    )
);

CREATE UNIQUE INDEX organization_inbox_messages_external_id_idx
    ON organization_inbox_messages(project_id, organization_id, channel, external_id)
    WHERE external_id IS NOT NULL;

CREATE INDEX organization_inbox_messages_feed_idx
    ON organization_inbox_messages(project_id, organization_id, scheduled_at DESC, created_at DESC)
    WHERE deleted_at IS NULL;

CREATE INDEX organization_inbox_messages_due_idx
    ON organization_inbox_messages(scheduled_at ASC)
    WHERE deleted_at IS NULL AND sent_at IS NULL;

CREATE INDEX organization_inbox_messages_unread_idx
    ON organization_inbox_messages(project_id, organization_id, created_at DESC)
    WHERE read_at IS NULL AND archived_at IS NULL AND deleted_at IS NULL;

CREATE INDEX organization_inbox_messages_channel_idx
    ON organization_inbox_messages(project_id, organization_id, channel)
    WHERE deleted_at IS NULL;

CREATE INDEX organization_inbox_messages_source_idx
    ON organization_inbox_messages(project_id, organization_id, source)
    WHERE deleted_at IS NULL AND source IS NOT NULL;

CREATE INDEX organization_inbox_messages_campaign_idx
    ON organization_inbox_messages(project_id, campaign_id)
    WHERE deleted_at IS NULL AND campaign_id IS NOT NULL;

CREATE INDEX organization_inbox_messages_broadcast_idx
    ON organization_inbox_messages(project_id, broadcast_id)
    WHERE deleted_at IS NULL AND broadcast_id IS NOT NULL;

CREATE INDEX organization_inbox_messages_tags_idx ON organization_inbox_messages USING GIN(tags) WHERE deleted_at IS NULL;
CREATE INDEX organization_inbox_messages_data_idx ON organization_inbox_messages USING GIN(data) WHERE deleted_at IS NULL;
CREATE INDEX organization_inbox_messages_content_title_trgm_idx ON organization_inbox_messages USING GIN((content->>'title') gin_trgm_ops) WHERE deleted_at IS NULL;
CREATE INDEX organization_inbox_messages_content_body_trgm_idx ON organization_inbox_messages USING GIN((content->>'body') gin_trgm_ops) WHERE deleted_at IS NULL;
CREATE INDEX organization_inbox_messages_content_subject_trgm_idx ON organization_inbox_messages USING GIN((content->>'subject') gin_trgm_ops) WHERE deleted_at IS NULL;

CREATE TRIGGER set_updated_at_organization_inbox_messages
    BEFORE UPDATE ON organization_inbox_messages
    FOR EACH ROW EXECUTE PROCEDURE set_updated_at();
