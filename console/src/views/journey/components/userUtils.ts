import type { User } from "@/types"

export function getUserDisplayName(user?: User): string {
    if (!user) return "Unknown"
    if (user.full_name) return user.full_name
    if ((user.data as Record<string, unknown>)?.full_name)
        return (user.data as Record<string, unknown>).full_name as string
    if (user.email) return user.email
    return user.external_id ?? "Unknown"
}

export function getUserInitials(user?: User): string {
    const name = getUserDisplayName(user)
    const parts = name.trim().split(/[\s@.]+/)
    if (parts.length >= 2) {
        return (parts[0][0] + parts[parts.length - 1][0]).toUpperCase()
    }
    return name.substring(0, 2).toUpperCase()
}

export function getUserSubtext(user?: User): string | null {
    if (!user) return null
    if (user.full_name && user.email) return user.email
    if (user.full_name && user.external_id) return user.external_id
    if (user.email && user.external_id) return user.external_id
    return null
}
