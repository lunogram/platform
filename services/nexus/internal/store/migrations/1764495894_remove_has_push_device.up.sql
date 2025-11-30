-- Remove has_push_device column as it will be computed from devices array
ALTER TABLE users DROP COLUMN has_push_device;
