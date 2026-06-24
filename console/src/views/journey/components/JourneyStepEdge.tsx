import { memo, useCallback, useContext, useMemo, createElement, useState } from "react"
import type { EdgeProps } from "@xyflow/react"
import {
    getBezierPath,
    EdgeLabelRenderer,
    EdgeToolbar,
    useNodes,
    useEdges,
    useReactFlow,
} from "@xyflow/react"
import { useTranslation } from "react-i18next"
import { Trash2 } from "lucide-react"
import { ProjectContext, JourneyContext } from "@/contexts"
import { getStepType } from "../editor/JourneyEditor.utils"
import type { JourneyEdge, JourneyNode } from "../editor/JourneyEditor.types"
import { useJourneyEditorActions } from "../editor/JourneyEditorActions"

const EDGE_TOOLBAR_Z_INDEX = 1003

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
    }: EdgeProps<JourneyEdge>) => {
        const { t } = useTranslation()
        const [project] = useContext(ProjectContext)
        const [journey] = useContext(JourneyContext)
        const { onEdgeDataChange, onEdgeDelete } = useJourneyEditorActions()
        const nodes = useNodes<JourneyNode>()
        const edges = useEdges<JourneyEdge>()

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
        const { deleteElements } = useReactFlow<JourneyNode, JourneyEdge>()

        const onChangeData = useCallback(
            (next: Record<string, unknown>) => onEdgeDataChange(id, next),
            [id, onEdgeDataChange],
        )

        const onDelete = useCallback(() => {
            onEdgeDelete(id)
            deleteElements({ edges: [{ id }] }).catch((err) => {
                console.error("Failed to delete edge:", err)
            })
        }, [deleteElements, id, onEdgeDelete])

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
                <EdgeToolbar
                    edgeId={id}
                    x={labelX}
                    y={labelY + (hasEditEdge ? -28 : 0)}
                    isVisible={showDelete}
                    // JourneyEditor.css lifts nodes to z-index 1002 so live
                    // connection handles stay above the connection-line SVG.
                    // Keep the delete affordance above that custom node layer.
                    style={{ zIndex: EDGE_TOOLBAR_Z_INDEX }}
                >
                    <button
                        type="button"
                        onClick={(e) => {
                            e.stopPropagation()
                            onDelete()
                        }}
                        onMouseEnter={() => setHovered(true)}
                        onMouseLeave={() => setHovered(false)}
                        aria-label={t("delete_connection", "Delete connection")}
                        className="nodrag nopan pointer-events-auto flex items-center justify-center w-6 h-6 rounded-full bg-destructive text-white shadow-md hover:bg-red-700 transition-colors cursor-pointer"
                    >
                        <Trash2 className="h-3 w-3" />
                    </button>
                </EdgeToolbar>
            </>
        )
    },
)
