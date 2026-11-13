import { queryOptions, type QueryClient } from "@tanstack/react-query"
import api from "@/api"
import { oapiClient } from "@/oapi/client"
import { unwrap } from "@/oapi/unwrap"
import type { UUID } from "@/types/common"

// The most items the API returns for one list request.
export const API_PAGE_LIMIT = 100

/**
 * @returns every item of a paginated list, requesting pages of API_PAGE_LIMIT
 * until the reported total is reached.
 */
export async function fetchAllPages<T>(
    fetchPage: (offset: number) => Promise<{ results?: T[] | null; total: number }>,
): Promise<T[]> {
    const items: T[] = []
    for (;;) {
        const page = await fetchPage(items.length)
        const results = page.results ?? []
        items.push(...results)
        if (results.length === 0 || items.length >= page.total) return items
    }
}

export const providerKeys = {
    all: (projectId: UUID) => ["projects", projectId, "providers"] as const,
    list: (projectId: UUID) => [...providerKeys.all(projectId), "list"] as const,
    meta: (projectId: UUID) => [...providerKeys.all(projectId), "meta"] as const,
    detail: (projectId: UUID, providerId: UUID) =>
        [...providerKeys.all(projectId), "detail", providerId] as const,
}

/**
 * Every provider of the project. The one list used by every page, so none of
 * them misses providers past an arbitrary page size.
 */
export function providersQuery(projectId: UUID) {
    return queryOptions({
        queryKey: providerKeys.list(projectId),
        queryFn: () =>
            fetchAllPages(async (offset) =>
                unwrap(
                    await oapiClient.GET("/api/admin/projects/{projectID}/providers", {
                        params: {
                            path: { projectID: projectId },
                            query: { limit: API_PAGE_LIMIT, offset },
                        },
                    }),
                ),
            ),
    })
}

/** The provider modules the project can configure. */
export function providersMetaQuery(projectId: UUID) {
    return queryOptions({
        queryKey: providerKeys.meta(projectId),
        queryFn: async () =>
            unwrap(
                await oapiClient.GET("/api/admin/projects/{projectID}/providers/meta", {
                    params: { path: { projectID: projectId } },
                }),
            ) ?? [],
    })
}

/**
 * One provider with its full configuration, or null when the project has no
 * provider with this id.
 *
 * The get endpoint is addressed by module as well as id, and edit links carry
 * only the id, so the module is looked up in the full provider list.
 */
export function providerQuery(queryClient: QueryClient, projectId: UUID, providerId: UUID) {
    return queryOptions({
        queryKey: providerKeys.detail(projectId, providerId),
        queryFn: async () => {
            const providers = await queryClient.fetchQuery(providersQuery(projectId))
            const found = providers.find((p) => p.id === providerId)
            if (!found) return null
            return unwrap(
                await oapiClient.GET(
                    "/api/admin/projects/{projectID}/providers/{type}/{providerID}",
                    {
                        params: {
                            path: {
                                projectID: projectId,
                                type: found.module,
                                providerID: found.id,
                            },
                        },
                    },
                ),
            )
        },
    })
}

/** @returns whether the project has a Courier provider (which unlocks domains). */
export async function fetchHasCourierProvider(queryClient: QueryClient, projectId: UUID) {
    const providers = await queryClient.fetchQuery(providersQuery(projectId))
    return providers.some((p) => p.module === "courier")
}

export const actionKeys = {
    all: (projectId: UUID) => ["projects", projectId, "actions"] as const,
    list: (projectId: UUID) => [...actionKeys.all(projectId), "list"] as const,
    meta: (projectId: UUID) => [...actionKeys.all(projectId), "meta"] as const,
}

/** Every action of the project. */
export function actionsQuery(projectId: UUID) {
    return queryOptions({
        queryKey: actionKeys.list(projectId),
        queryFn: () =>
            fetchAllPages(async (offset) =>
                unwrap(
                    await oapiClient.GET("/api/admin/projects/{projectID}/actions", {
                        params: {
                            path: { projectID: projectId },
                            query: { limit: API_PAGE_LIMIT, offset },
                        },
                    }),
                ),
            ),
    })
}

/** The action types the project can configure. */
export function actionsMetaQuery(projectId: UUID) {
    return queryOptions({
        queryKey: actionKeys.meta(projectId),
        queryFn: async () =>
            unwrap(
                await oapiClient.GET("/api/admin/projects/{projectID}/actions/meta", {
                    params: { path: { projectID: projectId } },
                }),
            ) ?? [],
    })
}

/**
 * The project's subscriptions. Same key and data as the campaign editor's
 * query, so both read one cache entry.
 */
export function subscriptionsQuery(projectId: UUID) {
    return queryOptions({
        queryKey: ["projects", projectId, "subscriptions"],
        queryFn: async () =>
            (await api.subscriptions.search(projectId, { limit: API_PAGE_LIMIT })).results ?? [],
    })
}
