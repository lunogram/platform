-- Revert link_wrap default back to false.
ALTER TABLE providers ALTER COLUMN link_wrap SET DEFAULT false;
