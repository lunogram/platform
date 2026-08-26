-- SMS opt-out compliance state.
--
-- Suppression is keyed on the phone number, never on a user: a STOP can arrive
-- from a number that matches no user record, or from a handset that two user
-- records share. Keying on the number makes the opt-out apply to users created
-- later and to numbers shared across records.
--
-- The table deliberately holds no provider-side identifier and no foreign key
-- to providers or sender identities, so uninstalling a provider can never
-- cascade into opt-out state.
CREATE TABLE sms_suppressions (
    id                UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id        UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    -- E.164 sender, or '*' while opt-out scope is project-wide. The column
    -- exists so per-sender scoping becomes possible without a migration.
    sender_address    TEXT NOT NULL,
    recipient_phone   TEXT NOT NULL,
    state             TEXT NOT NULL CHECK (state IN ('opted_out','opted_in')),
    reason            TEXT NOT NULL,
    source_message_id UUID,
    occurred_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (project_id, sender_address, recipient_phone)
);

-- The send-path check is "is this recipient opted out anywhere in the project",
-- which does not constrain sender_address and so cannot use the UNIQUE index.
CREATE INDEX sms_suppressions_opted_out_idx
    ON sms_suppressions (project_id, recipient_phone)
    WHERE state = 'opted_out';

-- Append-only consent ledger. This is the record produced in a dispute, so
-- rows are never updated or deleted in normal operation and the table carries
-- no foreign keys that could cascade a delete into it.
CREATE TABLE sms_consent_events (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id      UUID NOT NULL,
    recipient_phone TEXT NOT NULL,
    sender_address  TEXT NOT NULL,
    transition      TEXT NOT NULL,
    source          TEXT NOT NULL,
    -- No foreign key: the inbound landing table arrives in a later phase.
    inbound_id      UUID,
    occurred_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX sms_consent_events_recipient_idx
    ON sms_consent_events (project_id, recipient_phone, occurred_at DESC);
