DROP INDEX IF EXISTS devices_project_device_active_idx;

CREATE UNIQUE INDEX devices_project_device_uniq ON devices(project_id, device_id);
