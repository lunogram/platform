-- Change default for link_wrap on providers to true so that newly created
-- providers have link wrapping enabled by default.
ALTER TABLE providers ALTER COLUMN link_wrap SET DEFAULT true;
