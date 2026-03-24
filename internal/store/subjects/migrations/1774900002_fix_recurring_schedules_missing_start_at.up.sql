-- Fix recurring schedules that were created without start_at.
--
-- When a recurring schedule was created via the API without an explicit
-- start_at (or scheduled_at), all three anchor columns ended up NULL:
--   start_at     = NULL
--   anchor_at    = NULL
--   scheduled_at = NULL
--
-- This left the scheduler unable to advance them (AdvanceAndGenerate*
-- bails out when anchor_at is NULL) and the API returned the Go zero-time
-- (0001-01-01) as scheduled_at.
--
-- The fix uses created_at as the best approximation of "now" at the time
-- the schedule was originally submitted, then computes the first future
-- occurrence from that anchor. Finally it generates the scheduled events
-- that were never created because scheduled_at was NULL at upsert time.

-- 1. Backfill start_at and anchor_at from created_at where missing.
UPDATE user_schedules
SET start_at  = created_at,
    anchor_at = created_at
WHERE interval IS NOT NULL
  AND start_at IS NULL;

UPDATE organization_schedules
SET start_at  = created_at,
    anchor_at = created_at
WHERE interval IS NOT NULL
  AND start_at IS NULL;

-- 2. Compute the first future scheduled_at for recurring schedules that
--    still have scheduled_at = NULL (i.e. were never advanced).
--    Uses DISTINCT ON to pick the smallest n per schedule that yields a
--    future timestamp — the same logic as computeNextOccurrence in Go.
UPDATE user_schedules us
SET scheduled_at = sub.next_at,
    occurrence   = sub.n
FROM (
    SELECT DISTINCT ON (s.id)
        s.id,
        n,
        s.anchor_at + n * s.interval AS next_at
    FROM user_schedules s,
         generate_series(1, 10000) AS n
    WHERE s.interval IS NOT NULL
      AND s.anchor_at IS NOT NULL
      AND s.scheduled_at IS NULL
      AND s.anchor_at + n * s.interval > NOW()
    ORDER BY s.id, n
) sub
WHERE us.id = sub.id;

UPDATE organization_schedules os
SET scheduled_at = sub.next_at,
    occurrence   = sub.n
FROM (
    SELECT DISTINCT ON (s.id)
        s.id,
        n,
        s.anchor_at + n * s.interval AS next_at
    FROM organization_schedules s,
         generate_series(1, 10000) AS n
    WHERE s.interval IS NOT NULL
      AND s.anchor_at IS NOT NULL
      AND s.scheduled_at IS NULL
      AND s.anchor_at + n * s.interval > NOW()
    ORDER BY s.id, n
) sub
WHERE os.id = sub.id;

-- 3. Generate scheduled events for the fixed user schedules.
--    These schedules never had events created because scheduled_at was
--    NULL at upsert time, causing generateScheduledEvents to bail out.
--    This mirrors the INSERT in ScheduledStore.generateScheduledEvents.
INSERT INTO user_scheduled_events (user_schedule_id, schedule_offset_id, user_id, schedule_id, fire_at, data)
SELECT
    us.id,
    so.id,
    us.user_id,
    us.schedule_id,
    CASE so.direction
        WHEN 'before' THEN us.scheduled_at - so."offset"
        WHEN 'after'  THEN us.scheduled_at + so."offset"
    END,
    us.data
FROM user_schedules us
JOIN schedule_offsets so ON so.schedule_id = us.schedule_id
WHERE us.interval IS NOT NULL
  AND us.scheduled_at IS NOT NULL
  AND us.start_at = us.anchor_at          -- only schedules we just fixed
  AND us.start_at = us.created_at
  AND CASE so.direction
        WHEN 'before' THEN us.scheduled_at - so."offset"
        WHEN 'after'  THEN us.scheduled_at + so."offset"
      END > NOW()
ON CONFLICT (user_schedule_id, schedule_offset_id, user_id) WHERE fired_at IS NULL DO NOTHING;

-- 4. Generate scheduled events for the fixed organization schedules.
--    This mirrors the INSERT in ScheduledStore.generateOrgScheduledEvents.
INSERT INTO organization_scheduled_events (organization_schedule_id, schedule_offset_id, organization_id, schedule_id, fire_at, data)
SELECT
    os.id,
    so.id,
    os.organization_id,
    os.schedule_id,
    CASE so.direction
        WHEN 'before' THEN os.scheduled_at - so."offset"
        WHEN 'after'  THEN os.scheduled_at + so."offset"
    END,
    os.data
FROM organization_schedules os
JOIN schedule_offsets so ON so.schedule_id = os.schedule_id
WHERE os.interval IS NOT NULL
  AND os.scheduled_at IS NOT NULL
  AND os.start_at = os.anchor_at          -- only schedules we just fixed
  AND os.start_at = os.created_at
  AND CASE so.direction
        WHEN 'before' THEN os.scheduled_at - so."offset"
        WHEN 'after'  THEN os.scheduled_at + so."offset"
      END > NOW()
ON CONFLICT (organization_schedule_id, schedule_offset_id, organization_id) WHERE fired_at IS NULL DO NOTHING;
