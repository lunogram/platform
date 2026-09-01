import type { Edge, Node } from "@xyflow/react"
import type { UUID } from "@/types/common"

export interface JourneyEdgeData extends Record<string, unknown> {}

export type JourneyEdge = Edge<JourneyEdgeData, "step">

export interface JourneyNodeData extends Record<string, unknown> {
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
    hasUnsavedChanges?: boolean
    connectedSourceHandles?: string[]
    width?: number
    height?: number
    skipDelay?: (stepId: string) => Promise<void>
    openUserModal?: (nodeId: string) => void
}

export type JourneyNode = Node<JourneyNodeData, "step">
