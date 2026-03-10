-- Drop unused columns from campaigns table
-- These columns are no longer used by the application:
-- - list_ids: campaign targeting now handled differently
-- - exclusion_list_ids: campaign targeting now handled differently
-- - state: campaign state no longer stored on the campaign
-- - send_at: scheduling now handled differently
-- - send_in_user_timezone: scheduling now handled differently
-- - type: campaign type distinction no longer used

-- First drop the indexes that reference these columns
DROP INDEX IF EXISTS campaigns_send_at_idx;
DROP INDEX IF EXISTS campaigns_project_state_idx;

-- Then drop the columns
ALTER TABLE campaigns
    DROP COLUMN IF EXISTS list_ids,
    DROP COLUMN IF EXISTS exclusion_list_ids,
    DROP COLUMN IF EXISTS state,
    DROP COLUMN IF EXISTS send_at,
    DROP COLUMN IF EXISTS send_in_user_timezone,
    DROP COLUMN IF EXISTS type;
