import { toast } from "sonner"

/**
 * The problem title the platform answers a refused origin with. It is a wire
 * contract with `CrossOriginTitle` in internal/http/auth/auth.go — change one
 * and you must change the other.
 */
const CROSS_ORIGIN_TITLE = "cross-origin request refused"

/** One toast, however many requests are refused. */
const TOAST_ID = "cross-origin-refused"

interface Problem {
    title?: string
    detail?: string
}

function isCrossOriginProblem(status: number | undefined, data: unknown): data is Problem {
    if (status !== 403) return false
    return (data as Problem)?.title === CROSS_ORIGIN_TITLE
}

/**
 * Tells the admin why a request was refused when the reason is one they cannot
 * discover from the screen.
 *
 * A refused origin is not a permission the admin lacks and not a session that
 * expired: it is a deployment whose PUBLIC_URL disagrees with the address the
 * console is served from, and it silently breaks every write. Without this the
 * console showed a request that simply did nothing.
 */
export function reportCrossOriginRefusal(status: number | undefined, data: unknown): boolean {
    if (!isCrossOriginProblem(status, data)) return false

    toast.error(data.title, { id: TOAST_ID, description: data.detail, duration: Infinity })
    return true
}
