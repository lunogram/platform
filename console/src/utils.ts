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

export function debounce(fn: Function, ms = 300) {
    let timeoutId: ReturnType<typeof setTimeout>
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

export function createComparator<T>(getter: (o: T) => any, desc = false) {
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

export function groupBy<T>(arr: T[], fn: (item: T) => any) {
    return arr.reduce<Record<string, T[]>>((prev, curr) => {
        const groupKey = fn(curr)
        const group = prev[groupKey] || []
        group.push(curr)
        return { ...prev, [groupKey]: group }
    }, {})
}

export function groupByKey<T>(arr: T[], key: keyof T) {
    return groupBy(arr, (item) => item[key])
}

export function arrayMove<T>(arr: T[], currentIndex: number, targetIndex: number) {
    if (targetIndex >= arr.length) {
        let k = targetIndex - arr.length + 1
        while (k--) {
            ;(arr as any).push(undefined)
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
