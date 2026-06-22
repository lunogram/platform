import { useCallback } from "react"
import { useParams } from "react-router"
import { ScanLine } from "lucide-react"

import {
    grantsFor,
    hasVerifiedSubject,
    restrictableGrants,
    type Client,
    type GrantConstraints,
    type RestrictableResource,
} from "../model"
import { ResourceCreateScope } from "./ResourceCreateScope"
import type { PermissionSelection } from "../../PermissionSelector"
import PermissionSelector from "../../PermissionSelector"
import { describePermissions } from "@/lib/rbac-presets"
import api, { client as apiClient } from "@/api"
import oapiClient from "@/oapi/client"

// loadEventNames lists the project's known event names (event schema), so the
// allow-list can suggest existing events while still letting any new name through.
async function loadEventNames(projectId: string): Promise<string[]> {
    const { data } = await oapiClient.GET(
        "/api/admin/projects/{projectID}/subjects/user/events/schema",
        { params: { path: { projectID: projectId } } },
    )
    return (data?.results ?? []).map((e) => e.name).filter(Boolean)
}

// loadSubscriptionNames lists the project's existing subscriptions by name.
async function loadSubscriptionNames(projectId: string): Promise<string[]> {
    const res = await api.subscriptions.search(projectId, { limit: 100 })
    return (res?.results ?? []).map((s) => s.name).filter(Boolean)
}

// loadScheduledNames lists the project's known scheduled event names. The
// endpoint may not exist yet, so fall back to no suggestions (any name still works).
async function loadScheduledNames(projectId: string): Promise<string[]> {
    try {
        const { data } = await apiClient.get<{ results: { name: string }[] }>(
            `/admin/projects/${projectId}/subjects/user/scheduled/schema`,
        )
        return (data?.results ?? []).map((s) => s.name).filter(Boolean)
    } catch {
        return []
    }
}

// PermissionsFields: a plain-English summary of what the client can do, then the
// preset cards and the resource × verb matrix to refine it. Restrictable
// resources (events, subscriptions) carry an inline create-scope pill on their
// row once create access is granted, limiting which named instances may be made.
export function PermissionsFields({
    client,
    set,
}: {
    client: Client
    set: (patch: Partial<Client>) => void
}) {
    const { projectId = "" } = useParams()
    const ownScope = client.subjectScope === "own" && hasVerifiedSubject(client.identity)
    const summary = describePermissions(grantsFor(client.permissions), ownScope)
    const restrictable = new Set<string>(restrictableGrants(client))

    const setNames = (resource: RestrictableResource, names: string[] | undefined) => {
        const next: GrantConstraints = { ...client.constraints }
        // No names means unrestricted — drop the key rather than store an empty list.
        if (!names || names.length === 0) delete next[resource]
        else next[resource] = names
        set({ constraints: next })
    }

    // Stable per-resource suggestion loaders (re-created only when the project changes).
    const loadEvents = useCallback(() => loadEventNames(projectId), [projectId])
    const loadSubscriptions = useCallback(() => loadSubscriptionNames(projectId), [projectId])
    const loadScheduled = useCallback(() => loadScheduledNames(projectId), [projectId])
    const loaders: Record<RestrictableResource, () => Promise<string[]>> = {
        events: loadEvents,
        subscriptions: loadSubscriptions,
        scheduled: loadScheduled,
    }

    // Show the create-scope pill only for resources that are restrictable and
    // currently have create access; other rows get no aside.
    const renderResourceAside = (resource: string) => {
        if (!restrictable.has(resource)) return null
        const r = resource as RestrictableResource
        return (
            <ResourceCreateScope
                resource={r}
                names={client.constraints[r]}
                onChange={(names) => setNames(r, names)}
                loadSuggestions={loaders[r]}
            />
        )
    }

    return (
        <div className="flex flex-col gap-5">
            <div className="flex items-start gap-2.5 rounded-md bg-surface-soft px-3.5 py-3 text-sm">
                <ScanLine className="mt-px h-4 w-4 shrink-0 text-ink-soft" strokeWidth={1.75} />
                <span className="text-foreground">{summary}</span>
            </div>
            <PermissionSelector
                selection={client.permissions}
                onChange={(permissions: PermissionSelection) => set({ permissions })}
                renderResourceAside={renderResourceAside}
            />
        </div>
    )
}
