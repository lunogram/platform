import { createContext, useContext } from "react"

export interface JourneyEditorActionsValue {
    onEdgeDataChange: (edgeId: string, next: Record<string, unknown>) => void
    onEdgeDelete: (edgeId: string) => void
}

export const JourneyEditorActionsContext = createContext<JourneyEditorActionsValue>({
    onEdgeDataChange: () => {},
    onEdgeDelete: () => {},
})

export const useJourneyEditorActions = () => useContext(JourneyEditorActionsContext)
