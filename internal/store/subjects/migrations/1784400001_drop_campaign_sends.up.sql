-- The campaign_sends table is superseded by the inbox_messages tables: per-send
-- delivery state now lives in sent_at/read_at on {user,organization}_inbox_messages
-- and aggregate broadcast progress lives in the sent counter on campaign_broadcasts.
--
-- This drop is kept in its own migration (rather than bundled with the inbox
-- table creation) so each migration has a single, reversible responsibility.
-- The accompanying down migration recreates the table structure; it does not
-- restore row data, which is inherent to dropping a table.
DROP TABLE IF EXISTS campaign_sends CASCADE;
