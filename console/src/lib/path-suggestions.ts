import { oapiClient } from "@/oapi/client"
import type { UUID } from "@/types/common"
import type { VariableSuggestions } from "@/types"

/**
 * Swallow a failure from an endpoint that not every deployment serves, so one
 * missing schema source cannot take the whole aggregate down.
 */
function optional<T>(request: Promise<T>, what: string): Promise<T | undefined> {
    return request.catch((error) => {
        console.debug(`Failed to fetch ${what}:`, error)
        return undefined
    })
}

/**
 * Fetches variable path suggestions for a project by aggregating the various
 * subject schema endpoints. Organization-related schemas are optional and fall
 * back to empty arrays when the endpoints are unavailable.
 *
 * The six endpoints are independent, so they go out together: requested one
 * after another this cost six round trips, which editors that must wait for a
 * complete variable list before mounting pay in full.
 *
 * Replaces the legacy `api.projects.pathSuggestions` aggregator; uses the typed
 * OpenAPI client throughout.
 */
export async function fetchPathSuggestions(projectId: UUID): Promise<VariableSuggestions> {
    const path = { projectID: projectId }

    const [userEvents, users, scheduled, organizationEvents, organizationUsers, organizations] =
        await Promise.all([
            oapiClient.GET("/api/admin/projects/{projectID}/subjects/user/events/schema", {
                params: { path },
            }),
            oapiClient.GET("/api/admin/projects/{projectID}/subjects/users/schema", {
                params: { path },
            }),
            optional(
                oapiClient.GET("/api/admin/projects/{projectID}/subjects/user/scheduled/schema", {
                    params: { path },
                }),
                "scheduled schemas",
            ),
            optional(
                oapiClient.GET(
                    "/api/admin/projects/{projectID}/subjects/organization/events/schema",
                    {
                        params: { path },
                    },
                ),
                "organization event schemas",
            ),
            optional(
                oapiClient.GET(
                    "/api/admin/projects/{projectID}/subjects/organizations/users/schema",
                    { params: { path } },
                ),
                "organization user schemas",
            ),
            optional(
                oapiClient.GET("/api/admin/projects/{projectID}/subjects/organizations/schema", {
                    params: { path },
                }),
                "organization schemas",
            ),
        ])

    const eventPaths = (userEvents.data?.results ?? []).map((event) => ({
        ...event,
        schema: event.schema ?? [],
    })) as VariableSuggestions["eventPaths"]

    const userPaths = (users.data?.results ?? []) as VariableSuggestions["userPaths"]

    const scheduledPaths = (scheduled?.data?.results ?? []).map((s) => ({
        ...s,
        schema: s.schema ?? [],
    })) as VariableSuggestions["scheduledPaths"]

    const organizationEventPaths = (organizationEvents?.data?.results ?? []).map((event) => ({
        ...event,
        schema: event.schema ?? [],
    })) as VariableSuggestions["organizationEventPaths"]

    const organizationUserPaths = (organizationUsers?.data?.results ??
        []) as VariableSuggestions["organizationUserPaths"]

    const organizationPaths = (organizations?.data?.results ??
        []) as VariableSuggestions["organizationPaths"]

    return {
        eventPaths,
        userPaths,
        scheduledPaths,
        organizationEventPaths,
        organizationUserPaths,
        organizationPaths,
    }
}
