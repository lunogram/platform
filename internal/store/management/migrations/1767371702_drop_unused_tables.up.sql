-- Drop unused tables that are no longer referenced in the codebase

-- Drop job_locks table (no longer used)
DROP TABLE IF EXISTS job_locks;

-- Drop entity_tags table (no longer used)
DROP TABLE IF EXISTS entity_tags;

-- Drop resources table (no longer used)
DROP TABLE IF EXISTS resources;

-- Drop notifications table (no longer used)
DROP TABLE IF EXISTS notifications;
