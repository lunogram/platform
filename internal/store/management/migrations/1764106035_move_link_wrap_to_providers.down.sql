-- Reverse: move link_wrap from providers back to projects

-- 1. Add link_wrap_email and link_wrap_push columns back to projects
ALTER TABLE projects
    ADD COLUMN link_wrap_email BOOLEAN DEFAULT false,
    ADD COLUMN link_wrap_push BOOLEAN DEFAULT false;

-- 2. Migrate data back: if any email provider in a project has link_wrap = true,
--    set link_wrap_email = true on the project. Same for push.
UPDATE projects p
SET link_wrap_email = true
FROM providers prov
WHERE prov.project_id = p.id
  AND prov.channel = 'email'
  AND prov.link_wrap = true;

UPDATE projects p
SET link_wrap_push = true
FROM providers prov
WHERE prov.project_id = p.id
  AND prov.channel = 'push'
  AND prov.link_wrap = true;

-- 3. Drop link_wrap column from providers
ALTER TABLE providers DROP COLUMN IF EXISTS link_wrap;
