import { useMutation } from "@tanstack/react-query"
import { useNavigate } from "react-router"
import { t } from "i18next"
import { oapiClient } from "@/oapi/client"
import type { components } from "@/oapi/management.generated"
import { unwrap } from "@/oapi/unwrap"
import type { UUID } from "@/types/common"

/**
 * Creates the "Onboarding" journey from its template and opens it. With
 * `reuseExisting`, a project that already has a journey opens that one instead.
 * A failed request is reported by the mutation cache.
 */
export function useCreateOnboardingJourney(projectId: UUID, { reuseExisting = false } = {}) {
    const navigate = useNavigate()
    return useMutation({
        mutationFn: async () => {
            if (reuseExisting) {
                const journeys = unwrap(
                    await oapiClient.GET("/api/admin/projects/{projectID}/journeys", {
                        params: { path: { projectID: projectId }, query: { limit: 1 } },
                    }),
                )
                const existing = journeys.results ?? []
                if (existing.length > 0) return existing[0]
            }
            return unwrap(
                await oapiClient.POST("/api/admin/projects/{projectID}/journeys", {
                    params: { path: { projectID: projectId } },
                    body: {
                        name: "Onboarding",
                        description: "Getting started with your first journey",
                        template_id: "onboarding",
                        status: "draft",
                    } as components["schemas"]["CreateJourney"],
                }),
            )
        },
        onSuccess: (journey) => navigate(`/projects/${projectId}/journeys/${journey.id}`),
        meta: {
            errorMessage: t("onboarding_journey_create_failed", "Couldn't create the journey"),
        },
    })
}
