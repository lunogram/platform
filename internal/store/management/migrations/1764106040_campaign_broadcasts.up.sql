CREATE TABLE campaign_broadcasts (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id      UUID NOT NULL,
    campaign_id     UUID NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    list_id         UUID NOT NULL,
    list_name       VARCHAR(255) NOT NULL DEFAULT '',
    list_type       VARCHAR(25) NOT NULL DEFAULT '',
    state           VARCHAR(25) NOT NULL DEFAULT 'pending'
                    CONSTRAINT chk_broadcast_state CHECK (state IN ('pending', 'scheduled', 'sending', 'completed', 'failed', 'cancelled')),
    total           INTEGER NOT NULL DEFAULT 0,
    error           TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at      TIMESTAMPTZ,
    completed_at    TIMESTAMPTZ
);

CREATE INDEX idx_campaign_broadcasts_project ON campaign_broadcasts(project_id);
CREATE INDEX idx_campaign_broadcasts_campaign ON campaign_broadcasts(campaign_id);
CREATE INDEX idx_campaign_broadcasts_list ON campaign_broadcasts(list_id);
