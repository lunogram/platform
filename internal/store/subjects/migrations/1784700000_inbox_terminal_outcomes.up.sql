-- Terminal outcomes for inbox messages.
--
-- Until now a permanently failed send was indistinguishable from one that had
-- not been attempted yet: both are rows where sent_at is still NULL. failed_at
-- plus failure_reason give the dispatch path a terminal state to settle on.
--
-- class gates a compliance bypass. It is a column rather than a value in the
-- user-controllable tags array precisely because callers must not be able to
-- grant themselves that bypass.
--
-- recipient_timezone is recorded but not yet enforced; it exists so a later
-- quiet-hours rollout can be measured against real history.
ALTER TABLE user_inbox_messages
    ADD COLUMN failed_at TIMESTAMPTZ,
    ADD COLUMN failure_reason TEXT,
    ADD COLUMN class TEXT NOT NULL DEFAULT 'standard' CHECK (class IN ('standard','compliance')),
    ADD COLUMN recipient_timezone TEXT;

ALTER TABLE organization_inbox_messages
    ADD COLUMN failed_at TIMESTAMPTZ,
    ADD COLUMN failure_reason TEXT,
    ADD COLUMN class TEXT NOT NULL DEFAULT 'standard' CHECK (class IN ('standard','compliance')),
    ADD COLUMN recipient_timezone TEXT;

-- The due scan looks for unsettled messages. Now that failure is terminal, a
-- failed message is settled and must drop out of the scan, so the existing
-- partial index is replaced by one whose predicate matches the new query.
DROP INDEX IF EXISTS user_inbox_messages_due_idx;
CREATE INDEX user_inbox_messages_due_idx
    ON user_inbox_messages(scheduled_at ASC)
    WHERE deleted_at IS NULL AND sent_at IS NULL AND failed_at IS NULL;

DROP INDEX IF EXISTS organization_inbox_messages_due_idx;
CREATE INDEX organization_inbox_messages_due_idx
    ON organization_inbox_messages(scheduled_at ASC)
    WHERE deleted_at IS NULL AND sent_at IS NULL AND failed_at IS NULL;
