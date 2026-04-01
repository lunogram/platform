-- Rename channel value "text" to "sms" across all tables that store channel types.
-- The OAPI Channel enum was unified from [email, text, push] to [email, sms, push]
-- to match the provider and sender identity channel naming convention.

UPDATE campaigns SET channel = 'sms' WHERE channel = 'text';
UPDATE templates SET type = 'sms' WHERE type = 'text';
UPDATE subscriptions SET channel = 'sms' WHERE channel = 'text';
