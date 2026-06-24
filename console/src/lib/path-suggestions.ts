import { oapiClient } from "@/oapi/client"
import type { UUID } from "@/types/common"
import type { VariableSuggestions } from "@/types"

/**
 * Fetches variable path suggestions for a project by aggregating the various
 * subject schema endpoints. Organization-related schemas are optional and fall
 * back to empty arrays when the endpoints are unavailable.
 *
 * Replaces the legacy `api.projects.pathSuggestions` aggregator; uses the typed
 * OpenAPI client throughout.
 */
export async function fetchPathSuggestions(projectId: UUID): Promise<VariableSuggestions> {
    const path = { projectID: projectId }

    const { data: userEvents } = await oapiClient.GET(
        "/api/admin/projects/{projectID}/subjects/user/events/schema",
        { params: { path } },
    )
    const eventPaths = (userEvents?.results ?? []).map((event) => ({
        ...event,
        schema: event.schema ?? [],
    })) as VariableSuggestions["eventPaths"]

    const { data: users } = await oapiClient.GET(
        "/api/admin/projects/{projectID}/subjects/users/schema",
        { params: { path } },
    )
    const userPaths = (users?.results ?? []) as VariableSuggestions["userPaths"]

    let scheduledPaths: VariableSuggestions["scheduledPaths"] = []
    try {
        const { data } = await oapiClient.GET(
            "/api/admin/projects/{projectID}/subjects/user/scheduled/schema",
            { params: { path } },
        )
        scheduledPaths = (data?.results ?? []).map((s) => ({
            ...s,
            schema: s.schema ?? [],
        })) as VariableSuggestions["scheduledPaths"]
    } catch (error) {
        console.debug("Failed to fetch scheduled schemas:", error)
    }

    let organizationEventPaths: VariableSuggestions["organizationEventPaths"] = []
    try {
        const { data } = await oapiClient.GET(
            "/api/admin/projects/{projectID}/subjects/organization/events/schema",
            { params: { path } },
        )
        organizationEventPaths = (data?.results ?? []).map((event) => ({
            ...event,
            schema: event.schema ?? [],
        })) as VariableSuggestions["organizationEventPaths"]
    } catch (error) {
        console.debug("Failed to fetch organization event schemas:", error)
    }

    let organizationUserPaths: VariableSuggestions["organizationUserPaths"] = []
    try {
        const { data } = await oapiClient.GET(
            "/api/admin/projects/{projectID}/subjects/organizations/users/schema",
            { params: { path } },
        )
        organizationUserPaths = (data?.results ??
            []) as VariableSuggestions["organizationUserPaths"]
    } catch (error) {
        console.debug("Failed to fetch organization user schemas:", error)
    }

    let organizationPaths: VariableSuggestions["organizationPaths"] = []
    try {
        const { data } = await oapiClient.GET(
            "/api/admin/projects/{projectID}/subjects/organizations/schema",
            { params: { path } },
        )
        organizationPaths = (data?.results ?? []) as VariableSuggestions["organizationPaths"]
    } catch (error) {
        console.debug("Failed to fetch organization schemas:", error)
    }

    return {
        eventPaths,
        userPaths,
        scheduledPaths,
        organizationEventPaths,
        organizationUserPaths,
        organizationPaths,
    }
}
