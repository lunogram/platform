import { useState, useCallback, type SetStateAction, type Dispatch } from "react"
import { oapiClient } from "@/oapi/client"
import { toast } from "sonner"
import { useTranslation } from "react-i18next"
import { stepsToNodes, nodesToSteps } from "../editor/JourneyEditor.utils"
import type { JourneyEdge, JourneyNode } from "../editor/JourneyEditor.types"
import type { Journey, Project } from "@/types"
import type { UUID } from "@/types/common"

type Actions = {
    setViewUsersStep?: (step: { stepId: UUID; stepType: string; stepName?: string }) => void
    skipDelay?: (stepId: string) => Promise<void>
    openUserModal?: (nodeId: string) => void
}

export function useJourneyPersistence(
    project: Project,
    journey: Journey,
    setJourney: Dispatch<SetStateAction<Journey>>,
    setNodes: Dispatch<SetStateAction<JourneyNode[]>>,
    setEdges: Dispatch<SetStateAction<JourneyEdge[]>>,
    hasUnsavedChanges: boolean,
    setHasUnsavedChanges: Dispatch<SetStateAction<boolean>>,
) {
    const { t } = useTranslation()
    const [saving, setSaving] = useState(false)
    const [publishing, setPublishing] = useState(false)

    const saveDraft = useCallback(
        async (nodes: JourneyNode[], edges: JourneyEdge[], actions: Actions) => {
            const { data: stepMap, error } = await oapiClient.PUT(
                "/api/admin/projects/{projectID}/journeys/{journeyID}/steps",
                {
                    params: {
                        path: { projectID: project.id, journeyID: journey.id },
                    },
                    body: nodesToSteps(nodes, edges),
                },
            )
            if (error || !stepMap) throw error ?? new Error("Failed to save journey steps")
            return stepsToNodes(stepMap, actions)
        },
        [project.id, journey.id],
    )

    const saveSteps = useCallback(
        async (nodes: JourneyNode[], edges: JourneyEdge[], actions: Actions) => {
            setSaving(true)
            try {
                const refreshed = await saveDraft(nodes, edges, actions)
                setNodes(refreshed.nodes)
                setEdges(refreshed.edges)
                setHasUnsavedChanges(false)

                // Refresh journey state so status reflects the draft
                const { data: updated } = await oapiClient.GET(
                    "/api/admin/projects/{projectID}/journeys/{journeyID}",
                    {
                        params: {
                            path: { projectID: project.id, journeyID: journey.id },
                        },
                    },
                )
                if (updated) setJourney(updated as Journey)

                toast.success(t("journey_saved"))
            } catch (e) {
                toast.error(`Error: ${e}`)
                throw e
            } finally {
                setSaving(false)
            }
        },
        [
            saveDraft,
            setNodes,
            setEdges,
            setJourney,
            setHasUnsavedChanges,
            project.id,
            journey.id,
            t,
        ],
    )

    const publishJourney = useCallback(
        async (nodes: JourneyNode[], edges: JourneyEdge[], actions: Actions) => {
            if (!confirm(t("journey_publish_confirmation"))) return
            setPublishing(true)
            try {
                if (hasUnsavedChanges) {
                    const refreshed = await saveDraft(nodes, edges, actions)
                    setNodes(refreshed.nodes)
                    setEdges(refreshed.edges)
                }
                const { error } = await oapiClient.POST(
                    "/api/admin/projects/{projectID}/journeys/{journeyID}/publish",
                    {
                        params: {
                            path: { projectID: project.id, journeyID: journey.id },
                        },
                    },
                )
                if (error) throw error

                // Refresh journey state to reflect published status
                const { data: updated } = await oapiClient.GET(
                    "/api/admin/projects/{projectID}/journeys/{journeyID}",
                    {
                        params: {
                            path: { projectID: project.id, journeyID: journey.id },
                        },
                    },
                )
                if (updated) setJourney(updated as Journey)
                setHasUnsavedChanges(false)
                toast.success(t("journey_published", "Journey published"))
            } catch (e) {
                toast.error(`Error: ${e}`)
                throw e
            } finally {
                setPublishing(false)
            }
        },
        [
            project.id,
            journey.id,
            hasUnsavedChanges,
            saveDraft,
            setNodes,
            setEdges,
            setJourney,
            setHasUnsavedChanges,
            t,
        ],
    )

    return {
        saving,
        publishing,
        hasUnsavedChanges,
        setHasUnsavedChanges,
        saveDraft,
        saveSteps,
        publishJourney,
    }
}
