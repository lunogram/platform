import { keepPreviousData, queryOptions, useMutation, useQueryClient } from "@tanstack/react-query"
import { useContext } from "react"
import { t } from "i18next"
import { ProjectContext } from "@/contexts"
import { oapiClient } from "@/oapi/client"
import { unwrap } from "@/oapi/unwrap"
import type { components } from "@/oapi/management.generated"

export type InboxMessage = components["schemas"]["InboxMessage"]
export type InboxStatus = "all" | "unread" | "read" | "archived" | "failed"
export type InboxChannel = components["schemas"]["Channel"]
export type InboxSubjectType = "users" | "organizations"
export type InboxEvent = "opened" | "archived" | "scheduled" | "unarchived" | "unread"

export const INBOX_PAGE_SIZE = 15

export const inboxKeys = {
    subject: (projectId: string, subjectType: InboxSubjectType, subjectId: string) =>
        ["projects", projectId, subjectType, subjectId, "inbox"] as const,
}

interface InboxListParams {
    page: number
    search: string
    status: InboxStatus
}

/** One page of a subject's inbox. */
export function inboxListQuery(
    projectId: string,
    subjectType: InboxSubjectType,
    subjectId: string,
    { page, search, status }: InboxListParams,
) {
    return queryOptions({
        queryKey: [
            ...inboxKeys.subject(projectId, subjectType, subjectId),
            { page, search, status },
        ],
        queryFn: async () => {
            const query = {
                limit: INBOX_PAGE_SIZE,
                offset: (page - 1) * INBOX_PAGE_SIZE,
                search: search || undefined,
                status: status === "all" ? undefined : status,
                include_archived: status === "all" ? true : undefined,
                include_scheduled: true,
            }
            const data =
                subjectType === "users"
                    ? unwrap(
                          await oapiClient.GET(
                              "/api/admin/projects/{projectID}/subjects/users/{userID}/inbox",
                              {
                                  params: {
                                      path: { projectID: projectId, userID: subjectId },
                                      query,
                                  },
                              },
                          ),
                      )
                    : unwrap(
                          await oapiClient.GET(
                              "/api/admin/projects/{projectID}/subjects/organizations/{organizationID}/inbox",
                              {
                                  params: {
                                      path: { projectID: projectId, organizationID: subjectId },
                                      query,
                                  },
                              },
                          ),
                      )
            return data ?? { results: [], total: 0 }
        },
        // Keep the current page on screen while the next one loads.
        placeholderData: keepPreviousData,
    })
}

type InboxCreateBody = components["schemas"]["CreateInboxMessageRequest"]

function useSubjectInbox(subjectType: InboxSubjectType, subjectId: string) {
    const [project] = useContext(ProjectContext)
    const queryClient = useQueryClient()
    return {
        projectId: project.id,
        invalidate: () =>
            queryClient.invalidateQueries({
                queryKey: inboxKeys.subject(project.id, subjectType, subjectId),
            }),
    }
}

export function useCreateInboxMessage(subjectType: InboxSubjectType, subjectId: string) {
    const { projectId, invalidate } = useSubjectInbox(subjectType, subjectId)
    return useMutation({
        mutationFn: async (body: InboxCreateBody) =>
            subjectType === "users"
                ? unwrap(
                      await oapiClient.POST(
                          "/api/admin/projects/{projectID}/subjects/users/{userID}/inbox",
                          { params: { path: { projectID: projectId, userID: subjectId } }, body },
                      ),
                  )
                : unwrap(
                      await oapiClient.POST(
                          "/api/admin/projects/{projectID}/subjects/organizations/{organizationID}/inbox",
                          {
                              params: {
                                  path: { projectID: projectId, organizationID: subjectId },
                              },
                              body,
                          },
                      ),
                  ),
        onSuccess: invalidate,
        meta: {
            errorMessage: t("inbox_message_create_failed", "Failed to create inbox message"),
        },
    })
}

const userActionPaths = {
    opened: "/api/admin/projects/{projectID}/subjects/users/{userID}/inbox/{messageID}/read",
    archived: "/api/admin/projects/{projectID}/subjects/users/{userID}/inbox/{messageID}/archive",
    unarchived:
        "/api/admin/projects/{projectID}/subjects/users/{userID}/inbox/{messageID}/unarchive",
    unread: "/api/admin/projects/{projectID}/subjects/users/{userID}/inbox/{messageID}/unread",
} as const

const organizationActionPaths = {
    opened: "/api/admin/projects/{projectID}/subjects/organizations/{organizationID}/inbox/{messageID}/read",
    archived:
        "/api/admin/projects/{projectID}/subjects/organizations/{organizationID}/inbox/{messageID}/archive",
    unarchived:
        "/api/admin/projects/{projectID}/subjects/organizations/{organizationID}/inbox/{messageID}/unarchive",
    unread: "/api/admin/projects/{projectID}/subjects/organizations/{organizationID}/inbox/{messageID}/unread",
} as const

export interface InboxUpdate {
    messageId: string
    event: InboxEvent
    /** The new time, for the "scheduled" event. */
    scheduledAt?: string
}

export function useUpdateInboxMessage(subjectType: InboxSubjectType, subjectId: string) {
    const { projectId, invalidate } = useSubjectInbox(subjectType, subjectId)
    return useMutation({
        mutationFn: async ({ messageId, event, scheduledAt }: InboxUpdate) => {
            if (subjectType === "users") {
                const path = { projectID: projectId, userID: subjectId, messageID: messageId }
                return event === "scheduled"
                    ? unwrap(
                          await oapiClient.POST(
                              "/api/admin/projects/{projectID}/subjects/users/{userID}/inbox/{messageID}/schedule",
                              { params: { path }, body: { scheduled_at: scheduledAt ?? "" } },
                          ),
                      )
                    : unwrap(await oapiClient.POST(userActionPaths[event], { params: { path } }))
            }
            const path = { projectID: projectId, organizationID: subjectId, messageID: messageId }
            return event === "scheduled"
                ? unwrap(
                      await oapiClient.POST(
                          "/api/admin/projects/{projectID}/subjects/organizations/{organizationID}/inbox/{messageID}/schedule",
                          { params: { path }, body: { scheduled_at: scheduledAt ?? "" } },
                      ),
                  )
                : unwrap(
                      await oapiClient.POST(organizationActionPaths[event], { params: { path } }),
                  )
        },
        onSuccess: invalidate,
        meta: {
            errorMessage: t("inbox_message_update_failed", "Failed to update inbox message"),
        },
    })
}
