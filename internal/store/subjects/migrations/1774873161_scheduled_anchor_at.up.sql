-- Add anchor_at column to user_schedules and organization_schedules.
-- anchor_at is the reference point from which occurrence is computed:
--   scheduled_at = anchor_at + occurrence * interval
--
-- For new schedules, anchor_at starts equal to start_at. When scheduled_at
-- is explicitly overridden (e.g. via PATCH), anchor_at is set to the new
-- scheduled_at and occurrence resets to 0, preserving the original start_at
-- as historical data.

ALTER TABLE user_schedules
    ADD COLUMN IF NOT EXISTS anchor_at TIMESTAMPTZ;

ALTER TABLE organization_schedules
    ADD COLUMN IF NOT EXISTS anchor_at TIMESTAMPTZ;

-- Backfill: set anchor_at = start_at for all existing recurring schedules.
UPDATE user_schedules SET anchor_at = start_at WHERE start_at IS NOT NULL AND anchor_at IS NULL;
UPDATE organization_schedules SET anchor_at = start_at WHERE start_at IS NOT NULL AND anchor_at IS NULL;
