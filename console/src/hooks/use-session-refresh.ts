import { useEffect, useRef } from "react"
import api from "@/api"
import type { NetworkError } from "@/api"
import { isPublicPage } from "@/lib/public-paths"

/**
 * Fraction of a session's remaining lifetime to wait before extending it.
 *
 * Refreshing at three quarters leaves a full quarter of slack, so a request that
 * is slow, retried, or fired while the laptop was asleep still lands well before
 * the session lapses.
 */
const REFRESH_AT = 0.75

/** Never schedule tighter than this, so a short-lived session cannot spin. */
const MIN_DELAY_MS = 30_000

function isStatus(error: unknown, status: number) {
    return (error as NetworkError)?.response?.status === status
}

/**
 * Keeps the console session alive while a tab is open.
 *
 * The server owns the session lifetime, so the schedule is derived from the
 * expiry it reports rather than from a constant the client guesses: refresh,
 * read `expires_at`, wait three quarters of what is left, repeat. The first call
 * happens on mount, which is also how the schedule bootstraps — nothing else
 * tells the client when its session ends.
 *
 * Failure is not uniform, and conflating the two kinds is what would produce a
 * loop:
 *
 * - **403** — the session is alive but cannot be extended, which is how an
 *   impersonated session is recorded. Stop scheduling and leave the user where
 *   they are; ejecting them from a working session would be the bug.
 * - **401** — the session is gone. Send them to the login page once, and do not
 *   schedule again.
 * - anything else (offline, a 5xx) — stop scheduling rather than retrying on a
 *   timer. The response interceptor already redirects on the next real request,
 *   so a genuinely dead session is still caught; a transient one recovers on the
 *   next page load.
 */
export function useSessionRefresh(enabled = true) {
    const timer = useRef<ReturnType<typeof setTimeout>>()

    useEffect(() => {
        // Nothing to keep alive on a page reached without a session, and asking
        // would eject its visitor to the login page: the 401 below is the
        // expected answer there, not a lapsed session.
        if (!enabled || isPublicPage()) return

        let cancelled = false

        const schedule = (expiresAt: string) => {
            const remaining = new Date(expiresAt).getTime() - Date.now()
            if (!Number.isFinite(remaining) || remaining <= 0) return
            timer.current = setTimeout(run, Math.max(remaining * REFRESH_AT, MIN_DELAY_MS))
        }

        const run = async () => {
            if (cancelled) return
            try {
                const { expires_at } = await api.auth.refresh()
                if (!cancelled) schedule(expires_at)
            } catch (error) {
                if (cancelled) return
                if (isStatus(error, 401)) api.auth.login()
                // Every other outcome stops the schedule. See the doc comment.
            }
        }

        run().catch(() => {})

        return () => {
            cancelled = true
            clearTimeout(timer.current)
        }
    }, [enabled])
}
