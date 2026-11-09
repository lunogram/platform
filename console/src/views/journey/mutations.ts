import { useMutation, useQueryClient } from "@tanstack/react-query"
import { useContext } from "react"
import { t } from "i18next"
import api from "@/api"
import { ProjectContext } from "@/contexts"
import { oapiClient } from "@/oapi/client"
import { unwrap } from "@/oapi/unwrap"
import type { Journey, JourneyStepMap } from "@/types"
import type { UUID } from "@/types/common"
import { invalidateJourneyLists, journeyKeys } from "./queries"

type JourneyCreateParams = Parameters<typeof api.journeys.create>[1]
type JourneyUpdateParams = Parameters<typeof api.journeys.update>[2]

// useJourneyCache writes server responses into the cached journey (which the
// editor's JourneyContext reads) and marks the lists, which show its name and
// status, as outdated.
function useJourneyCache() {
    const [project] = useContext(ProjectContext)
    const queryClient = useQueryClient()
    const setJourney = (journey: Journey) => {
        queryClient.setQueryData(journeyKeys.detail(project.id, journey.id), journey)
        void invalidateJourneyLists(queryClient, project.id)
    }
    return { projectId: project.id, queryClient, setJourney }
}

export function useCreateJourney() {
    const { projectId, setJourney } = useJourneyCache()
    return useMutation({
        mutationFn: (params: JourneyCreateParams) => api.journeys.create(projectId, params),
        onSuccess: setJourney,
        meta: { errorMessage: t("journey_create_failed", "Couldn't create the journey") },
    })
}

export function useUpdateJourney() {
    const { projectId, setJourney } = useJourneyCache()
    return useMutation({
        mutationFn: ({ journeyId, params }: { journeyId: UUID; params: JourneyUpdateParams }) =>
            api.journeys.update(projectId, journeyId, params),
        onSuccess: setJourney,
        meta: { errorMessage: t("journey_save_failed", "Couldn't save the journey") },
    })
}

// useJourneyListMutation runs a journeys-list action, then refreshes every
// cached list (and journey) of the project so the change shows everywhere.
function useJourneyListMutation<T>(
    request: (projectId: UUID, journeyId: UUID) => Promise<T>,
    errorMessage: string,
) {
    const { projectId, queryClient } = useJourneyCache()
    return useMutation({
        mutationFn: (journeyId: UUID) => request(projectId, journeyId),
        onSuccess: () => queryClient.invalidateQueries({ queryKey: journeyKeys.all(projectId) }),
        meta: { errorMessage },
    })
}

export function useDuplicateJourney() {
    return useJourneyListMutation(
        (projectId, journeyId) => api.journeys.duplicate(projectId, journeyId),
        t("journey_duplicate_failed", "Couldn't duplicate the journey"),
    )
}

export function useArchiveJourney() {
    return useJourneyListMutation(
        (projectId, journeyId) => api.journeys.delete(projectId, journeyId),
        t("journey_archive_failed", "Couldn't archive the journey"),
    )
}

export function useUnarchiveJourney() {
    return useJourneyListMutation(
        async (projectId, journeyId) =>
            unwrap(
                await oapiClient.POST(
                    "/api/admin/projects/{projectID}/journeys/{journeyID}/unarchive",
                    { params: { path: { projectID: projectId, journeyID: journeyId } } },
                ),
            ),
        t("journey_unarchive_failed", "Couldn't unarchive the journey"),
    )
}

interface SaveStepsParams {
    journeyId: UUID
    steps: JourneyStepMap
}

/**
 * Saves a journey's steps and reloads the journey, whose status reflects the
 * new draft. The journey id is part of the variables, not of the hook, so a
 * save always targets the journey whose steps were passed.
 */
export function useSaveJourneySteps() {
    const { projectId, queryClient, setJourney } = useJourneyCache()
    return useMutation({
        mutationFn: async ({ journeyId, steps }: SaveStepsParams) => {
            const saved = await api.journeys.steps.set(projectId, journeyId, steps)
            const journey = await api.journeys.get(projectId, journeyId)
            return { steps: saved, journey }
        },
        onSuccess: ({ steps, journey }) => {
            queryClient.setQueryData(journeyKeys.steps(projectId, journey.id), steps)
            setJourney(journey)
        },
        meta: { errorMessage: t("journey_save_steps_failed", "Couldn't save the journey") },
    })
}

/** Publishes a journey, first saving its steps when they are passed. */
export function usePublishJourney() {
    const { projectId, queryClient, setJourney } = useJourneyCache()
    return useMutation({
        mutationFn: async ({ journeyId, steps }: { journeyId: UUID; steps?: JourneyStepMap }) => {
            const saved = steps
                ? await api.journeys.steps.set(projectId, journeyId, steps)
                : undefined
            await api.journeys.publish(projectId, journeyId)
            const journey = await api.journeys.get(projectId, journeyId)
            return { steps: saved, journey }
        },
        onSuccess: ({ steps, journey }) => {
            if (steps) queryClient.setQueryData(journeyKeys.steps(projectId, journey.id), steps)
            setJourney(journey)
        },
        meta: { errorMessage: t("journey_publish_failed", "Couldn't publish the journey") },
    })
}
