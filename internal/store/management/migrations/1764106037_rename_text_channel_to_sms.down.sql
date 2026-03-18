-- Revert channel value "sms" back to "text" in tables that were updated
-- by the corresponding up migration.

UPDATE campaigns SET channel = 'text' WHERE channel = 'sms';
UPDATE templates SET type = 'text' WHERE type = 'sms';
UPDATE subscriptions SET channel = 'text' WHERE channel = 'sms';
