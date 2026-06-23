import type { Connection, Edge, Node } from "@xyflow/react"
import { MarkerType, getConnectedEdges } from "@xyflow/react"
import * as journeySteps from "../steps/index"
import { createUuid } from "@/utils"
import type { JourneyStepMap, JourneyStepType } from "@/types"
import { STEP_STYLE } from "../hooks/JourneyEditor.constants"
import type { JourneyEdge, JourneyNode, JourneyNodeData } from "./JourneyEditor.types"
import type { UUID } from "@/types/common"

export const getStepType = (type: string) =>
    (type ? (journeySteps[type as keyof typeof journeySteps] as JourneyStepType) : null) ?? null

export const getSourcePath = (handleId: string) => handleId.substring(0, handleId.indexOf("-s-"))

export function isValidJourneyConnection(
    connection: Connection | JourneyEdge,
    nodes: JourneyNode[],
    edges: JourneyEdge[],
) {
    const { source, target } = connection
    const sourceHandle = connection.sourceHandle ?? null
    const targetHandle = connection.targetHandle ?? null
    if (!source || !sourceHandle || !target || source === target) return false

    const sourceNode = nodes.find((node) => node.id === source)
    const targetNode = nodes.find((node) => node.id === target)
    if (!sourceNode || !targetNode) return false

    const sourceType = sourceNode.data.type ? getStepType(sourceNode.data.type) : null
    const targetType = targetNode.data.type ? getStepType(targetNode.data.type) : null
    if (!sourceType || !targetType || targetType.hideTopHandle) return false

    if (
        edges.some(
            (edge) =>
                edge.source === source &&
                edge.sourceHandle === sourceHandle &&
                edge.target === target &&
                edge.targetHandle === targetHandle,
        )
    )
        return false
    if (wouldCreateCycle(edges, source, target)) return false

    if (sourceType.multiChildSources) return true

    return (
        edges.filter((edge) => edge.source === source && edge.sourceHandle === sourceHandle)
            .length === 0
    )
}

export function wouldCreateCycle(edges: JourneyEdge[], source: string, target: string) {
    const childrenBySource = new Map<string, string[]>()
    for (const edge of edges) {
        const children = childrenBySource.get(edge.source) ?? []
        children.push(edge.target)
        childrenBySource.set(edge.source, children)
    }

    const queue = [target]
    const visited = new Set<string>()

    while (queue.length > 0) {
        const current = queue.shift()!
        if (current === source) return true
        if (visited.has(current)) continue
        visited.add(current)

        for (const child of childrenBySource.get(current) ?? []) {
            queue.push(child)
        }
    }

    return false
}

interface CreateEdgeParams {
    sourceId: string
    targetId: string
    data: Record<string, unknown>
    path?: string
}

export function createEdge({ data, sourceId, targetId, path }: CreateEdgeParams): JourneyEdge {
    const sourcePath = path ?? ""
    const sourceHandle = `${sourcePath}-s-${sourceId}`

    return {
        id: `e-${sourceId}__${sourcePath}__${targetId}`,
        source: sourceId,
        sourceHandle,
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
        setViewUsersStep?: (step: { stepId: UUID; stepType: string; stepName?: string }) => void
        skipDelay?: (stepId: string) => Promise<void>
        openUserModal?: (nodeId: string) => void
    },
) {
    const nodes: JourneyNode[] = []
    const edges: JourneyEdge[] = []
    const entries = Object.entries(stepMap)
    const nodeIds = new Set(entries.map(([id]) => id))

    for (const [
        id,
        { x, y, type, data, name, data_key, children, stats, stats_at, id: stepId },
    ] of entries) {
        const { width, height, ...restData } = (data as Record<string, unknown>) ?? {}
        const sizeStyle =
            typeof width === "number" && typeof height === "number" ? { width, height } : undefined
        nodes.push({
            id,
            position: { x, y },
            style: sizeStyle,
            type: "step",
            data: {
                type,
                name,
                data_key,
                data: restData,
                stats,
                stats_at,
                stepId: stepId ?? id,
                width: typeof width === "number" ? width : undefined,
                height: typeof height === "number" ? height : undefined,
                ...actions,
            },
        })
        children
            ?.filter(({ external_id }) => nodeIds.has(external_id))
            .forEach(({ external_id, path, data }) =>
                edges.push(createEdge({ sourceId: id, targetId: external_id, data, path })),
            )
    }
    return { nodes, edges }
}

export function nodesToSteps(nodes: JourneyNode[], edges: JourneyEdge[]) {
    const nodeIds = new Set(nodes.map(({ id }) => id))

    return nodes.reduce<JourneyStepMap>(
        (
            a,
            {
                id,
                data: { type, name = "", data_key, data = {}, width, height },
                position: { x, y },
            },
        ) => {
            a[id] = {
                type,
                data: {
                    ...data,
                    ...(typeof width === "number" && typeof height === "number"
                        ? { width, height }
                        : {}),
                },
                name,
                data_key,
                x,
                y,
                children: edges
                    .filter((e) => e.source === id && nodeIds.has(e.target) && e.sourceHandle)
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
    const nodesById = new Map(nodes.map((node) => [node.id, node]))

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
        const node = nodesById.get(current)
        if (node?.data.data_key) {
            const stepData = node.data.data as Record<string, unknown> | undefined
            // Entrance steps carry trigger details in nested `event` /
            // `scheduled` blocks; other step types still expose `event_name` at
            // the top level, so fall back to that.
            const eventBlock = stepData?.event as { name?: string } | undefined
            const scheduledBlock = stepData?.scheduled as
                | { name?: string; offset_id?: string }
                | undefined
            result.push({
                nodeId: node.id,
                name: node.data.name ?? "",
                type: node.data.type,
                data_key: node.data.data_key,
                event_name:
                    eventBlock?.name ??
                    scheduledBlock?.name ??
                    (stepData?.event_name as string) ??
                    undefined,
                scheduled_name: (stepData?.scheduled_name as string) ?? undefined,
                schedule_offset_id:
                    scheduledBlock?.offset_id ??
                    (stepData?.schedule_offset_id as string) ??
                    undefined,
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

export function cloneNodes(edges: JourneyEdge[], targets: JourneyNode[]) {
    const mapping: { [prev: string]: string } = {}
    const nodeCopies: JourneyNode[] = []
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
