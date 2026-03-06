import type { Node } from "reactflow"
import type { UUID } from "@/types/common"

export interface JourneyNodeData {
    stepId?: string
    type: string
    name?: string
    data?: Record<string, unknown>
    data_key?: string
    stats?: Record<string, number>
    stats_at?: Date
    visited?: boolean
    active?: boolean
    editing?: boolean
    skipDelay?: (stepId: string) => Promise<void>
    setViewUsersStep?: (step: { stepId: UUID; stepType: string }) => void
}

export type JourneyNode = Node<JourneyNodeData, string | undefined>
