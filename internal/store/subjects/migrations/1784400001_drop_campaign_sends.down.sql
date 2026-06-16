-- Recreate the campaign_sends table as it existed prior to its removal (the
-- combined result of its original creation plus the broadcast_id column and the
-- broadcast-aware composite primary key added by later migrations). Row data is
-- not restored — a dropped table's contents cannot be recovered by a migration.
CREATE TABLE campaign_sends (
    id UUID DEFAULT uuid_generate_v4(),
    campaign_id UUID NOT NULL,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    state VARCHAR(50),
    sent_at TIMESTAMPTZ,
    opened_at TIMESTAMPTZ,
    clicks INTEGER DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    reference_type VARCHAR(255),
    reference_id VARCHAR(255) NOT NULL DEFAULT '0',
    broadcast_id UUID NOT NULL,
    PRIMARY KEY (campaign_id, user_id, broadcast_id, reference_id)
);

CREATE INDEX campaign_sends_user_id_idx ON campaign_sends(user_id);
CREATE INDEX campaign_sends_sent_at_idx ON campaign_sends(sent_at);
CREATE INDEX campaign_sends_state_idx ON campaign_sends(state);
CREATE INDEX campaign_sends_campaign_state_idx ON campaign_sends(campaign_id, state);
CREATE INDEX campaign_sends_broadcast_id_idx ON campaign_sends(broadcast_id);

CREATE TRIGGER set_updated_at_campaign_sends BEFORE UPDATE ON campaign_sends FOR EACH ROW EXECUTE PROCEDURE set_updated_at();
