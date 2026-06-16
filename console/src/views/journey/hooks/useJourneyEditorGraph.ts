import { useCallback, useEffect, useMemo, useRef, useState } from "react"
import type { Connection, EdgeChange, NodeChange, ReactFlowInstance } from "@xyflow/react"
import { useEdgesState, useNodesState } from "@xyflow/react"
import { useTranslation } from "react-i18next"

import { createUuid } from "@/utils"
import type { EntranceTrigger } from "../components/JourneyTriggerSetup"
import type { JourneyEditorActionsValue } from "../editor/JourneyEditorActions"
import type { JourneyHintsValue } from "../editor/JourneyHints"
import type { JourneyEdge, JourneyNode } from "../editor/JourneyEditor.types"
import { cloneNodes, getStepType, isValidJourneyConnection } from "../editor/JourneyEditor.utils"
import { useJourneyFlowHandlers } from "./useJourneyFlowHandlers"
import { useKeyboardShortcuts } from "./useKeyboardShortcuts"
import { useStepEditing } from "./useStepEditing"

interface UseJourneyEditorGraphParams {
    isEditable: boolean
    stepsLoaded: boolean
    followingUserId: string | null
    replayUserId: string | null
}

export interface JourneyEditorGraph {
    wrapperRef: React.RefObject<HTMLDivElement | null>
    nodes: JourneyNode[]
    edges: JourneyEdge[]
    setNodes: React.Dispatch<React.SetStateAction<JourneyNode[]>>
    setEdges: React.Dispatch<React.SetStateAction<JourneyEdge[]>>
    editNode?: JourneyNode
    selectedCount: number
    hasUnsavedChanges: boolean
    setHasUnsavedChanges: React.Dispatch<React.SetStateAction<boolean>>
    clearHistory: () => void
    editorActions: JourneyEditorActionsValue
    hintsValue: JourneyHintsValue
    createEntranceFromSetup: (trigger: EntranceTrigger) => Promise<void>
    onUserEnteredNode: (nodeId: string) => void
    onStepExecuted: (nodeId: string) => void
    resetRuntimeState: () => void
    handlers: {
        onInit: React.Dispatch<
            React.SetStateAction<ReactFlowInstance<JourneyNode, JourneyEdge> | null>
        >
        onNodesChange: (changes: NodeChange<JourneyNode>[]) => void
        onEdgesChange: (changes: EdgeChange<JourneyEdge>[]) => void
        onConnect: (connection: Connection) => Promise<void>
        isValidConnection: (connection: Connection | JourneyEdge) => boolean
        onNodeDoubleClick: (_: unknown, node: JourneyNode) => void
        onDrop: (event: React.DragEvent) => Promise<void>
        onPaneClick: () => void
        onElementsDelete: (hasDeletedElements: boolean) => void
        onDuplicateSelected: () => void
        onUpdateEditNode: (partial: Partial<JourneyNode["data"]>) => void
        onDeleteNode: (id: string) => void
    }
}

export function useJourneyEditorGraph({
    isEditable,
    stepsLoaded,
    followingUserId,
    replayUserId,
}: UseJourneyEditorGraphParams): JourneyEditorGraph {
    const { t } = useTranslation()
    const wrapperRef = useRef<HTMLDivElement>(null)
    const [flowInstance, setFlowInstance] = useState<ReactFlowInstance<
        JourneyNode,
        JourneyEdge
    > | null>(null)
    const [hasUnsavedChanges, setHasUnsavedChanges] = useState(false)

    const [nodes, setNodes, applyNodesChange] = useNodesState<JourneyNode>([])
    const [edges, setEdges, applyEdgesChange] = useEdgesState<JourneyEdge>([])

    const markDirty = useCallback(() => setHasUnsavedChanges(true), [])

    const { editNode, selected, updateEditNode, deleteNode, updateNodes } = useStepEditing(
        nodes,
        setNodes,
        setEdges,
        markDirty,
    )

    const selectedCount = selected.length

    const resetRuntimeState = useCallback(() => {
        setNodes((nds) =>
            nds.map((n) => ({
                ...n,
                data: { ...n.data, active: false, visited: false },
            })),
        )
        setEdges((eds) =>
            eds.map((e) => ({
                ...e,
                animated: false,
                style: { ...e.style, stroke: "#b1b1b7" },
            })),
        )
    }, [setNodes, setEdges])

    const onUserEnteredNode = useCallback(
        (nodeId: string) => {
            setNodes((prevNodes) =>
                prevNodes.map((node) => {
                    const isBecomingActive = node.id === nodeId
                    const wasActive = node.data.active

                    return {
                        ...node,
                        data: {
                            ...node.data,
                            visited: node.data.visited || wasActive,
                            active: isBecomingActive,
                        },
                    }
                }),
            )

            setEdges((prevEdges) =>
                prevEdges.map((edge) => {
                    const isNextLine = edge.source === nodeId

                    return {
                        ...edge,
                        animated: isNextLine,
                        style: {
                            ...edge.style,
                            stroke: isNextLine
                                ? "#f97316"
                                : edge.style?.stroke === "#22c55e" || edge.source === nodeId
                                  ? "#22c55e"
                                  : "#b1b1b7",
                        },
                    }
                }),
            )
        },
        [setNodes, setEdges],
    )

    const onStepExecuted = useCallback(
        (nodeId: string) => {
            setNodes((prevNodes) =>
                prevNodes.map((node) => ({
                    ...node,
                    data: {
                        ...node.data,
                        visited: node.data.visited || node.id === nodeId,
                        active: node.id === nodeId ? false : node.data.active,
                    },
                })),
            )
        },
        [setNodes],
    )

    useEffect(() => {
        setEdges((eds) =>
            eds.map((edge) => {
                const sourceNode = nodes.find((n) => n.id === edge.source)
                const targetNode = nodes.find((n) => n.id === edge.target)

                const isOrange = sourceNode?.data.active
                const isGreen =
                    sourceNode?.data.visited &&
                    (targetNode?.data.visited || targetNode?.data.active)

                return {
                    ...edge,
                    animated: isOrange,
                    style: {
                        ...edge.style,
                        stroke: isOrange ? "#f97316" : isGreen ? "#22c55e" : "#b1b1b7",
                    },
                }
            }),
        )
    }, [nodes, setEdges])

    useEffect(() => {
        const handlesByNode = new Map<string, string[]>()
        for (const edge of edges) {
            if (!edge.sourceHandle) continue
            const handles = handlesByNode.get(edge.source) ?? []
            handles.push(edge.sourceHandle)
            handlesByNode.set(edge.source, handles)
        }

        setNodes((nds) => {
            let changed = false
            const nextNodes = nds.map((node) => {
                const nextHandles = handlesByNode.get(node.id) ?? []
                const currentHandles = node.data.connectedSourceHandles ?? []
                if (
                    currentHandles.length === nextHandles.length &&
                    currentHandles.every((handle, index) => handle === nextHandles[index])
                ) {
                    return node
                }
                changed = true
                return {
                    ...node,
                    data: { ...node.data, connectedSourceHandles: nextHandles },
                }
            })
            return changed ? nextNodes : nds
        })
    }, [edges, setNodes])

    useEffect(() => {
        setNodes((nds) =>
            nds.map((n) => ({
                ...n,
                data: { ...n.data, hasUnsavedChanges },
            })),
        )
    }, [hasUnsavedChanges, setNodes])

    const { pushHistory, clearHistory } = useKeyboardShortcuts({
        nodes,
        edges,
        setNodes,
        setEdges,
        onNodesUpdated: markDirty,
        enabled: isEditable,
    })

    const onNodesChange = useCallback(
        (changes: NodeChange<JourneyNode>[]) => {
            const hasGraphChange = changes.some(
                (change) => change.type !== "select" && change.type !== "dimensions",
            )
            const hasDimensionChange = changes.some(
                (change) => change.type === "dimensions" && change.resizing === false,
            )
            if (hasGraphChange || hasDimensionChange) markDirty()
            applyNodesChange(changes)
        },
        [applyNodesChange, markDirty],
    )

    const onEdgesChange = useCallback(
        (changes: EdgeChange<JourneyEdge>[]) => {
            if (changes.some((change) => change.type !== "select")) markDirty()
            applyEdgesChange(changes)
        },
        [applyEdgesChange, markDirty],
    )

    const editorActions = useMemo(
        () => ({
            onEdgeDataChange: (edgeId: string, next: Record<string, unknown>) => {
                pushHistory()
                markDirty()
                setEdges((eds) =>
                    eds.map((edge) => (edge.id === edgeId ? { ...edge, data: next } : edge)),
                )
            },
            onEdgeDelete: () => {
                markDirty()
            },
        }),
        [pushHistory, setEdges, markDirty],
    )

    const { onConnect, onDrop, onNodeDoubleClick } = useJourneyFlowHandlers(
        nodes,
        edges,
        setNodes,
        setEdges,
        flowInstance,
        markDirty,
        pushHistory,
    )

    const isValidConnection = useCallback(
        (connection: Connection | JourneyEdge) =>
            isValidJourneyConnection(connection, nodes, edges),
        [nodes, edges],
    )

    const onPaneClick = useCallback(() => {
        if (editNode) setNodes(nodes.map((n) => ({ ...n, data: { ...n.data, editing: false } })))
    }, [editNode, nodes, setNodes])

    const onElementsDelete = useCallback(
        (hasDeletedElements: boolean) => {
            if (hasDeletedElements) {
                pushHistory()
                markDirty()
            }
        },
        [pushHistory, markDirty],
    )

    const onDuplicateSelected = useCallback(() => {
        pushHistory({ nodes, edges })
        const { nodeCopies, edgeCopies } = cloneNodes(
            edges,
            nodes.filter((n) => n.selected),
        )
        updateNodes([
            ...nodes.map((n) => ({
                ...n,
                selected: false,
            })),
            ...nodeCopies,
        ])
        setEdges([
            ...edges.map((e) => ({
                ...e,
                selected: false,
            })),
            ...edgeCopies,
        ])
    }, [edges, nodes, setEdges, updateNodes, pushHistory])

    const showConnectHint = useMemo(() => {
        if (!isEditable || !stepsLoaded) return false
        if (followingUserId || replayUserId) return false
        if (nodes.length === 0) return false
        if (nodes.length === 1) return true
        const hasIncoming = new Set<string>()
        for (const e of edges) hasIncoming.add(e.target)
        for (const n of nodes) {
            const type = n.data.type ? getStepType(n.data.type) : null
            if (!type || type.hideTopHandle) continue
            if (!hasIncoming.has(n.id)) return true
        }
        return false
    }, [isEditable, stepsLoaded, followingUserId, replayUserId, nodes, edges])

    const hintsValue = useMemo<JourneyHintsValue>(() => ({ showConnectHint }), [showConnectHint])

    const createEntranceFromSetup = useCallback(
        async (trigger: EntranceTrigger) => {
            const type = getStepType("entrance")
            const defaultData = type?.newData ? await type.newData() : {}

            markDirty()
            setNodes([
                {
                    id: createUuid(),
                    position: { x: 0, y: 0 },
                    type: "step",
                    data: {
                        type: "entrance",
                        name: t("entrance"),
                        data: { ...defaultData, trigger },
                        editing: true,
                    },
                },
            ])
        },
        [markDirty, setNodes, t],
    )

    return {
        wrapperRef,
        nodes,
        edges,
        setNodes,
        setEdges,
        editNode,
        selectedCount,
        hasUnsavedChanges,
        setHasUnsavedChanges,
        clearHistory,
        editorActions,
        hintsValue,
        createEntranceFromSetup,
        onUserEnteredNode,
        onStepExecuted,
        resetRuntimeState,
        handlers: {
            onInit: setFlowInstance,
            onNodesChange,
            onEdgesChange,
            onConnect,
            isValidConnection,
            onNodeDoubleClick,
            onDrop,
            onPaneClick,
            onElementsDelete,
            onDuplicateSelected,
            onUpdateEditNode: updateEditNode,
            onDeleteNode: deleteNode,
        },
    }
}
