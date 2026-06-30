import { Background, Controls, MarkerType, MiniMap, ReactFlow } from "@xyflow/react"
import { Info } from "lucide-react"
import { useTranslation } from "react-i18next"

import { cn } from "@/utils"
import type { Action } from "@/oapi/client"
import type { Journey, Project } from "@/types"
import { JourneyConnectionLine } from "./JourneyConnectionLine"
import { JourneyLibrarySidebar } from "./JourneyLibrarySidebar"
import { FollowingPanel, ReplayPanel } from "./JourneyRuntimePanels"
import { JourneySelectionPanel } from "./JourneySelectionPanel"
import { JourneyStepEdge } from "./JourneyStepEdge"
import { JourneyStepNode } from "./JourneyStepNode"
import { JourneyStepSidebar } from "./JourneyStepSidebar"
import { getStepType } from "../editor/JourneyEditor.utils"
import type { JourneyEditorGraph } from "../hooks/useJourneyEditorGraph"
import type { JourneyEdge, JourneyNode } from "../editor/JourneyEditor.types"

const nodeTypes = { step: JourneyStepNode }
const edgeTypes = { step: JourneyStepEdge }

type JourneyCanvasGraph = Pick<
    JourneyEditorGraph,
    "nodes" | "edges" | "editNode" | "selectedCount" | "wrapperRef" | "handlers"
>

interface JourneyCanvasProps {
    project: Project
    journey: Journey
    actions: Action[] | null
    graph: JourneyCanvasGraph
    sidebar: {
        tab: "components" | "actions"
        onTabChange: (tab: "components" | "actions") => void
        onSaveDraft: () => Promise<void>
    }
    runtime: {
        followingUserId: string | null
        replayUserId: string | null
        onStopFollowing: () => void
        onCancelExecution: (userId: string) => void
        onDismissReplay: () => void
    }
    isArchived: boolean
    isEditable: boolean
    isMobile: boolean
}

export function JourneyCanvas({
    project,
    journey,
    actions,
    graph,
    sidebar,
    runtime,
    isArchived,
    isEditable,
    isMobile,
}: JourneyCanvasProps) {
    const { t } = useTranslation()
    const { nodes, edges, editNode, handlers } = graph

    return (
        <div className={cn("flex flex-1 min-h-0", editNode && "journey-editing")}>
            <div className="flex-1 relative flex flex-col" ref={graph.wrapperRef}>
                {isMobile && !isArchived && (
                    <div className="flex items-center justify-center gap-2 bg-muted/90 border-b px-4 py-2.5 text-sm text-muted-foreground">
                        <Info className="h-4 w-4 shrink-0" />
                        {t("journey_view_only_mobile", "View only, edit on desktop")}
                    </div>
                )}
                <ReactFlow<JourneyNode, JourneyEdge>
                    nodeTypes={nodeTypes}
                    edgeTypes={edgeTypes}
                    nodes={nodes}
                    edges={edges}
                    onNodesChange={handlers.onNodesChange}
                    onEdgesChange={handlers.onEdgesChange}
                    onConnect={
                        isEditable ? (connection) => void handlers.onConnect(connection) : undefined
                    }
                    isValidConnection={isEditable ? handlers.isValidConnection : undefined}
                    connectionLineComponent={JourneyConnectionLine}
                    connectionRadius={150}
                    defaultEdgeOptions={{ markerEnd: { type: MarkerType.ArrowClosed } }}
                    onInit={handlers.onInit}
                    onNodeDoubleClick={isEditable ? handlers.onNodeDoubleClick : undefined}
                    onDrop={isEditable ? (event) => void handlers.onDrop(event) : undefined}
                    onDragOver={
                        isEditable
                            ? (e) => {
                                  e.preventDefault()
                                  e.dataTransfer.dropEffect = "move"
                              }
                            : undefined
                    }
                    onPaneClick={handlers.onPaneClick}
                    nodesDraggable={isEditable}
                    nodesConnectable={isEditable}
                    elementsSelectable={isEditable}
                    deleteKeyCode={isEditable ? ["Backspace", "Delete"] : []}
                    onDelete={
                        isEditable
                            ? ({ nodes: deletedNodes, edges: deletedEdges }) =>
                                  handlers.onElementsDelete(
                                      !!deletedNodes.length || !!deletedEdges.length,
                                  )
                            : undefined
                    }
                    panOnScroll
                    selectNodesOnDrag={isEditable}
                    fitView
                    minZoom={0.1}
                    fitViewOptions={{ padding: 0.3, maxZoom: 1 }}
                >
                    <Background className="!bg-muted/30" />
                    {!editNode && (
                        <>
                            <Controls showInteractive={isEditable} />
                            {!isMobile && (
                                <MiniMap
                                    nodeClassName={(n) => {
                                        const type =
                                            typeof n.data?.type === "string" ? n.data.type : ""
                                        return `journey-minimap ${getStepType(type)?.category ?? "unknown"}`
                                    }}
                                />
                            )}

                            <FollowingPanel
                                userId={runtime.followingUserId}
                                onStopFollowing={runtime.onStopFollowing}
                                onCancelExecution={runtime.onCancelExecution}
                            />
                            <ReplayPanel
                                visible={!!runtime.replayUserId && !runtime.followingUserId}
                                onDismiss={runtime.onDismissReplay}
                            />

                            {isEditable && (
                                <JourneySelectionPanel
                                    selectedCount={graph.selectedCount}
                                    onDuplicateSelected={handlers.onDuplicateSelected}
                                />
                            )}
                        </>
                    )}
                </ReactFlow>
            </div>

            {isEditable && (
                <div
                    className={cn(
                        "border-l bg-background shrink-0 flex flex-col",
                        editNode ? "w-[40%]" : "w-1/4",
                    )}
                >
                    {editNode ? (
                        <JourneyStepSidebar
                            editNode={editNode}
                            nodes={nodes}
                            project={project}
                            journey={journey}
                            onUpdate={handlers.onUpdateEditNode}
                            onDelete={handlers.onDeleteNode}
                            onSaveDraft={sidebar.onSaveDraft}
                        />
                    ) : (
                        <JourneyLibrarySidebar
                            actions={actions}
                            sidebarTab={sidebar.tab}
                            onSidebarTabChange={sidebar.onTabChange}
                        />
                    )}
                </div>
            )}
        </div>
    )
}
