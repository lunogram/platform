import { memo, useCallback, useContext, useMemo, createElement, useState } from "react"
import type { EdgeProps } from "reactflow"
import { getBezierPath, EdgeLabelRenderer, useNodes, useEdges, useReactFlow } from "reactflow"
import { Trash2 } from "lucide-react"
import { ProjectContext, JourneyContext } from "@/contexts"
import { getStepType } from "../editor/JourneyEditor.utils"
import type { JourneyNode } from "../editor/JourneyEditor.types"

import "reactflow/dist/style.css"

export const JourneyStepEdge = memo(
    ({
        id,
        sourceX,
        sourceY,
        targetX,
        targetY,
        sourcePosition,
        targetPosition,
        source,
        sourceHandleId,
        targetHandleId,
        data = {},
        style = {},
        selected,
        markerEnd,
    }: EdgeProps) => {
        const [project] = useContext(ProjectContext)
        const [journey] = useContext(JourneyContext)
        const nodes = useNodes() as JourneyNode[]
        const edges = useEdges()

        const [hovered, setHovered] = useState(false)

        const isEditable = journey.status !== "archived"
        const showDelete = isEditable && (hovered || selected)

        const siblingData = useMemo(
            () =>
                edges
                    .filter(
                        (e) =>
                            e.sourceHandle === sourceHandleId && e.targetHandle !== targetHandleId,
                    )
                    .map((e) => e.data ?? {}),
            [edges, sourceHandleId, targetHandleId],
        )

        const [edgePath, labelX, labelY] = getBezierPath({
            sourceX,
            sourceY,
            sourcePosition,
            targetX,
            targetY,
            targetPosition,
        })
        const { setEdges } = useReactFlow()

        const onChangeData = useCallback(
            (data: Record<string, unknown>) =>
                setEdges((edges) => edges.map((e) => (e.id === id ? { ...e, data } : e))),
            [id, setEdges],
        )

        const onDelete = useCallback(
            () => setEdges((edges) => edges.filter((e) => e.id !== id)),
            [id, setEdges],
        )

        const sourceNode = nodes.find((n) => n.id === source)
        const sourceType = sourceNode?.data?.type ? getStepType(sourceNode.data.type) : null
        const hasEditEdge = !!(sourceNode && sourceType?.EditEdge)

        return (
            <>
                {/* Invisible wider path for easier mouse targeting */}
                <path
                    d={edgePath}
                    fill="none"
                    stroke="transparent"
                    strokeWidth={20}
                    onMouseEnter={() => setHovered(true)}
                    onMouseLeave={() => setHovered(false)}
                    className="react-flow__edge-interaction"
                />
                {/* Visible edge path — pointer-events disabled so the wider
                    interaction path underneath handles all mouse events */}
                <path
                    id={id}
                    className="react-flow__edge-path"
                    style={{ ...style, pointerEvents: "none" }}
                    d={edgePath}
                    markerEnd={markerEnd}
                />
                <EdgeLabelRenderer>
                    {/* Delete button — offset when an edge editor label is present */}
                    {showDelete && (
                        <button
                            type="button"
                            onClick={(e) => {
                                e.stopPropagation()
                                onDelete()
                            }}
                            onMouseEnter={() => setHovered(true)}
                            onMouseLeave={() => setHovered(false)}
                            style={{
                                position: "absolute",
                                transform: `translate(-50%, -50%) translate(${labelX}px,${labelY + (hasEditEdge ? -28 : 0)}px)`,
                            }}
                            className="pointer-events-auto flex items-center justify-center w-6 h-6 rounded-full bg-destructive text-white shadow-md hover:bg-red-700 transition-colors cursor-pointer"
                        >
                            <Trash2 className="h-3 w-3" />
                        </button>
                    )}
                    {/* Step-type edge editor (e.g. condition labels) */}
                    {hasEditEdge && (
                        <div
                            style={{
                                position: "absolute",
                                transform: `translate(-50%, -50%) translate(${labelX}px,${labelY}px)`,
                            }}
                            className="nodrag nopan bg-background border rounded-lg p-2.5 pointer-events-auto"
                        >
                            {createElement(sourceType!.EditEdge!, {
                                value: data,
                                onChange: onChangeData,
                                stepData: sourceNode!.data.data,
                                siblingData,
                                journey,
                                project,
                            })}
                        </div>
                    )}
                </EdgeLabelRenderer>
            </>
        )
    },
)
