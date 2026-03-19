-- Move link_wrap settings from projects table to providers table.
-- Previously, link_wrap_email and link_wrap_push were project-level booleans.
-- This migration moves them to a per-provider link_wrap boolean.

-- Step 1: Add link_wrap column to providers
ALTER TABLE providers ADD COLUMN link_wrap BOOLEAN DEFAULT false;

-- Step 2: Migrate existing data from projects to providers
-- For projects with link_wrap_email = true, set link_wrap = true on all email providers in that project
UPDATE providers p
SET link_wrap = true
FROM projects proj
WHERE p.project_id = proj.id
  AND p.channel = 'email'
  AND proj.link_wrap_email = true;

-- For projects with link_wrap_push = true, set link_wrap = true on all push providers in that project
UPDATE providers p
SET link_wrap = true
FROM projects proj
WHERE p.project_id = proj.id
  AND p.channel = 'push'
  AND proj.link_wrap_push = true;

-- Step 3: Drop the old columns from projects
ALTER TABLE projects
    DROP COLUMN IF EXISTS link_wrap_email,
    DROP COLUMN IF EXISTS link_wrap_push;
