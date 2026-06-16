import { createContext, useContext } from "react"

/**
 * Lightweight context that signals to source handles whether the editor
 * should hint the user to connect steps. Computed once at the editor level
 * to avoid every node re-running the orphan scan on each store transaction.
 */
export interface JourneyHintsValue {
    showConnectHint: boolean
}

export const JourneyHintsContext = createContext<JourneyHintsValue>({
    showConnectHint: false,
})

export const useJourneyHints = () => useContext(JourneyHintsContext)
