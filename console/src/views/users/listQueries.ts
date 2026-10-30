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
import { ListContext, ProjectContext } from "@/contexts"
import { oapiClient } from "@/oapi/client"
import { unwrap } from "@/oapi/unwrap"
import type { List, ListCreateParams, ListUpdateParams } from "@/types"
import type { UUID } from "@/types/common"

export const listKeys = {
    all: (projectId: UUID) => ["projects", projectId, "lists"] as const,
    pages: (projectId: UUID) => [...listKeys.all(projectId), "list"] as const,
    detail: (projectId: UUID, listId: UUID) => [...listKeys.all(projectId), listId] as const,
    users: (projectId: UUID, listId: UUID) =>
        [...listKeys.detail(projectId, listId), "users"] as const,
}

/** A list; the route loader fills it and ListContext reads it. */
export function listQuery(projectId: UUID, listId: UUID) {
    return queryOptions({
        queryKey: listKeys.detail(projectId, listId),
        queryFn: () => api.lists.get(projectId, listId),
    })
}

export const LIST_PAGE_SIZE = 15

interface ListPageParams {
    offset: number
    search: string
    /** The archived view also lists archived lists. */
    archived: boolean
}

/** One page of the lists overview. */
export function listPageQuery(projectId: UUID, { offset, search, archived }: ListPageParams) {
    return queryOptions({
        queryKey: [...listKeys.pages(projectId), { offset, search, archived }] as const,
        queryFn: async () =>
            unwrap(
                await oapiClient.GET("/api/admin/projects/{projectID}/lists", {
                    params: {
                        path: { projectID: projectId },
                        query: {
                            limit: LIST_PAGE_SIZE,
                            offset,
                            search: search || undefined,
                            ...(archived ? { include_deleted: true } : {}),
                        },
                    },
                }),
            ),
        // Keep the current page on screen while the next one loads.
        placeholderData: keepPreviousData,
    })
}

/**
 * Subset of user fields shown in a list's users table. A type alias rather than
 * an interface, so it is assignable to the name helpers' Record parameter.
 */
export type ListUser = {
    id: string
    full_name?: string
    identifier?: Array<{
        source: string
        external_id: string
        metadata?: Record<string, unknown> | null
    }>
    email?: string
    phone?: string
}

interface ListUsersParams {
    offset: number
    search: string
    /** A dynamic list with an unpublished rule shows who the draft would match. */
    preview: boolean
}

export const LIST_USERS_PAGE_SIZE = 25

export function listUsersQuery(
    projectId: UUID,
    listId: UUID,
    { offset, search, preview }: ListUsersParams,
) {
    return queryOptions({
        queryKey: [...listKeys.users(projectId, listId), { offset, search, preview }] as const,
        queryFn: async (): Promise<{ results: ListUser[]; total: number | null }> => {
            if (preview) {
                const data = unwrap(
                    await oapiClient.GET(
                        "/api/admin/projects/{projectID}/lists/{listID}/users/preview",
                        {
                            params: {
                                path: { projectID: projectId, listID: listId },
                                query: { limit: LIST_USERS_PAGE_SIZE },
                            },
                        },
                    ),
                )
                const results = (data.results ?? []) as ListUser[]
                return { results, total: data.total ?? results.length }
            }
            const result = await api.lists.users(projectId, listId, {
                limit: LIST_USERS_PAGE_SIZE,
                offset,
                search: search || undefined,
            })
            return { results: result.results, total: result.total ?? null }
        },
        placeholderData: keepPreviousData,
    })
}

/** Marks every cached page of the lists overview as outdated. */
export function invalidateListPages(queryClient: QueryClient, projectId: UUID) {
    return queryClient.invalidateQueries({ queryKey: listKeys.pages(projectId) })
}

// useListsMutation runs an action on a list of the project, then refreshes
// every cached list (and lists page) so the change shows everywhere.
function useListsMutation<V, T>(
    request: (projectId: UUID, variables: V) => Promise<T>,
    errorMessage: string,
) {
    const [project] = useContext(ProjectContext)
    const queryClient = useQueryClient()
    return useMutation({
        mutationFn: (variables: V) => request(project.id, variables),
        onSuccess: () => queryClient.invalidateQueries({ queryKey: listKeys.all(project.id) }),
        meta: { errorMessage },
    })
}

export function useCreateList() {
    return useListsMutation(
        (projectId, params: ListCreateParams) => api.lists.create(projectId, params),
        t("list_create_failed", "Couldn't create the list"),
    )
}

export function useDuplicateList() {
    return useListsMutation(
        (projectId, listId: UUID) => api.lists.duplicate(projectId, listId),
        t("list_duplicate_failed", "Couldn't duplicate the list"),
    )
}

export function useArchiveList() {
    return useListsMutation(
        (projectId, listId: UUID) => api.lists.delete(projectId, listId),
        t("list_archive_failed", "Couldn't archive the list"),
    )
}

export function useUnarchiveList() {
    return useListsMutation(
        async (projectId, listId: UUID) =>
            unwrap(
                await oapiClient.POST("/api/admin/projects/{projectID}/lists/{listID}/unarchive", {
                    params: { path: { projectID: projectId, listID: listId } },
                }),
            ),
        t("list_unarchive_failed", "Couldn't unarchive the list"),
    )
}

// useListCache addresses the cached copy of the list in context.
function useListCache() {
    const [project] = useContext(ProjectContext)
    const [list] = useContext(ListContext)
    const queryClient = useQueryClient()
    return { queryClient, projectId: project.id, listId: list.id }
}

export function useUpdateList() {
    const { queryClient, projectId, listId } = useListCache()
    return useMutation({
        mutationFn: (params: ListUpdateParams) => api.lists.update(projectId, listId, params),
        onSuccess: (list: List) => {
            queryClient.setQueryData(listKeys.detail(projectId, listId), list)
            void invalidateListPages(queryClient, projectId)
            void queryClient.invalidateQueries({ queryKey: listKeys.users(projectId, listId) })
        },
        meta: { errorMessage: t("list_save_failed", "Couldn't save the list") },
    })
}

export function useUploadListUsers() {
    const { queryClient, projectId, listId } = useListCache()
    return useMutation({
        mutationFn: (file: File) => api.lists.upload(projectId, listId, file),
        // The upload starts generating the list; refetch it to show the progress.
        onSuccess: () =>
            queryClient.invalidateQueries({ queryKey: listKeys.detail(projectId, listId) }),
        meta: { errorMessage: t("list_upload_failed", "Couldn't upload the users") },
    })
}
