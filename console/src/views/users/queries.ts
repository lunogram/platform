import {
    keepPreviousData,
    queryOptions,
    useMutation,
    useQueryClient,
    type QueryClient,
} from "@tanstack/react-query"
import { useContext } from "react"
import { t } from "i18next"
import api from "@/api"
import { ProjectContext, UserContext } from "@/contexts"
import { oapiClient } from "@/oapi/client"
import { unwrap } from "@/oapi/unwrap"
import type { SubscriptionParams, User } from "@/types"
import type { UUID } from "@/types/common"

export const userKeys = {
    all: (projectId: UUID) => ["projects", projectId, "users"] as const,
    lists: (projectId: UUID) => [...userKeys.all(projectId), "list"] as const,
    detail: (projectId: UUID, userId: UUID) => [...userKeys.all(projectId), userId] as const,
    events: (projectId: UUID, userId: UUID) =>
        [...userKeys.detail(projectId, userId), "events"] as const,
    subscriptions: (projectId: UUID, userId: UUID) =>
        [...userKeys.detail(projectId, userId), "subscriptions"] as const,
}

/** A user; the route loader fills it and UserContext reads it. */
export function userQuery(projectId: UUID, userId: UUID) {
    return queryOptions({
        queryKey: userKeys.detail(projectId, userId),
        queryFn: () => api.users.get(projectId, userId),
    })
}

interface UserSearchParams {
    limit: number
    offset: number
    search: string
}

/** One page of the project's users. */
export function userSearchQuery(projectId: UUID, { limit, offset, search }: UserSearchParams) {
    return queryOptions({
        queryKey: [...userKeys.lists(projectId), { limit, offset, search }] as const,
        queryFn: () => api.users.search(projectId, { limit, offset, search: search || undefined }),
        // Keep the current page on screen while the next one loads.
        placeholderData: keepPreviousData,
    })
}

interface UserPageParams {
    page: number
    limit: number
    search?: string
}

export function userEventsQuery(
    projectId: UUID,
    userId: UUID,
    { page, limit, search = "" }: UserPageParams,
) {
    return queryOptions({
        queryKey: [...userKeys.events(projectId, userId), { page, limit, search }] as const,
        queryFn: () =>
            api.users.events(projectId, userId, {
                limit,
                offset: (page - 1) * limit,
                search: search || undefined,
            }),
        placeholderData: keepPreviousData,
    })
}

/** The names (and data paths) of the events users have sent. */
export function userEventSchemasQuery(projectId: UUID) {
    return queryOptions({
        queryKey: ["projects", projectId, "user-event-schemas"] as const,
        queryFn: async () =>
            unwrap(
                await oapiClient.GET(
                    "/api/admin/projects/{projectID}/subjects/user/events/schema",
                    {
                        params: { path: { projectID: projectId } },
                    },
                ),
            ).results ?? [],
    })
}

export function userJourneysQuery(projectId: UUID, userId: UUID, { page, limit }: UserPageParams) {
    return queryOptions({
        queryKey: [...userKeys.detail(projectId, userId), "journeys", { page, limit }] as const,
        queryFn: async () =>
            unwrap(
                await oapiClient.GET(
                    "/api/admin/projects/{projectID}/subjects/users/{userID}/journeys",
                    {
                        params: {
                            path: { projectID: projectId, userID: userId },
                            query: { limit, offset: (page - 1) * limit },
                        },
                    },
                ),
            ),
        placeholderData: keepPreviousData,
    })
}

export function userOrganizationsQuery(
    projectId: UUID,
    userId: UUID,
    { page, limit, search = "" }: UserPageParams,
) {
    return queryOptions({
        queryKey: [
            ...userKeys.detail(projectId, userId),
            "organizations",
            { page, limit, search },
        ] as const,
        queryFn: async () =>
            unwrap(
                await oapiClient.GET(
                    "/api/admin/projects/{projectID}/subjects/users/{userID}/subject-organizations",
                    {
                        params: {
                            path: { projectID: projectId, userID: userId },
                            query: {
                                limit,
                                offset: (page - 1) * limit,
                                search: search || undefined,
                            },
                        },
                    },
                ),
            ),
        placeholderData: keepPreviousData,
    })
}

// The API returns at most this many subscriptions per request.
const SUBSCRIPTION_PAGE_SIZE = 100

function fetchUserSubscriptions(projectId: UUID, userId: UUID, offset: number) {
    return oapiClient
        .GET("/api/admin/projects/{projectID}/subjects/users/{userID}/subscriptions", {
            params: {
                path: { projectID: projectId, userID: userId },
                query: { limit: SUBSCRIPTION_PAGE_SIZE, offset },
            },
        })
        .then(unwrap)
}

export function userSubscriptionsQuery(projectId: UUID, userId: UUID) {
    return queryOptions({
        queryKey: userKeys.subscriptions(projectId, userId),
        queryFn: () => fetchUserSubscriptions(projectId, userId, 0),
    })
}

/** Marks every cached users list of the project as outdated. */
export function invalidateUserLists(queryClient: QueryClient, projectId: UUID) {
    return queryClient.invalidateQueries({ queryKey: userKeys.lists(projectId) })
}

export function useCreateUser() {
    const [project] = useContext(ProjectContext)
    const queryClient = useQueryClient()
    return useMutation({
        mutationFn: (user: User) => api.users.create(project.id, user),
        onSuccess: () => invalidateUserLists(queryClient, project.id),
        meta: { errorMessage: t("user_create_failed", "Couldn't create the user") },
    })
}

export function useImportUsers() {
    const [project] = useContext(ProjectContext)
    const queryClient = useQueryClient()
    return useMutation({
        mutationFn: (file: File) => api.users.addImport(project.id, file),
        onSuccess: () => invalidateUserLists(queryClient, project.id),
        meta: { errorMessage: t("user_import_failed", "Couldn't import the users") },
    })
}

// useUserCache addresses the cached copy of the user in context.
function useUserCache() {
    const [project] = useContext(ProjectContext)
    const [user] = useContext(UserContext)
    const queryClient = useQueryClient()
    return { queryClient, projectId: project.id, userId: user.id }
}

/**
 * Saves fields of the user in context. Each caller names its own failure
 * message, since one page edits several unrelated fields.
 */
export function useUpdateUser(errorMessage: string) {
    const { queryClient, projectId, userId } = useUserCache()
    return useMutation({
        mutationFn: (params: Partial<User>) => api.users.update(projectId, userId, params as User),
        onSuccess: (user) => {
            queryClient.setQueryData(userKeys.detail(projectId, userId), user)
            void invalidateUserLists(queryClient, projectId)
        },
        meta: { errorMessage },
    })
}

export function useDeleteUser() {
    const { queryClient, projectId, userId } = useUserCache()
    return useMutation({
        mutationFn: () => api.users.delete(projectId, userId),
        onSuccess: () => {
            queryClient.removeQueries({ queryKey: userKeys.detail(projectId, userId) })
            void invalidateUserLists(queryClient, projectId)
        },
        meta: { errorMessage: t("user_delete_failed", "Couldn't delete the user") },
    })
}

export function useCreateUserEvent() {
    const { queryClient, projectId, userId } = useUserCache()
    return useMutation({
        mutationFn: async (body: { name: string; data?: Record<string, unknown> }) =>
            unwrap(
                await oapiClient.POST(
                    "/api/admin/projects/{projectID}/subjects/users/{userID}/events",
                    { params: { path: { projectID: projectId, userID: userId } }, body },
                ),
            ),
        onSuccess: () => {
            // Events are processed asynchronously; give the new one time to show up.
            setTimeout(() => {
                void queryClient.invalidateQueries({ queryKey: userKeys.events(projectId, userId) })
            }, 1000)
        },
        meta: { errorMessage: t("failed_to_create_event", "Failed to create event") },
    })
}

async function patchUserSubscriptions(
    projectId: UUID,
    userId: UUID,
    subscriptions: SubscriptionParams[],
) {
    return unwrap(
        await oapiClient.PATCH(
            "/api/admin/projects/{projectID}/subjects/users/{userID}/subscriptions",
            {
                params: { path: { projectID: projectId, userID: userId } },
                body: subscriptions,
            },
        ),
    )
}

export function useUpdateUserSubscription() {
    const { queryClient, projectId, userId } = useUserCache()
    return useMutation({
        mutationFn: (subscription: SubscriptionParams) =>
            patchUserSubscriptions(projectId, userId, [subscription]),
        onSettled: () =>
            queryClient.invalidateQueries({
                queryKey: userKeys.subscriptions(projectId, userId),
            }),
        meta: {
            errorMessage: t("subscription_update_failed", "Couldn't change the subscription"),
        },
    })
}

/** Unsubscribes the user from every subscription, not only the ones on screen. */
export function useUnsubscribeAll() {
    const { queryClient, projectId, userId } = useUserCache()
    return useMutation({
        mutationFn: async () => {
            const params: SubscriptionParams[] = []
            for (let offset = 0; ; offset += SUBSCRIPTION_PAGE_SIZE) {
                const page = await fetchUserSubscriptions(projectId, userId, offset)
                params.push(
                    ...page.results.map((item) => ({
                        subscription_id: item.subscription_id,
                        state: "unsubscribed" as const,
                    })),
                )
                if (page.results.length === 0 || params.length >= page.total) break
            }
            // The API rejects an empty update.
            if (params.length === 0) return
            await patchUserSubscriptions(projectId, userId, params)
        },
        onSettled: () =>
            queryClient.invalidateQueries({
                queryKey: userKeys.subscriptions(projectId, userId),
            }),
        meta: { errorMessage: t("unsubscribe_all_failed", "Couldn't unsubscribe the user") },
    })
}
