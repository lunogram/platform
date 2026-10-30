import {
    keepPreviousData,
    queryOptions,
    useMutation,
    useQueryClient,
    type QueryClient,
} from "@tanstack/react-query"
import { useContext } from "react"
import { t } from "i18next"
import { OrganizationContext, ProjectContext } from "@/contexts"
import {
    oapiClient,
    type Organization,
    type UpdateOrganization,
    type UpsertOrganization,
} from "@/oapi/client"
import { unwrap } from "@/oapi/unwrap"
import type { UUID } from "@/types/common"

export const organizationKeys = {
    all: (projectId: UUID) => ["projects", projectId, "organizations"] as const,
    lists: (projectId: UUID) => [...organizationKeys.all(projectId), "list"] as const,
    detail: (projectId: UUID, organizationId: UUID) =>
        [...organizationKeys.all(projectId), organizationId] as const,
    members: (projectId: UUID, organizationId: UUID) =>
        [...organizationKeys.detail(projectId, organizationId), "members"] as const,
    events: (projectId: UUID, organizationId: UUID) =>
        [...organizationKeys.detail(projectId, organizationId), "events"] as const,
}

interface PageParams {
    limit: number
    offset: number
    search: string
}

const pageQuery = ({ limit, offset, search }: PageParams) => ({
    limit,
    offset,
    search: search || undefined,
})

/**
 * organizationQuery loads one organization. It is the only copy the detail
 * pages read, so a save on one tab is seen by the header and every other tab.
 */
export function organizationQuery(projectId: UUID, organizationId: UUID) {
    return queryOptions({
        queryKey: organizationKeys.detail(projectId, organizationId),
        queryFn: async () =>
            unwrap(
                await oapiClient.GET(
                    "/api/admin/projects/{projectID}/subjects/organizations/{organizationID}",
                    {
                        params: {
                            path: { projectID: projectId, organizationID: organizationId },
                        },
                    },
                ),
            ),
    })
}

/** One page of the organizations list. */
export function organizationListQuery(projectId: UUID, params: PageParams) {
    return queryOptions({
        queryKey: [...organizationKeys.lists(projectId), params] as const,
        queryFn: async () =>
            unwrap(
                await oapiClient.GET("/api/admin/projects/{projectID}/subjects/organizations", {
                    params: { path: { projectID: projectId }, query: pageQuery(params) },
                }),
            ),
        // Keep the current page on screen while the next one loads.
        placeholderData: keepPreviousData,
    })
}

/** One page of an organization's members. */
export function organizationMembersQuery(
    projectId: UUID,
    organizationId: UUID,
    params: PageParams,
) {
    return queryOptions({
        queryKey: [...organizationKeys.members(projectId, organizationId), params] as const,
        queryFn: async () =>
            unwrap(
                await oapiClient.GET(
                    "/api/admin/projects/{projectID}/subjects/organizations/{organizationID}/users",
                    {
                        params: {
                            path: { projectID: projectId, organizationID: organizationId },
                            query: pageQuery(params),
                        },
                    },
                ),
            ),
        placeholderData: keepPreviousData,
    })
}

/** One page of an organization's events. */
export function organizationEventsQuery(projectId: UUID, organizationId: UUID, params: PageParams) {
    return queryOptions({
        queryKey: [...organizationKeys.events(projectId, organizationId), params] as const,
        queryFn: async () =>
            unwrap(
                await oapiClient.GET(
                    "/api/admin/projects/{projectID}/subjects/organizations/{organizationID}/events",
                    {
                        params: {
                            path: { projectID: projectId, organizationID: organizationId },
                            query: pageQuery(params),
                        },
                    },
                ),
            ),
        placeholderData: keepPreviousData,
    })
}

/** Known organization event names, offered as suggestions when creating an event. */
export function organizationEventSchemasQuery(projectId: UUID) {
    return queryOptions({
        queryKey: ["projects", projectId, "organization-event-schemas"],
        queryFn: async () =>
            unwrap(
                await oapiClient.GET(
                    "/api/admin/projects/{projectID}/subjects/organization/events/schema",
                    { params: { path: { projectID: projectId } } },
                ),
            ).results ?? [],
    })
}

/** Marks every cached organizations list of the project as outdated. */
export function invalidateOrganizationLists(queryClient: QueryClient, projectId: UUID) {
    return queryClient.invalidateQueries({ queryKey: organizationKeys.lists(projectId) })
}

export function useCreateOrganization() {
    const [project] = useContext(ProjectContext)
    const queryClient = useQueryClient()
    return useMutation({
        mutationFn: async (body: UpsertOrganization) =>
            unwrap(
                await oapiClient.POST("/api/admin/projects/{projectID}/subjects/organizations", {
                    params: { path: { projectID: project.id } },
                    body,
                }),
            ),
        onSuccess: () => invalidateOrganizationLists(queryClient, project.id),
        meta: {
            errorMessage: t("organization_create_failed", "Couldn't create the organization"),
        },
    })
}

// useOrganizationCache addresses the cached copy of the organization in context.
function useOrganizationCache() {
    const [project] = useContext(ProjectContext)
    const [organization] = useContext(OrganizationContext)
    const queryClient = useQueryClient()
    return {
        queryClient,
        projectId: project.id,
        organizationId: organization.id,
        detailKey: organizationKeys.detail(project.id, organization.id),
    }
}

/** Saves organization fields; errorMessage is shown when the save fails. */
export function useUpdateOrganization(errorMessage: string) {
    const { queryClient, projectId, organizationId, detailKey } = useOrganizationCache()
    return useMutation({
        mutationFn: async (body: UpdateOrganization) =>
            unwrap(
                await oapiClient.PATCH(
                    "/api/admin/projects/{projectID}/subjects/organizations/{organizationID}",
                    {
                        params: {
                            path: { projectID: projectId, organizationID: organizationId },
                        },
                        body,
                    },
                ),
            ),
        onSuccess: (organization: Organization) => {
            queryClient.setQueryData(detailKey, organization)
            // The list shows the name, so it is outdated now too.
            void invalidateOrganizationLists(queryClient, projectId)
        },
        meta: { errorMessage },
    })
}

export function useDeleteOrganization() {
    const { queryClient, projectId, organizationId } = useOrganizationCache()
    return useMutation({
        mutationFn: async () =>
            unwrap(
                await oapiClient.DELETE(
                    "/api/admin/projects/{projectID}/subjects/organizations/{organizationID}",
                    {
                        params: {
                            path: { projectID: projectId, organizationID: organizationId },
                        },
                    },
                ),
            ),
        onSuccess: () => invalidateOrganizationLists(queryClient, projectId),
        meta: {
            errorMessage: t("organization_delete_failed", "Couldn't delete the organization"),
        },
    })
}

/** Adds a user to the organization, or replaces the data of an existing member. */
export function useSaveMember(errorMessage: string) {
    const { queryClient, projectId, organizationId } = useOrganizationCache()
    return useMutation({
        mutationFn: async (body: { user_id: UUID; data?: Record<string, unknown> }) =>
            unwrap(
                await oapiClient.POST(
                    "/api/admin/projects/{projectID}/subjects/organizations/{organizationID}/users",
                    {
                        params: {
                            path: { projectID: projectId, organizationID: organizationId },
                        },
                        body,
                    },
                ),
            ),
        onSuccess: () =>
            queryClient.invalidateQueries({
                queryKey: organizationKeys.members(projectId, organizationId),
            }),
        meta: { errorMessage },
    })
}

export function useRemoveMember() {
    const { queryClient, projectId, organizationId } = useOrganizationCache()
    return useMutation({
        mutationFn: async (userId: UUID) =>
            unwrap(
                await oapiClient.DELETE(
                    "/api/admin/projects/{projectID}/subjects/organizations/{organizationID}/users/{userID}",
                    {
                        params: {
                            path: {
                                projectID: projectId,
                                organizationID: organizationId,
                                userID: userId,
                            },
                        },
                    },
                ),
            ),
        onSuccess: () =>
            queryClient.invalidateQueries({
                queryKey: organizationKeys.members(projectId, organizationId),
            }),
        meta: {
            errorMessage: t("organization_member_remove_failed", "Couldn't remove the member"),
        },
    })
}

export function useCreateOrganizationEvent() {
    const { queryClient, projectId, organizationId } = useOrganizationCache()
    return useMutation({
        mutationFn: async (body: { name: string; data?: Record<string, unknown> }) =>
            unwrap(
                await oapiClient.POST(
                    "/api/admin/projects/{projectID}/subjects/organizations/{organizationID}/events",
                    {
                        params: {
                            path: { projectID: projectId, organizationID: organizationId },
                        },
                        body,
                    },
                ),
            ),
        onSuccess: () => {
            // Events are processed asynchronously, so give the new one a moment
            // to show up before reloading the list.
            setTimeout(
                () =>
                    void queryClient.invalidateQueries({
                        queryKey: organizationKeys.events(projectId, organizationId),
                    }),
                1000,
            )
        },
        meta: {
            errorMessage: t("failed_to_create_event", "Failed to create event"),
        },
    })
}
