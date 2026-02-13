-- Add unique constraint for project_admins to prevent duplicate admin assignments (idempotent)
CREATE UNIQUE INDEX IF NOT EXISTS project_admins_project_admin_uniq ON public.project_admins USING btree (project_id, admin_id) WHERE (deleted_at IS NULL);
