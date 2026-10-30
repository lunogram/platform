import { queryOptions, useMutation, useQueryClient } from "@tanstack/react-query"
import { t } from "i18next"
import { oapiClient } from "@/oapi/client"
import type { components } from "@/oapi/management.generated"
import { unwrap } from "@/oapi/unwrap"
import type { UUID } from "@/types/common"

type EventWithSchema = components["schemas"]["EventWithSchema"]

export interface EventSchemaItem extends EventWithSchema {
    subject_type: "user" | "organization" | "scheduled"
}

/** The user, organization and scheduled event schemas of the project, as one list. */
export function eventSchemasQuery(projectId: UUID) {
    const path = { projectID: projectId }
    return queryOptions({
        queryKey: ["projects", projectId, "event-schemas"],
        queryFn: async (): Promise<EventSchemaItem[]> => {
            const [user, organization, scheduled] = await Promise.all([
                oapiClient
                    .GET("/api/admin/projects/{projectID}/subjects/user/events/schema", {
                        params: { path },
                    })
                    .then(unwrap),
                oapiClient
                    .GET("/api/admin/projects/{projectID}/subjects/organization/events/schema", {
                        params: { path },
                    })
                    .then(unwrap),
                oapiClient
                    .GET("/api/admin/projects/{projectID}/subjects/user/scheduled/schema", {
                        params: { path },
                    })
                    .then(unwrap),
            ])
            return [
                ...(user?.results ?? []).map((e) => ({ ...e, subject_type: "user" as const })),
                ...(organization?.results ?? []).map((e) => ({
                    ...e,
                    subject_type: "organization" as const,
                })),
                ...(scheduled?.results ?? []).map((e) => ({
                    ...e,
                    subject_type: "scheduled" as const,
                })),
            ]
        },
    })
}

export function useDeleteEventSchema(projectId: UUID) {
    const queryClient = useQueryClient()
    return useMutation({
        mutationFn: async (event: EventSchemaItem) => {
            if (event.subject_type === "user") {
                return unwrap(
                    await oapiClient.DELETE(
                        "/api/admin/projects/{projectID}/subjects/user/events/schema/{eventID}",
                        { params: { path: { projectID: projectId, eventID: event.id } } },
                    ),
                )
            }
            if (event.subject_type === "organization") {
                return unwrap(
                    await oapiClient.DELETE(
                        "/api/admin/projects/{projectID}/subjects/organization/events/schema/{eventID}",
                        { params: { path: { projectID: projectId, eventID: event.id } } },
                    ),
                )
            }
            return unwrap(
                await oapiClient.DELETE(
                    "/api/admin/projects/{projectID}/subjects/user/scheduled/schema/{scheduledID}",
                    { params: { path: { projectID: projectId, scheduledID: event.id } } },
                ),
            )
        },
        onSuccess: () =>
            queryClient.invalidateQueries({ queryKey: eventSchemasQuery(projectId).queryKey }),
        meta: { errorMessage: t("schema_delete_failed", "Couldn't delete the schema") },
    })
}
