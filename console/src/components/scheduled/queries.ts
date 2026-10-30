import { keepPreviousData, queryOptions, useMutation, useQueryClient } from "@tanstack/react-query"
import { useContext } from "react"
import { t } from "i18next"
import { client } from "@/api"
import { ProjectContext } from "@/contexts"
import { oapiClient } from "@/oapi/client"
import { unwrap } from "@/oapi/unwrap"
import type { ScheduledSchema } from "@/types"

export type ScheduledSubjectType = "users" | "organizations"

/**
 * Common interface for a scheduled instance row.
 * Both user and organization scheduled instances share this shape.
 */
export interface ScheduledItem {
    id: string
    scheduled_id: string
    scheduled_at: string
    start_at: string | null
    anchor_at: string | null
    interval: string | null
    paused_at: string | null
    has_pending_events: boolean | null
    data: Record<string, unknown> | null
    created_at: string
    updated_at: string
}

interface ScheduledPage {
    results: ScheduledItem[]
    total: number
    limit: number
    offset: number
}

export const SCHEDULED_PAGE_SIZE = 15

export const scheduledKeys = {
    schemas: (projectId: string) => ["projects", projectId, "scheduled-schemas"] as const,
    subject: (projectId: string, subjectType: ScheduledSubjectType, subjectId: string) =>
        ["projects", projectId, subjectType, subjectId, "scheduled"] as const,
}

/** The project's schedule schemas, which give scheduled items their names. */
export function scheduledSchemasQuery(projectId: string) {
    return queryOptions({
        queryKey: scheduledKeys.schemas(projectId),
        queryFn: async () => {
            const { data } = await client.get<{ results: ScheduledSchema[] }>(
                `/admin/projects/${projectId}/subjects/user/scheduled/schema`,
            )
            return data.results ?? []
        },
    })
}

/** One page of a subject's scheduled items. */
export function scheduledListQuery(
    projectId: string,
    subjectType: ScheduledSubjectType,
    subjectId: string,
    { page, search }: { page: number; search: string },
) {
    return queryOptions({
        queryKey: [...scheduledKeys.subject(projectId, subjectType, subjectId), { page, search }],
        queryFn: async () => {
            const { data } = await client.get<ScheduledPage>(
                `/admin/projects/${projectId}/subjects/${subjectType}/${subjectId}/scheduled`,
                {
                    params: {
                        limit: SCHEDULED_PAGE_SIZE,
                        offset: (page - 1) * SCHEDULED_PAGE_SIZE,
                        search: search || undefined,
                    },
                },
            )
            return data
        },
        // Keep the current page on screen while the next one loads.
        placeholderData: keepPreviousData,
    })
}

export interface ScheduledCreateBody {
    scheduled_name: string
    scheduled_at?: string
    start_at?: string
    interval?: string
    data?: Record<string, unknown>
}

// useSubjectScheduled gives the mutations below the subject's URLs and a way
// to refresh its cached lists.
function useSubjectScheduled(subjectType: ScheduledSubjectType, subjectId: string) {
    const [project] = useContext(ProjectContext)
    const queryClient = useQueryClient()
    return {
        projectId: project.id,
        itemUrl: (itemId: string) =>
            `/admin/projects/${project.id}/subjects/${subjectType}/${subjectId}/scheduled/${itemId}`,
        invalidate: () =>
            queryClient.invalidateQueries({
                queryKey: scheduledKeys.subject(project.id, subjectType, subjectId),
            }),
        invalidateSchemas: () =>
            queryClient.invalidateQueries({ queryKey: scheduledKeys.schemas(project.id) }),
    }
}

export function useCreateScheduled(subjectType: ScheduledSubjectType, subjectId: string) {
    const { projectId, invalidate, invalidateSchemas } = useSubjectScheduled(subjectType, subjectId)
    return useMutation({
        mutationFn: async (body: ScheduledCreateBody) =>
            subjectType === "users"
                ? unwrap(
                      await oapiClient.PUT(
                          "/api/admin/projects/{projectID}/subjects/users/{userID}/scheduled",
                          { params: { path: { projectID: projectId, userID: subjectId } }, body },
                      ),
                  )
                : unwrap(
                      await oapiClient.PUT(
                          "/api/admin/projects/{projectID}/subjects/organizations/{organizationID}/scheduled",
                          {
                              params: {
                                  path: { projectID: projectId, organizationID: subjectId },
                              },
                              body,
                          },
                      ),
                  ),
        // A new name registers a new schedule schema, so refresh the id-to-name
        // map too; otherwise the new row shows its raw scheduled_id.
        onSuccess: () => Promise.all([invalidate(), invalidateSchemas()]),
        meta: {
            errorMessage: t("failed_to_create_scheduled", "Failed to create scheduled item"),
        },
    })
}

export type ScheduledPatch =
    | { scheduled_at: string }
    | { pause: "immediately" | "after_next_interval" }
    | { resume: "immediately" | "at_next_interval" }

export function useUpdateScheduled(
    subjectType: ScheduledSubjectType,
    subjectId: string,
    errorMessage: string,
) {
    const { itemUrl, invalidate } = useSubjectScheduled(subjectType, subjectId)
    return useMutation({
        mutationFn: ({ itemId, patch }: { itemId: string; patch: ScheduledPatch }) =>
            client.patch(itemUrl(itemId), patch),
        onSuccess: invalidate,
        meta: { errorMessage },
    })
}

export function useDeleteScheduled(subjectType: ScheduledSubjectType, subjectId: string) {
    const { itemUrl, invalidate } = useSubjectScheduled(subjectType, subjectId)
    return useMutation({
        mutationFn: (itemId: string) => client.delete(itemUrl(itemId)),
        onSuccess: invalidate,
        meta: {
            errorMessage: t("failed_to_delete_scheduled", "Failed to delete scheduled item"),
        },
    })
}
