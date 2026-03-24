CREATE UNIQUE INDEX devices_project_device_active_idx 
ON devices (project_id, device_id, deleted_at) 
WHERE deleted_at IS NULL;