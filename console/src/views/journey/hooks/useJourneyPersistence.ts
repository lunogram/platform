import { useState, useCallback, type SetStateAction, type Dispatch } from "react"
import api from "@/api"
import { toast } from "sonner"
import { useTranslation } from "react-i18next"
import { stepsToNodes, nodesToSteps } from "../editor/JourneyEditor.utils"
import type { JourneyNode } from "../editor/JourneyEditor.types"
import type { Edge } from "reactflow"
import type { Journey, Project } from "@/types"
import type { UUID } from "@/types/common"

type Actions = {
    setViewUsersStep?: (step: { stepId: UUID; stepType: string }) => void
    skipDelay?: (stepId: string) => Promise<void>
    openUserModal?: (nodeId: string) => void
}

export function useJourneyPersistence(
    project: Project,
    journey: Journey,
    setJourney: Dispatch<SetStateAction<Journey>>,
    setNodes: Dispatch<SetStateAction<JourneyNode[]>>,
    setEdges: Dispatch<SetStateAction<Edge[]>>,
) {
    const { t } = useTranslation()
    const [saving, setSaving] = useState(false)
    const [publishing, setPublishing] = useState(false)
    const [hasUnsavedChanges, setHasUnsavedChanges] = useState(false)

    const saveDraft = useCallback(
        async (nodes: JourneyNode[], edges: Edge[], actions: Actions) => {
            const stepMap = await api.journeys.steps.set(
                project.id,
                journey.id,
                nodesToSteps(nodes, edges),
            )
            return stepsToNodes(stepMap, actions)
        },
        [project.id, journey.id],
    )

    const saveSteps = useCallback(
        async (nodes: JourneyNode[], edges: Edge[], actions: Actions) => {
            setSaving(true)
            try {
                const refreshed = await saveDraft(nodes, edges, actions)
                setNodes(refreshed.nodes)
                setEdges(refreshed.edges)
                setHasUnsavedChanges(false)

                // Refresh journey state so status reflects the draft
                const updated = await api.journeys.get(project.id, journey.id)
                setJourney(updated)

                toast.success(t("journey_saved"))
            } catch (e) {
                toast.error(`Error: ${e}`)
            } finally {
                setSaving(false)
            }
        },
        [saveDraft, setNodes, setEdges, setJourney, project.id, journey.id, t],
    )

    const publishJourney = useCallback(
        async (nodes: JourneyNode[], edges: Edge[], actions: Actions) => {
            if (!confirm(t("journey_publish_confirmation"))) return
            if (hasUnsavedChanges) await saveDraft(nodes, edges, actions)
            setPublishing(true)
            try {
                await api.journeys.publish(project.id, journey.id)

                // Refresh journey state to reflect published status
                const updated = await api.journeys.get(project.id, journey.id)
                setJourney(updated)
                setHasUnsavedChanges(false)
            } finally {
                setPublishing(false)
            }
        },
        [project.id, journey.id, hasUnsavedChanges, saveDraft, setJourney, t],
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
