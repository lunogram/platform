import {
    infiniteQueryOptions,
    keepPreviousData,
    queryOptions,
    type QueryClient,
} from "@tanstack/react-query"
import { NIL } from "uuid"
import api from "@/api"
import { fetchPathSuggestions } from "@/lib/path-suggestions"
import { oapiClient } from "@/oapi/client"
import { unwrap } from "@/oapi/unwrap"
import type { User } from "@/types"
import type { UUID } from "@/types/common"

export const journeyKeys = {
    all: (projectId: UUID) => ["projects", projectId, "journeys"] as const,
    lists: (projectId: UUID) => [...journeyKeys.all(projectId), "list"] as const,
    detail: (projectId: UUID, journeyId: UUID) =>
        [...journeyKeys.all(projectId), journeyId] as const,
    steps: (projectId: UUID, journeyId: UUID) =>
        [...journeyKeys.detail(projectId, journeyId), "steps"] as const,
}

/** The journey the editor shows; the route loader puts it in the cache. */
export function journeyQuery(projectId: UUID, journeyId: UUID) {
    return queryOptions({
        queryKey: journeyKeys.detail(projectId, journeyId),
        queryFn: () => api.journeys.get(projectId, journeyId),
    })
}

/** The saved steps of a journey, keyed by journey so two journeys never share a canvas. */
export function journeyStepsQuery(projectId: UUID, journeyId: UUID) {
    return queryOptions({
        queryKey: journeyKeys.steps(projectId, journeyId),
        queryFn: () => api.journeys.steps.get(projectId, journeyId),
    })
}

/** Marks every cached journeys list of the project as outdated. */
export function invalidateJourneyLists(queryClient: QueryClient, projectId: UUID) {
    return queryClient.invalidateQueries({ queryKey: journeyKeys.lists(projectId) })
}

export const JOURNEY_PAGE_SIZE = 15

interface JourneyListParams {
    search: string
    cursor?: string
    page?: "next" | "prev"
}

/** One cursor page of the active journeys. */
export function journeyListQuery(projectId: UUID, { search, cursor, page }: JourneyListParams) {
    return queryOptions({
        queryKey: [...journeyKeys.lists(projectId), { search, cursor, page }] as const,
        queryFn: () =>
            api.journeys.search(projectId, {
                limit: JOURNEY_PAGE_SIZE,
                cursor,
                page,
                search: search || undefined,
            }),
        // Keep the current page on screen while the next one loads.
        placeholderData: keepPreviousData,
    })
}

/** One offset page of the journeys list including archived journeys. */
export function archivedJourneyListQuery(
    projectId: UUID,
    { search, offset }: { search: string; offset: number },
) {
    return queryOptions({
        queryKey: [...journeyKeys.lists(projectId), { search, offset, archived: true }] as const,
        queryFn: async () =>
            unwrap(
                await oapiClient.GET("/api/admin/projects/{projectID}/journeys", {
                    params: {
                        path: { projectID: projectId },
                        query: {
                            limit: JOURNEY_PAGE_SIZE,
                            offset,
                            include_deleted: true,
                            search: search || undefined,
                        },
                    },
                }),
            ),
        placeholderData: keepPreviousData,
    })
}

/** The project's actions, for the editor's library sidebar. */
export function actionListQuery(projectId: UUID) {
    return queryOptions({
        queryKey: ["projects", projectId, "actions", "list", { limit: 100 }],
        queryFn: async () =>
            unwrap(
                await oapiClient.GET("/api/admin/projects/{projectID}/actions", {
                    params: { path: { projectID: projectId }, query: { limit: 100 } },
                }),
            ).results ?? [],
    })
}

/** One action; shared by every Action step that uses it and by the step editor. */
export function actionQuery(projectId: UUID, actionId: UUID) {
    return queryOptions({
        queryKey: ["projects", projectId, "actions", actionId],
        queryFn: async () =>
            unwrap(
                await oapiClient.GET("/api/admin/projects/{projectID}/actions/{actionID}", {
                    params: { path: { projectID: projectId, actionID: actionId } },
                }),
            ),
        enabled: !!actionId && actionId !== NIL,
    })
}

/** The functions every action type offers. */
export function actionMetaQuery(projectId: UUID) {
    return queryOptions({
        queryKey: ["projects", projectId, "actions", "meta"],
        queryFn: async () =>
            unwrap(
                await oapiClient.GET("/api/admin/projects/{projectID}/actions/meta", {
                    params: { path: { projectID: projectId } },
                }),
            ),
    })
}

/**
 * A campaign sent by a journey step. The key is the one the campaign editor
 * uses, so the steps and the editor share one cached copy.
 */
export function stepCampaignQuery(projectId: UUID, campaignId: UUID) {
    return queryOptions({
        queryKey: ["projects", projectId, "campaigns", campaignId],
        queryFn: () => api.campaigns.get(projectId, campaignId),
        enabled: !!campaignId && campaignId !== NIL,
    })
}

/** The project's event, user and schedule schemas, used by variable and event pickers. */
export function pathSuggestionsQuery(projectId: UUID) {
    return queryOptions({
        queryKey: ["projects", projectId, "path-suggestions"],
        queryFn: () => fetchPathSuggestions(projectId),
    })
}

/** Lists matching a search, for the list entrance's picker. */
export function listSearchQuery(projectId: UUID, search: string) {
    return queryOptions({
        queryKey: ["projects", projectId, "lists", "list", { search, limit: 50 }] as const,
        queryFn: async () => (await api.lists.search(projectId, { limit: 50, search })).results,
    })
}

export const USER_PAGE_SIZE = 25

/** Users matching a search, a page at a time, for picking a user to run a journey for. */
export function userPagesQuery(projectId: UUID, search: string) {
    return infiniteQueryOptions({
        queryKey: ["projects", projectId, "users", "list", { search }] as const,
        queryFn: async ({ pageParam }) => {
            const data = unwrap(
                await oapiClient.GET("/api/admin/projects/{projectID}/subjects/users", {
                    params: {
                        path: { projectID: projectId },
                        query: {
                            limit: USER_PAGE_SIZE,
                            offset: pageParam,
                            ...(search ? { search } : {}),
                        },
                    },
                }),
            )
            return { results: (data.results ?? []) as unknown as User[], total: data.total ?? 0 }
        },
        initialPageParam: 0,
        getNextPageParam: (last, pages) => {
            const loaded = pages.reduce((sum, page) => sum + page.results.length, 0)
            return loaded < last.total && last.results.length > 0 ? loaded : undefined
        },
    })
}
