-- campaign_sends table has been replaced by sent_at on inbox_messages
-- and the sent counter on campaign_broadcasts. This migration is not
-- reversible; restore from backup if needed.

DROP TRIGGER IF EXISTS set_updated_at_organization_inbox_messages ON organization_inbox_messages;
DROP TABLE IF EXISTS organization_inbox_messages;

DROP TRIGGER IF EXISTS set_updated_at_user_inbox_messages ON user_inbox_messages;
DROP TABLE IF EXISTS user_inbox_messages;
