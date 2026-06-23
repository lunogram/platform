// RBAC preset model for the API & Clients permission picker.
//
// This mirrors the authorization model in internal/rbac/model.go: the
// per-resource role required for each verb, and the project-role hierarchy. It
// lets the console render a role preset as concrete (resource, verb) checkboxes
// and seed the custom matrix from a preset.
//
// IMPORTANT: keep in sync with internal/rbac/model.go (the `resources` table and
// the project-role hierarchy). This duplication is deliberate for a UI-only
// preview; the backend remains the single source of enforcement. A future
// management endpoint exposing presets would remove the need to mirror it here.

import type { GrantVerb, PermissionGrant, ProjectRole } from "@/types"
import { grantVerbs } from "@/types"

// resourceRoleRequirements is the project role required to perform each verb on
// each resource — the `resources` table in model.go.
const resourceRoleRequirements: Record<string, Record<GrantVerb, ProjectRole>> = {
    users: { read: "support", create: "client", update: "client", delete: "client" },
    events: { read: "support", create: "client", update: "client", delete: "editor" },
    inbox: { read: "support", create: "client", update: "client", delete: "admin" },
    scheduled: { read: "support", create: "client", update: "client", delete: "editor" },
    devices: { read: "support", create: "client", update: "client", delete: "client" },
    organizations: { read: "support", create: "client", update: "client", delete: "client" },
    subscriptions: { read: "support", create: "editor", update: "editor", delete: "admin" },
    campaigns: { read: "support", create: "editor", update: "editor", delete: "editor" },
    broadcasts: { read: "support", create: "editor", update: "editor", delete: "editor" },
    journeys: { read: "support", create: "editor", update: "editor", delete: "editor" },
    lists: { read: "support", create: "editor", update: "editor", delete: "editor" },
    tags: { read: "support", create: "editor", update: "editor", delete: "editor" },
    templates: { read: "support", create: "editor", update: "editor", delete: "editor" },
    locales: { read: "support", create: "editor", update: "editor", delete: "editor" },
    documents: { read: "support", create: "editor", update: "editor", delete: "editor" },
    actions: { read: "support", create: "editor", update: "editor", delete: "admin" },
    providers: { read: "support", create: "admin", update: "admin", delete: "admin" },
    push_providers: { read: "support", create: "admin", update: "admin", delete: "admin" },
    sender_identities: { read: "support", create: "admin", update: "admin", delete: "admin" },
}

// roleSatisfies maps a held project role to the set of required roles it covers.
// Mirrors the model.go hierarchy: editor implies both client and support, admin
// implies everything. support and client are independent branches (support is
// read-only, client is write-only) — neither implies the other.
const roleSatisfies: Record<ProjectRole, ReadonlySet<ProjectRole>> = {
    support: new Set(["support"]),
    client: new Set(["client"]),
    editor: new Set(["support", "client", "editor"]),
    admin: new Set(["support", "client", "editor", "admin"]),
}

// roleAllows reports whether a held role can perform a verb on a resource.
export function roleAllows(role: ProjectRole, resource: string, verb: GrantVerb): boolean {
    const required = resourceRoleRequirements[resource]?.[verb]
    return required ? roleSatisfies[role].has(required) : false
}

// presetGrants expands a role preset into the explicit (resource, verb) grants
// it confers, so a role can seed the custom matrix.
export function presetGrants(role: ProjectRole): PermissionGrant[] {
    const grants: PermissionGrant[] = []
    for (const resource of Object.keys(resourceRoleRequirements)) {
        for (const verb of grantVerbs) {
            if (roleAllows(role, resource, verb)) grants.push({ resource, verb })
        }
    }
    return grants
}

export interface RolePreset {
    role: ProjectRole
    label: string
    summary: string
}

// rolePresets are the curated permission bundles shown as cards, ordered from
// least to most privileged.
export const rolePresets: RolePreset[] = [
    { role: "support", label: "Support", summary: "Read-only access to every resource." },
    { role: "client", label: "Client", summary: "Write-only — create & update, no reads." },
    { role: "editor", label: "Editor", summary: "Read and write content resources." },
    { role: "admin", label: "Admin", summary: "Full access, including providers & settings." },
]

// resourceGroups organize the matrix so the long tail of resources stays
// legible. Audience (end-user) resources lead; administration trails.
export const resourceGroups: { label: string; resources: string[] }[] = [
    {
        label: "Audience",
        resources: [
            "users",
            "events",
            "inbox",
            "scheduled",
            "devices",
            "organizations",
            "subscriptions",
        ],
    },
    {
        label: "Content",
        resources: [
            "campaigns",
            "broadcasts",
            "journeys",
            "lists",
            "tags",
            "templates",
            "locales",
            "documents",
            "actions",
        ],
    },
    {
        label: "Administration",
        resources: ["providers", "push_providers", "sender_identities"],
    },
]

const allResources = resourceGroups.flatMap((g) => g.resources)

function humanJoin(parts: string[]): string {
    if (parts.length <= 1) return parts.join("")
    if (parts.length === 2) return `${parts[0]} and ${parts[1]}`
    return `${parts.slice(0, -1).join(", ")}, and ${parts[parts.length - 1]}`
}

// describePermissions turns a set of grants into a plain-English sentence, e.g.
// "Can read & write Audience and read Content, on the authenticated user's own
// data." It summarizes per resource group rather than per cell.
export function describePermissions(grants: PermissionGrant[], ownScope: boolean): string {
    const scope = ownScope ? "on the authenticated user's own data" : "across all users' data"
    if (grants.length === 0) return "No access yet — pick a preset or grant a permission below."

    const granted = new Set(grants.map((g) => `${g.resource}:${g.verb}`))
    const everything = allResources.every((r) => grantVerbs.every((v) => granted.has(`${r}:${v}`)))
    if (everything) return `Full access to everything, ${scope}.`

    // Group names by access level so shared levels collapse into one clause,
    // e.g. "read-only access to Audience, Content, and Administration".
    const levelPhrase = {
        full: "full access to",
        rw: "read & write access to",
        read: "read-only access to",
        write: "write access to",
    }
    const order: (keyof typeof levelPhrase)[] = ["full", "rw", "read", "write"]
    const byLevel = new Map<keyof typeof levelPhrase, string[]>()

    for (const group of resourceGroups) {
        const verbs = new Set<GrantVerb>()
        for (const r of group.resources)
            for (const v of grantVerbs) if (granted.has(`${r}:${v}`)) verbs.add(v)
        if (verbs.size === 0) continue

        const hasRead = verbs.has("read")
        const hasWrite = grantVerbs.some((v) => v !== "read" && verbs.has(v))
        const level = grantVerbs.every((v) => verbs.has(v))
            ? "full"
            : hasRead && hasWrite
              ? "rw"
              : hasRead
                ? "read"
                : "write"
        byLevel.set(level, [...(byLevel.get(level) ?? []), group.label])
    }

    const clauses = order
        .filter((l) => byLevel.has(l))
        .map((l) => `${levelPhrase[l]} ${humanJoin(byLevel.get(l)!)}`)

    return `Has ${clauses.join("; ")}, ${scope}.`
}
