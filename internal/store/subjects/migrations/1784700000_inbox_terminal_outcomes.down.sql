DROP INDEX IF EXISTS organization_inbox_messages_due_idx;
DROP INDEX IF EXISTS user_inbox_messages_due_idx;

ALTER TABLE organization_inbox_messages
    DROP COLUMN IF EXISTS recipient_timezone,
    DROP COLUMN IF EXISTS class,
    DROP COLUMN IF EXISTS failure_reason,
    DROP COLUMN IF EXISTS failed_at;

ALTER TABLE user_inbox_messages
    DROP COLUMN IF EXISTS recipient_timezone,
    DROP COLUMN IF EXISTS class,
    DROP COLUMN IF EXISTS failure_reason,
    DROP COLUMN IF EXISTS failed_at;

CREATE INDEX user_inbox_messages_due_idx
    ON user_inbox_messages(scheduled_at ASC)
    WHERE deleted_at IS NULL AND sent_at IS NULL;

CREATE INDEX organization_inbox_messages_due_idx
    ON organization_inbox_messages(scheduled_at ASC)
    WHERE deleted_at IS NULL AND sent_at IS NULL;
