import { keepPreviousData, queryOptions } from "@tanstack/react-query"
import api from "@/api"
import type { UUID } from "@/types/common"

type InviteListParams = NonNullable<Parameters<typeof api.invites.list>[1]>

export const memberKeys = {
    all: (projectId: UUID) => ["projects", projectId, "members"] as const,
    invites: (projectId: UUID) => ["projects", projectId, "invites"] as const,
}

/**
 * One page of the project's members. The params are part of the key, so a
 * slow response for an older search or page can't replace a newer one.
 */
export function membersPageQuery(
    projectId: UUID,
    params: { limit: number; offset: number; search?: string },
) {
    return queryOptions({
        queryKey: [...memberKeys.all(projectId), "page", params] as const,
        queryFn: () => api.projectAdmins.search(projectId, params),
        // Keep the current page on screen while the next one loads.
        placeholderData: keepPreviousData,
    })
}

/** The project's members for pickers (the API maximum of 100). */
export function allMembersQuery(projectId: UUID) {
    return queryOptions({
        queryKey: [...memberKeys.all(projectId), "all"] as const,
        queryFn: () => api.projectAdmins.search(projectId, { limit: 100 }),
    })
}

/** One page of the project's invites, filtered by `params`. */
export function invitesPageQuery(projectId: UUID, params: InviteListParams) {
    return queryOptions({
        queryKey: [...memberKeys.invites(projectId), params] as const,
        queryFn: () => api.invites.list(projectId, params),
        placeholderData: keepPreviousData,
    })
}
