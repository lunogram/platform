-- Restore has_push_device column
ALTER TABLE users ADD COLUMN has_push_device bool DEFAULT false;

-- Populate has_push_device based on devices array
UPDATE users SET has_push_device = true WHERE devices IS NOT NULL AND jsonb_array_length(devices) > 0;
