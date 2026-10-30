import { useCallback, type SetStateAction, type Dispatch } from "react"
import { toast } from "sonner"
import { useTranslation } from "react-i18next"
import { stepsToNodes, nodesToSteps } from "../editor/JourneyEditor.utils"
import type { JourneyEdge, JourneyNode } from "../editor/JourneyEditor.types"
import { usePublishJourney, useSaveJourneySteps } from "../mutations"
import type { JourneyStepMap } from "@/types"
import type { UUID } from "@/types/common"

type Actions = {
    skipDelay?: (stepId: string) => Promise<void>
    openUserModal?: (nodeId: string) => void
}

// Failures are reported by the mutation cache; saveSteps and publishJourney
// still reject so callers can skip what should only follow a successful save.
export function useJourneyPersistence(
    journeyId: UUID,
    setNodes: Dispatch<SetStateAction<JourneyNode[]>>,
    setEdges: Dispatch<SetStateAction<JourneyEdge[]>>,
    hasUnsavedChanges: boolean,
    setHasUnsavedChanges: Dispatch<SetStateAction<boolean>>,
) {
    const { t } = useTranslation()
    const { mutateAsync: save, isPending: saving } = useSaveJourneySteps()
    const { mutateAsync: publish, isPending: publishing } = usePublishJourney()

    // Shows the steps as the server saved them (with their new step ids).
    const showSaved = useCallback(
        (steps: JourneyStepMap, actions: Actions) => {
            const refreshed = stepsToNodes(steps, actions)
            setNodes(refreshed.nodes)
            setEdges(refreshed.edges)
        },
        [setNodes, setEdges],
    )

    const saveSteps = useCallback(
        async (nodes: JourneyNode[], edges: JourneyEdge[], actions: Actions) => {
            const { steps } = await save({ journeyId, steps: nodesToSteps(nodes, edges) })
            showSaved(steps, actions)
            setHasUnsavedChanges(false)
            toast.success(t("journey_saved"))
        },
        [save, journeyId, showSaved, setHasUnsavedChanges, t],
    )

    const publishJourney = useCallback(
        async (nodes: JourneyNode[], edges: JourneyEdge[], actions: Actions) => {
            if (!confirm(t("journey_publish_confirmation"))) return
            const { steps } = await publish({
                journeyId,
                steps: hasUnsavedChanges ? nodesToSteps(nodes, edges) : undefined,
            })
            if (steps) showSaved(steps, actions)
            setHasUnsavedChanges(false)
            toast.success(t("journey_published", "Journey published"))
        },
        [publish, journeyId, hasUnsavedChanges, showSaved, setHasUnsavedChanges, t],
    )

    return { saving, publishing, saveSteps, publishJourney }
}
