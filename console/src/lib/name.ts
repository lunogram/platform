/**
 * Centralized user/member display name resolution.
 *
 * Accepts any object that *might* carry name-related fields and checks them in
 * a consistent order. Both snake_case and camelCase variants are supported so
 * that the same helper works regardless of whether the data comes from the API,
 * the `data` bag, or an Admin profile.
 *
 * Fallback chain:
 *   full_name / fullName  →
 *   first_name+last_name / firstName+lastName  →
 *   data.full_name / data.fullName  →
 *   data.first_name+data.last_name / data.firstName+data.lastName  →
 *   email  →
 *   phone  →
 *   external_id  →
 *   anonymous_id  →
 *   "Unknown"
 */

type Rec = Record<string, unknown>

/** Safely read a string from an unknown value. */
function str(v: unknown): string | undefined {
    return typeof v === "string" && v.trim() ? v.trim() : undefined
}

/** Try to build a full name from separate first / last fields. */
function fromParts(first: unknown, last: unknown): string | undefined {
    const f = str(first)
    const l = str(last)
    if (f && l) return `${f} ${l}`
    return f ?? l
}

/**
 * Resolve a display name from a record, checking both top-level fields and
 * a nested `data` bag.
 */
function resolveNameFields(obj: Rec): string | undefined {
    // 1. Top-level full name
    const topFull = str(obj.full_name) ?? str(obj.fullName)
    if (topFull) return topFull

    // 2. Top-level first + last
    const topParts = fromParts(obj.first_name ?? obj.firstName, obj.last_name ?? obj.lastName)
    if (topParts) return topParts

    // 3. Nested data bag
    const data = obj.data as Rec | undefined
    if (data && typeof data === "object") {
        const dataFull = str(data.full_name) ?? str(data.fullName)
        if (dataFull) return dataFull

        const dataParts = fromParts(
            data.first_name ?? data.firstName,
            data.last_name ?? data.lastName,
        )
        if (dataParts) return dataParts
    }

    return undefined
}

/**
 * Return the best human-readable display name for a user-like object.
 *
 * Works with `User`, `Admin`, `OrganizationMember`, or any plain object that
 * may carry name fields.
 */
export function getUserDisplayName(user?: Rec | null, fallback = "Unknown"): string {
    if (!user) return fallback

    // Name fields (top-level + data bag)
    const name = resolveNameFields(user)
    if (name) return name

    // Identifiers
    return (
        str(user.email) ??
        str(user.phone) ??
        str(user.external_id) ??
        str(user.anonymous_id) ??
        fallback
    )
}

/**
 * Derive 1–2 character initials from a user's display name.
 *
 * Splits on whitespace, `@`, and `.` so that email-derived names still produce
 * two-letter initials (e.g. "john@acme.com" → "JA").
 */
export function getUserInitials(user?: Rec | null): string {
    const name = getUserDisplayName(user)
    const parts = name.trim().split(/[\s@.]+/)
    if (parts.length >= 2) {
        return (parts[0][0] + parts[parts.length - 1][0]).toUpperCase()
    }
    return name.substring(0, 2).toUpperCase()
}

/**
 * Return secondary text to show below the display name (e.g. email when the
 * name is already shown, or external_id).
 */
export function getUserSubtext(user?: Rec | null): string | null {
    if (!user) return null

    const name = resolveNameFields(user)
    const email = str(user.email)
    const externalId = str(user.external_id)

    if (name && email) return email
    if (name && externalId) return externalId
    if (email && externalId) return externalId

    return null
}
