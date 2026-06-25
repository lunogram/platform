-- next_schedule_occurrence returns the next occurrence of a recurring schedule:
-- the smallest N > current_occurrence for which (anchor + N * step) is strictly
-- after as_of, together with that N so it can be persisted. The timestamp is
-- always multiplied from the anchor (anchor + N * step) rather than chained
-- (prev + step), which avoids the drift that calendar intervals such as
-- '1 month' would otherwise accumulate via month-end clamping.
--
-- It replaces the previous generate_series(1, 10000) scan, which both capped how
-- far ahead a schedule could be advanced (a schedule dormant for more than 10000
-- intervals could never resume) and was O(N) in the number of skipped occurrences.
--
-- Accuracy: the result is EXACT, not approximate. The epoch-based estimate only
-- chooses where the correction loops start -- it never enters the result. The two
-- loops below settle on N purely via exact interval arithmetic (anchor + n * step),
-- so the returned N satisfies, for any strictly-increasing (positive) interval:
--   (1) anchor + N       * step >  as_of   (in the future)
--   (2) anchor + (N - 1) * step <= as_of   (it is the FIRST such occurrence)
--                                          unless N = current_occurrence + 1
--   (3) N >= current_occurrence + 1        (never moves backwards)
-- This is the same triple the old scan satisfied, so behaviour is identical where
-- the old scan was in range and well-defined beyond it. The epoch estimate lands
-- close to N, so the loops do little work, and there is no cap on the gap.
CREATE OR REPLACE FUNCTION next_schedule_occurrence(
    anchor             TIMESTAMPTZ,
    step               INTERVAL,
    current_occurrence BIGINT,
    as_of              TIMESTAMPTZ DEFAULT NOW()
) RETURNS TABLE(next_at TIMESTAMPTZ, occurrence BIGINT)
LANGUAGE plpgsql
STABLE
AS $$
DECLARE
    n         BIGINT;
    step_secs DOUBLE PRECISION := EXTRACT(EPOCH FROM step);
BEGIN
    IF step_secs <= 0 THEN
        RAISE EXCEPTION 'schedule interval must be positive, got %', step;
    END IF;

    -- O(1) starting estimate from the average interval length. This only seeds
    -- the correction loops below; the exact N comes from them, not from here.
    n := GREATEST(
        current_occurrence + 1,
        FLOOR(EXTRACT(EPOCH FROM (as_of - anchor)) / step_secs)::BIGINT
    );

    -- Correct exactly using interval arithmetic only. First walk back while the
    -- previous occurrence is still in the future (estimate overshot), enforcing
    -- property (3) via the n > current_occurrence + 1 guard. Then walk forward
    -- while the current occurrence is not yet past as_of (estimate undershot).
    -- On exit, anchor + (n-1)*step <= as_of < anchor + n*step, i.e. properties
    -- (1) and (2) hold. Both loops run only a few iterations.
    WHILE n > current_occurrence + 1 AND anchor + (n - 1) * step > as_of LOOP
        n := n - 1;
    END LOOP;

    WHILE anchor + n * step <= as_of LOOP
        n := n + 1;
    END LOOP;

    RETURN QUERY SELECT anchor + n * step, n;
END;
$$;
