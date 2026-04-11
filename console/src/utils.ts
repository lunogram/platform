import { parseISO, formatDuration as dateFnsFormatDuration, type Duration } from "date-fns"
import { format, toZonedTime } from "date-fns-tz"
import { organizationRoles, projectRoles } from "./types"
import type { OrganizationRole, Preferences, Project, ProjectRole } from "./types"
import { v4 } from "uuid"
import type { UUID } from "@/types/common"
import type { SignOut } from "@clerk/types"
import { clsx, type ClassValue } from "clsx"
import { twMerge } from "tailwind-merge"

export function createUuid() {
    return v4() as UUID
}

export function round(n: number, places?: number) {
    if (places && places > 0) {
        const f = Math.pow(10, places)
        return Math.round(n * f) / f
    }
    return Math.round(n)
}

// eslint-disable-next-line @typescript-eslint/no-explicit-any
export const prune = (obj: Record<string, any>): Record<string, any> => {
    return Object.fromEntries(Object.entries(obj).filter(([_, v]) => v != null && v !== ""))
}

export function snakeToTitle(snake: string) {
    return (snake ?? "")
        .split("_")
        .map((p) => p.charAt(0).toUpperCase() + p.substring(1))
        .join(" ")
}

export function camelToTitle(camel: string) {
    return camel
        .replace(/([A-Z])/g, (match) => ` ${match}`)
        .replace(/^./, (match) => match.toUpperCase())
        .trim()
}

export function combine(...parts: Array<string | number>) {
    return parts.filter((item) => item != null).join(" ")
}

export function localStorageGetJson<T extends object>(key: string) {
    try {
        const stored = localStorage.getItem(key)
        if (stored) {
            return JSON.parse(stored) as T
        }
    } catch (err) {
        console.warn(err)
    }
}

export function localStorageSetJson<T extends object>(key: string, o: T) {
    localStorage.setItem(key, JSON.stringify(o))
}

export function sessionStorageGetJson<T extends object>(key: string) {
    try {
        const stored = sessionStorage.getItem(key)
        if (stored) {
            return JSON.parse(stored) as T
        }
    } catch (err) {
        console.warn(err)
    }
}

export function sessionStorageSetJson<T extends object>(key: string, o: T) {
    sessionStorage.setItem(key, JSON.stringify(o))
}

// eslint-disable-next-line @typescript-eslint/no-explicit-any
export function debounce(fn: (...args: any[]) => void, ms = 300) {
    let timeoutId: ReturnType<typeof setTimeout>
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    return function (this: any, ...args: any[]) {
        clearTimeout(timeoutId)
        timeoutId = setTimeout(() => fn.apply(this, args), ms)
    }
}

type DateArg = number | string | Date

function parseDate(date: DateArg) {
    if (typeof date === "number") {
        return new Date(date)
    }
    if (typeof date === "string") {
        return parseISO(date)
    }
    return date
}

export function formatDate(
    preferences: Preferences,
    date: DateArg,
    fmt: string = "Pp",
    timeZone = preferences.timeZone,
) {
    const zonedDate = toZonedTime(parseDate(date), timeZone)
    return format(zonedDate, fmt)
}

export function formatDuration(_preferences: Preferences, duration: Duration) {
    return dateFnsFormatDuration(duration, {
        delimiter: ", ",
        // TODO locale
    })
}

export function createComparator<
    T,
    V extends string | number | boolean | Date = string | number | boolean | Date,
>(getter: (o: T) => V, desc = false) {
    return (a: T, b: T) => {
        const av = getter(a)
        const bv = getter(b)
        if (av < bv) {
            return desc ? 1 : -1
        }
        if (av > bv) {
            return desc ? -1 : 1
        }
        return 0
    }
}

export function groupBy<T>(arr: T[], fn: (item: T) => string | number) {
    return arr.reduce<Record<string, T[]>>((prev, curr) => {
        const groupKey = fn(curr)
        const group = prev[groupKey] || []
        group.push(curr)
        return { ...prev, [groupKey]: group }
    }, {})
}

export function groupByKey<T extends Record<K, string | number>, K extends keyof T>(
    arr: T[],
    key: K,
) {
    return groupBy(arr, (item) => item[key])
}

export function arrayMove<T>(arr: T[], currentIndex: number, targetIndex: number) {
    if (targetIndex >= arr.length) {
        let k = targetIndex - arr.length + 1
        while (k--) {
            arr.length = targetIndex + 1
        }
    }
    arr.splice(targetIndex, 0, arr.splice(currentIndex, 1)[0])
    return arr
}

const RECENT_PROJECTS = "recent-projects"

type RecentProjects = Array<{
    id: UUID
    when: number
}>

export function getRecentProjects() {
    return sessionStorageGetJson<RecentProjects>(RECENT_PROJECTS) ?? []
}

export function pushRecentProject(id: UUID) {
    const stored = getRecentProjects()
    const idx = stored.findIndex((p) => p.id === id)
    if (idx !== -1) {
        arrayMove(stored, idx, 0)
    } else {
        stored.unshift({
            id,
            when: Date.now(),
        })
    }
    while (stored.length > 3) {
        stored.pop()
    }
    sessionStorageSetJson(RECENT_PROJECTS, stored)
    return stored
}

export function completedGettingStarted(project: Project) {
    return (
        (project.campaigns_count ?? 0) > 0 &&
        (project.journeys_count ?? 0) > 0 &&
        (project.users_count ?? 0) > 0 &&
        (project.lists_count ?? 0) > 0
    )
}

/**
 * @returns true if user has at least the minRole
 */
export function checkProjectRole(minRole: ProjectRole, currentRole: ProjectRole = "support") {
    return projectRoles.indexOf(minRole) <= projectRoles.indexOf(currentRole)
}

export function checkOrganizationRole(
    minRole: OrganizationRole,
    currentRole: OrganizationRole = "member",
) {
    return organizationRoles.indexOf(minRole) <= organizationRoles.indexOf(currentRole)
}

function clearAuthCookies() {
    const cookieNames = ["__session", "oauth", "csrf_token"]
    for (const name of cookieNames) {
        document.cookie = `${name}=; expires=Thu, 01 Jan 1970 00:00:00 GMT; path=/`
    }
}

export async function logout(signOut: SignOut | undefined) {
    if (signOut) {
        await signOut()
    } else {
        clearAuthCookies()
    }

    window.location.href = "/login"
}

export function cn(...inputs: ClassValue[]) {
    return twMerge(clsx(inputs))
}

/**
 * Format a schedule offset for display.
 * The offset string is a PG INTERVAL (e.g., "00:30:00", "1 year", "1 mon").
 * Direction is "before" or "after".
 */
export function formatOffset(offset: string, direction: string): string {
    if (offset === "00:00:00" || offset === "0" || offset === "0 minutes") {
        return "at scheduled time"
    }
    return `${humanizeInterval(offset)} ${direction}`
}

/**
 * Convert a PG INTERVAL string into a human-readable label.
 *
 * Handles the two formats PostgreSQL uses:
 *  - HH:MM:SS for sub-day intervals (e.g. "00:30:00", "02:00:00")
 *  - Verbose for day+ intervals (e.g. "1 day", "3 days", "1 mon", "2 mons", "1 year")
 */
function humanizeInterval(raw: string): string {
    // Sub-day: "HH:MM:SS"
    const hms = raw.match(/^(\d{2}):(\d{2}):(\d{2})$/)
    if (hms) {
        const h = parseInt(hms[1], 10)
        const m = parseInt(hms[2], 10)
        const parts: string[] = []
        if (h) parts.push(`${h} ${h === 1 ? "hour" : "hours"}`)
        if (m) parts.push(`${m} ${m === 1 ? "minute" : "minutes"}`)
        return parts.length ? parts.join(" ") : raw
    }

    // PG shortens "month(s)" to "mon(s)" — expand it
    return raw.replace(/\bmons?\b/, (m) => (m === "mon" ? "month" : "months"))
}

/**
 * Generate a truncated page number array for pagination controls.
 * Returns page numbers and "..." ellipsis markers.
 */
export function getPageNumbers(current: number, total: number): (number | "...")[] {
    if (total <= 7) {
        return Array.from({ length: total }, (_, i) => i + 1)
    }

    if (current <= 3) {
        return [1, 2, 3, 4, 5, "...", total]
    }

    if (current >= total - 2) {
        return [1, "...", total - 4, total - 3, total - 2, total - 1, total]
    }

    return [1, "...", current - 1, current, current + 1, "...", total]
}

/**
 * Check if a project has a courier provider configured.
 */
export async function hasCourierProvider(projectId: string): Promise<boolean> {
    const { default: oapiClient } = await import("@/oapi/client")
    const { data } = await oapiClient.GET("/api/admin/projects/{projectID}/providers", {
        params: { path: { projectID: projectId } },
    })
    const providers = data?.results ?? []
    return providers.some((p) => p.module === "courier")
}
