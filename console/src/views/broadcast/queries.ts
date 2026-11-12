import { keepPreviousData, queryOptions, useMutation, useQueryClient } from "@tanstack/react-query"
import { useContext } from "react"
import { t } from "i18next"
import api from "@/api"
import { ProjectContext } from "@/contexts"
import { oapiClient } from "@/oapi/client"
import { unwrap } from "@/oapi/unwrap"
import type { Broadcast, BroadcastUser } from "@/types"
import type { UUID } from "@/types/common"
import { broadcastResponseSchema } from "@/validation/broadcast/broadcast-response"
import { isSentState, type ListUser, type RecipientRow } from "./broadcast-state"

export const broadcastKeys = {
    all: (projectId: UUID) => ["projects", projectId, "broadcasts"] as const,
    list: (projectId: UUID) => [...broadcastKeys.all(projectId), "list"] as const,
    detail: (projectId: UUID, broadcastId: UUID) =>
        [...broadcastKeys.all(projectId), broadcastId] as const,
    recipients: (projectId: UUID, broadcastId: UUID) =>
        [...broadcastKeys.detail(projectId, broadcastId), "recipients"] as const,
}

export const BROADCAST_PAGE_SIZE = 15

interface PageParams {
    offset: number
    search: string
}

/** One page of the broadcasts list. */
export function broadcastListQuery(projectId: UUID, { offset, search }: PageParams) {
    return queryOptions({
        queryKey: [...broadcastKeys.list(projectId), { offset, search }] as const,
        queryFn: async () => {
            const data = unwrap(
                await oapiClient.GET("/api/admin/projects/{projectID}/broadcasts", {
                    params: {
                        path: { projectID: projectId },
                        query: {
                            limit: BROADCAST_PAGE_SIZE,
                            offset,
                            search: search || undefined,
                        },
                    },
                }),
            )
            return { results: (data.results ?? []) as Broadcast[], total: data.total ?? 0 }
        },
        placeholderData: keepPreviousData,
        // Sending and scheduled broadcasts change on their own, so keep them current.
        refetchInterval: (query) =>
            query.state.data?.results.some((b) => b.state === "sending" || b.state === "scheduled")
                ? 5000
                : false,
    })
}

export function broadcastQuery(projectId: UUID, broadcastId: UUID) {
    return queryOptions({
        queryKey: broadcastKeys.detail(projectId, broadcastId),
        queryFn: async () =>
            unwrap(
                await oapiClient.GET("/api/admin/projects/{projectID}/broadcasts/{broadcastID}", {
                    params: { path: { projectID: projectId, broadcastID: broadcastId } },
                }),
            ) as Broadcast,
    })
}

export const RECIPIENTS_PAGE_SIZE = 25

/**
 * One page of a broadcast's recipients: the users it was sent to once it is
 * sent, and until then a preview of its list's members.
 */
export function recipientsQuery(
    projectId: UUID,
    broadcast: Pick<Broadcast, "id" | "state" | "list_id"> | undefined,
    { offset, search }: PageParams,
) {
    const sent = broadcast ? isSentState(broadcast.state) : false
    const listId = broadcast?.list_id
    const query = { limit: RECIPIENTS_PAGE_SIZE, offset, search: search || undefined }
    return queryOptions({
        queryKey: [
            ...broadcastKeys.recipients(projectId, broadcast?.id ?? ("" as UUID)),
            { sent, listId, offset, search },
        ] as const,
        queryFn: async (): Promise<{ results: RecipientRow[]; total: number }> => {
            if (sent) {
                const data = unwrap(
                    await oapiClient.GET(
                        "/api/admin/projects/{projectID}/broadcasts/{broadcastID}/users",
                        {
                            params: {
                                path: { projectID: projectId, broadcastID: broadcast!.id },
                                query,
                            },
                        },
                    ),
                )
                const results = (data.results ?? []) as BroadcastUser[]
                return { results, total: data.total ?? results.length }
            }
            const data = unwrap(
                await oapiClient.GET("/api/admin/projects/{projectID}/lists/{listID}/users", {
                    params: { path: { projectID: projectId, listID: listId! }, query },
                }),
            )
            const results = (data.results ?? []) as ListUser[]
            return { results, total: data.total ?? results.length }
        },
        enabled: !!broadcast && (sent || !!listId),
        // Keep the current page on screen while the next one loads.
        placeholderData: keepPreviousData,
    })
}

// The campaign and list pickers of the create dialog keep the old cap of 100
// entries; they refetch every time the dialog opens so new entries show up.
const PICKER_LIMIT = 100

export function campaignPickerQuery(projectId: UUID, enabled: boolean) {
    return queryOptions({
        queryKey: ["projects", projectId, "campaigns", "list", { limit: PICKER_LIMIT }] as const,
        queryFn: async () =>
            (await api.campaigns.search(projectId, { limit: PICKER_LIMIT })).results ?? [],
        enabled,
        staleTime: 0,
    })
}

export function listPickerQuery(projectId: UUID, enabled: boolean) {
    return queryOptions({
        queryKey: ["projects", projectId, "lists", "list", { limit: PICKER_LIMIT }] as const,
        queryFn: async () =>
            (await api.lists.search(projectId, { limit: PICKER_LIMIT })).results ?? [],
        enabled,
        staleTime: 0,
    })
}

interface CreateBroadcastParams {
    campaign_id: string
    list_id: string
    scheduled_at?: string
}

export function useCreateBroadcast() {
    const [project] = useContext(ProjectContext)
    const queryClient = useQueryClient()
    return useMutation({
        mutationFn: async (body: CreateBroadcastParams) => {
            const data = unwrap(
                await oapiClient.POST("/api/admin/projects/{projectID}/broadcasts", {
                    params: { path: { projectID: project.id } },
                    body,
                }),
            )
            return broadcastResponseSchema.parse(data) as Broadcast
        },
        // Not awaited: the caller usually navigates to the new broadcast right away.
        onSuccess: () => {
            void queryClient.invalidateQueries({ queryKey: broadcastKeys.all(project.id) })
        },
        meta: { errorMessage: t("broadcast_create_error", "Failed to create broadcast") },
    })
}

// useBroadcastMutation runs an action on one broadcast, then refetches the
// project's broadcasts (the detail, its recipients and every list) so the
// page shows what the server actually did.
function useBroadcastMutation<V>(
    broadcastId: UUID,
    request: (path: { projectID: UUID; broadcastID: UUID }, variables: V) => Promise<unknown>,
    errorMessage: string,
) {
    const [project] = useContext(ProjectContext)
    const queryClient = useQueryClient()
    return useMutation({
        mutationFn: (variables: V) =>
            request({ projectID: project.id, broadcastID: broadcastId }, variables),
        onSuccess: () => queryClient.invalidateQueries({ queryKey: broadcastKeys.all(project.id) }),
        meta: { errorMessage },
    })
}

export function useSendBroadcast(broadcastId: UUID) {
    return useBroadcastMutation(
        broadcastId,
        async (path) =>
            unwrap(
                await oapiClient.POST(
                    "/api/admin/projects/{projectID}/broadcasts/{broadcastID}/send",
                    { params: { path } },
                ),
            ),
        t("broadcast_send_error", "Failed to send broadcast"),
    )
}

export function useCancelBroadcast(broadcastId: UUID) {
    return useBroadcastMutation(
        broadcastId,
        async (path) =>
            unwrap(
                await oapiClient.DELETE(
                    "/api/admin/projects/{projectID}/broadcasts/{broadcastID}",
                    {
                        params: { path },
                    },
                ),
            ),
        t("broadcast_cancel_error", "Failed to cancel broadcast"),
    )
}

/** Sets (an ISO time) or removes (null) the time a broadcast is sent at. */
export function useScheduleBroadcast(broadcastId: UUID, errorMessage: string) {
    return useBroadcastMutation(
        broadcastId,
        async (path, scheduledAt: string | null) =>
            unwrap(
                await oapiClient.PATCH("/api/admin/projects/{projectID}/broadcasts/{broadcastID}", {
                    params: { path },
                    body: { scheduled_at: scheduledAt },
                }),
            ),
        errorMessage,
    )
}
