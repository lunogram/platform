import type { Edge, Node } from "reactflow"
import { MarkerType, getConnectedEdges } from "reactflow"
import * as journeySteps from "../steps/index"
import { createUuid } from "@/utils"
import type { JourneyStepMap, JourneyStepType } from "@/types"
import { STEP_STYLE } from "../hooks/JourneyEditor.constants"
import type { JourneyNode, JourneyNodeData } from "./JourneyEditor.types"
import type { UUID } from "@/types/common"

export const getStepType = (type: string) =>
    (type ? (journeySteps[type as keyof typeof journeySteps] as JourneyStepType) : null) ?? null

export const getSourcePath = (handleId: string) => handleId.substring(0, handleId.indexOf("-s-"))

interface CreateEdgeParams {
    sourceId: string
    targetId: string
    data: Record<string, unknown>
    path?: string
}

export function createEdge({ data, sourceId, targetId, path }: CreateEdgeParams): Edge {
    return {
        id: "e-" + sourceId + "__" + targetId,
        source: sourceId,
        sourceHandle: (path ?? "") + "-s-" + sourceId,
        target: targetId,
        targetHandle: "t-" + targetId,
        data,
        type: STEP_STYLE,
        markerEnd: {
            type: MarkerType.ArrowClosed,
        },
    }
}

export function stepsToNodes(
    stepMap: JourneyStepMap,
    actions: {
        setViewUsersStep?: (step: { stepId: UUID; stepType: string }) => void
        skipDelay?: (stepId: string) => Promise<void>
        openUserModal?: (nodeId: string) => void
    },
) {
    const nodes: JourneyNode[] = []
    const edges: Edge[] = []

    for (const [
        id,
        { x, y, type, data, name, data_key, children, stats, stats_at, id: stepId },
    ] of Object.entries(stepMap)) {
        nodes.push({
            id,
            position: { x, y },
            type: "step",
            data: { type, name, data_key, data, stats, stats_at, stepId: stepId ?? id, ...actions },
        })
        children?.forEach(({ external_id, path, data }) =>
            edges.push(createEdge({ sourceId: id, targetId: external_id, data, path })),
        )
    }
    return { nodes, edges }
}

export function nodesToSteps(nodes: Node<JourneyNodeData>[], edges: Edge[]) {
    return nodes.reduce<JourneyStepMap>(
        (a, { id, data: { type, name = "", data_key, data = {} }, position: { x, y } }) => {
            a[id] = {
                type,
                data,
                name,
                data_key,
                x,
                y,
                children: edges
                    .filter((e) => e.source === id)
                    .map(({ data = {}, sourceHandle, target }) => ({
                        external_id: target,
                        path: getSourcePath(sourceHandle!),
                        data,
                    })),
            }
            return a
        },
        {},
    )
}

/**
 * Walk edges backward from `nodeId` to find all ancestor nodes that have a
 * non-empty `data_key`.  Returns them in topological order (closest ancestors
 * first).
 */
export interface UpstreamDataKey {
    nodeId: string
    name: string
    type: string
    data_key: string
    /** The event_name from the step's data (if it captures an event, e.g. entrance) */
    event_name?: string
    /** The scheduled_name from the step's data (if trigger is "scheduled") */
    scheduled_name?: string
    /** The schedule_offset_id from the step's data (if it captures a scheduled event) */
    schedule_offset_id?: string
}

export function getUpstreamDataKeys(
    nodes: Node<JourneyNodeData>[],
    edges: Edge[],
    nodeId: string,
): UpstreamDataKey[] {
    // Build a reverse adjacency map: target -> sources
    const parentMap = new Map<string, string[]>()
    for (const edge of edges) {
        const sources = parentMap.get(edge.target) ?? []
        sources.push(edge.source)
        parentMap.set(edge.target, sources)
    }

    // BFS backward from nodeId
    const visited = new Set<string>()
    const queue: string[] = parentMap.get(nodeId) ?? []
    const result: UpstreamDataKey[] = []

    for (const id of queue) visited.add(id)

    while (queue.length > 0) {
        const current = queue.shift()!
        const node = nodes.find((n) => n.id === current)
        if (node?.data.data_key) {
            const stepData = node.data.data as Record<string, unknown> | undefined
            result.push({
                nodeId: node.id,
                name: node.data.name ?? "",
                type: node.data.type,
                data_key: node.data.data_key,
                event_name: (stepData?.event_name as string) ?? undefined,
                scheduled_name: (stepData?.scheduled_name as string) ?? undefined,
                schedule_offset_id: (stepData?.schedule_offset_id as string) ?? undefined,
            })
        }
        for (const parent of parentMap.get(current) ?? []) {
            if (!visited.has(parent)) {
                visited.add(parent)
                queue.push(parent)
            }
        }
    }

    return result
}

export function cloneNodes(edges: Edge[], targets: Node<JourneyNodeData>[]) {
    const mapping: { [prev: string]: string } = {}
    const nodeCopies: Node<JourneyNodeData>[] = []
    for (const node of targets) {
        const id = createUuid()
        mapping[node.id] = id
        nodeCopies.push({
            ...node,
            data: {
                ...(node.data ?? {}),
                name: node.data.name ? node.data.name + " (Copy)" : undefined,
            },
            id,
            position: { x: node.position.x + 50, y: node.position.y + 50 },
        })
    }
    const edgeCopies = getConnectedEdges(targets, edges)
        .filter((edge) => edge.source in mapping && edge.target in mapping)
        .map((edge) =>
            createEdge({
                sourceId: mapping[edge.source],
                targetId: mapping[edge.target],
                data: edge.data ?? {},
                path: getSourcePath(edge.sourceHandle!),
            }),
        )
    return { nodeCopies, edgeCopies }
}
