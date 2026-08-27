-- Messages a broadcast will never deliver (suppressed recipients, permanent
-- provider rejections) settle here rather than in `sent`. `sent` is reported to
-- the customer as the number of messages that actually went out, so counting a
-- deliberately blocked message there would misstate it; a broadcast is complete
-- once sent + failed reaches total.
ALTER TABLE campaign_broadcasts ADD COLUMN failed INTEGER NOT NULL DEFAULT 0;
