import { useCallback, useContext, useEffect, useRef, useState } from "react"
import { useNavigate } from "react-router"
import type { ReactFlowInstance } from "reactflow"
import ReactFlow, {
    Background,
    Controls,
    MiniMap,
    Panel,
    useEdgesState,
    useNodesState,
} from "reactflow"
import { JourneyContext, ProjectContext } from "../../../contexts"
import { cn, createComparator } from "../../../utils"
import * as journeySteps from "../steps/index"
import api from "../../../api"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { ScrollArea } from "@/components/ui/scroll-area"
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { JourneyForm } from "../JourneyForm"
import { useTranslation } from "react-i18next"
import { JourneyStepUsers } from "../JourneyStepUsers"
import type { UUID } from "@/types/common"
import { UserSelectionModal } from "../JourneyUserSelectionModal"
import { ChevronLeft, GripVertical } from "lucide-react"

import { JourneyStepNode } from "../components/JourneyStepNode"
import { JourneyStepEdge } from "../components/JourneyStepEdge"
import { DATA_FORMAT, stepCategoryColors } from "../hooks/JourneyEditor.constants"
import type { JourneyNodeData } from "./JourneyEditor.types"
import { cloneNodes, getStepType, stepsToNodes } from "./JourneyEditor.utils"

import "./JourneyEditor.css"
import "reactflow/dist/style.css"
import { useJourneyPersistence } from "../hooks/useJourneyPersistence"
import { useJourneyFlowHandlers } from "../hooks/useJourneyFlowHandlers"
import { useUserSelection } from "../hooks/useUserSelection"
import { useStepEditing } from "../hooks/useStepEditing"
import { JourneyStepSidebar } from "../components/JourneyStepSidebar"
import { useKeyboardShortcuts } from "../hooks/useKeyboardShortcuts"

const nodeTypes = { step: JourneyStepNode }
const edgeTypes = { step: JourneyStepEdge }

export default function JourneyEditor() {
    const navigate = useNavigate()
    const { t } = useTranslation()
    const wrapper = useRef<HTMLDivElement>(null)
    const [project] = useContext(ProjectContext)
    const [journey, setJourney] = useContext(JourneyContext)
    const [flowInstance, setFlowInstance] = useState<null | ReactFlowInstance>(null)

    const [nodes, setNodes, onNodesChange] = useNodesState<JourneyNodeData>([])
    const [edges, setEdges, onEdgesChange] = useEdgesState([])
    const [viewUsersStep, setViewUsersStep] = useState<null | {
        stepId: UUID
        stepType: string
        stepName: string
    }>(null)
    const [editOpen, setEditOpen] = useState(false)
    const [isUserModalOpen, setIsUserModalOpen] = useState(false)

    const [stepsLoaded, setStepsLoaded] = useState(false)

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

    const {
        saving,
        publishing,
        hasUnsavedChanges,
        setHasUnsavedChanges,
        saveSteps,
        publishJourney,
    } = useJourneyPersistence(project, journey, setJourney, setNodes, setEdges)

    const { editNode, selected, updateEditNode, deleteNode, updateNodes } = useStepEditing(
        nodes,
        setNodes,
        () => setHasUnsavedChanges(true),
    )

    const { users, triggerUser, skipDelayForActiveUser, searchParams, followUser, STORAGE_KEY } =
        useUserSelection(project.id, journey.id, isUserModalOpen, onUserEnteredNode)

    useEffect(() => {
        if (!stepsLoaded) return

        const userId =
            searchParams.get("follow") ??
            sessionStorage.getItem(STORAGE_KEY(project.id, journey.id))
        if (!userId) return

        const restore = async () => {
            try {
                const states = await api.journeys.users.getState(project.id, journey.id, userId)
                for (const state of states) {
                    onUserEnteredNode(state.external_step_id)
                }
            } catch (e) {
                console.error("Failed to restore state:", e)
            } finally {
                followUser(userId)
            }
        }

        void restore()
        // stepsLoaded is the trigger — adding other deps would re-run this on every render cycle
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [stepsLoaded])

    useEffect(() => {
        const load = async () => {
            const steps = await api.journeys.steps.get(project.id, journey.id)
            const { edges, nodes } = stepsToNodes(steps, {
                setViewUsersStep,
                skipDelay: skipDelayForActiveUser,
            })
            setNodes(nodes)
            setEdges(edges)
            setStepsLoaded(true)
        }
        void load()
    }, [project.id, journey.id, setNodes, setEdges, skipDelayForActiveUser])

    const onPaneClick = useCallback(() => {
        if (editNode) setNodes(nodes.map((n) => ({ ...n, data: { ...n.data, editing: false } })))
    }, [editNode, nodes, setNodes])

    const { pushHistory } = useKeyboardShortcuts({
        nodes,
        edges,
        setNodes,
        setEdges,
        onNodesUpdated: () => setHasUnsavedChanges(true),
        enabled: journey.status !== "archived",
    })

    const { onConnect, onDrop, onNodeDoubleClick } = useJourneyFlowHandlers(
        nodes,
        setNodes,
        setEdges,
        flowInstance,
        wrapper,
        () => setHasUnsavedChanges(true),
        pushHistory,
    )

    return (
        <div className="flex flex-col h-screen overflow-hidden">
            {/* Header toolbar */}
            <div className="flex items-center gap-3 px-4 py-2.5 border-b bg-background shrink-0">
                <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => navigate("../journeys")}
                    className="gap-1 text-muted-foreground hover:text-foreground"
                >
                    <ChevronLeft className="h-4 w-4" />
                    {t("journeys")}
                </Button>

                <div className="h-4 w-px bg-border" />

                <h1 className="flex-1 text-base font-semibold truncate">{journey.name}</h1>

                <div className="flex items-center gap-2 shrink-0">
                    {journey.status === "archived" ? (
                        <Badge variant="destructive">{t("journey_archived")}</Badge>
                    ) : (
                        <>
                            {journey.status === "published" && (
                                <Badge
                                    variant="secondary"
                                    className="bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400 border-transparent"
                                >
                                    {t("published")}
                                </Badge>
                            )}
                            {hasUnsavedChanges && (
                                <span className="text-xs text-amber-600 dark:text-amber-500">
                                    {t("unsaved_changes", "Unsaved changes")}
                                </span>
                            )}
                            <Button variant="ghost" size="sm" onClick={() => setEditOpen(true)}>
                                {t("edit_details")}
                            </Button>
                            <Button
                                variant="outline"
                                size="sm"
                                onClick={() => saveSteps(nodes, edges)}
                                isLoading={saving}
                            >
                                {t("journey_draft_save")}
                            </Button>
                            <Button
                                size="sm"
                                onClick={() => publishJourney(nodes, edges)}
                                isLoading={publishing}
                            >
                                {t("publish")}
                            </Button>
                        </>
                    )}
                </div>
            </div>

            {/* Main content: canvas + sidebar */}
            <div className={cn("flex flex-1 min-h-0", editNode && "journey-editing")}>
                <div className="flex-1" ref={wrapper}>
                    <ReactFlow
                        nodeTypes={nodeTypes}
                        edgeTypes={edgeTypes}
                        nodes={nodes}
                        edges={edges}
                        onNodesChange={onNodesChange}
                        onEdgesChange={onEdgesChange}
                        onConnect={onConnect}
                        onInit={setFlowInstance}
                        onNodeDoubleClick={onNodeDoubleClick}
                        onDrop={onDrop}
                        onDragOver={(e) => {
                            e.preventDefault()
                            e.dataTransfer.dropEffect = "move"
                        }}
                        onClick={onPaneClick}
                        nodesDraggable={journey.status !== "archived"}
                        nodesConnectable={journey.status !== "archived"}
                        deleteKeyCode={["Backspace", "Delete"]}
                        panOnScroll
                        selectNodesOnDrag
                        fitView
                        fitViewOptions={{ padding: 0.3, maxZoom: 1 }}
                    >
                        <Background className="!bg-muted/30" />
                        {!editNode && (
                            <>
                                <Controls showInteractive={journey.status !== "archived"} />
                                <MiniMap
                                    nodeClassName={(n) =>
                                        `journey-minimap ${getStepType(n.data.type)?.category ?? "unknown"}`
                                    }
                                />
                                {journey.status !== "archived" && (
                                    <Panel position="top-left">
                                        {selected.length ? (
                                            <Button
                                                onClick={() => {
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
                                                }}
                                                size="sm"
                                                variant="outline"
                                                className="shadow-sm"
                                            >
                                                {`Duplicate Selected Steps (${selected.length})`}
                                            </Button>
                                        ) : (
                                            <span className="text-xs text-muted-foreground bg-background/80 backdrop-blur-sm px-3 py-1.5 rounded-md border shadow-sm">
                                                Shift+Drag to Multi Select
                                            </span>
                                        )}
                                    </Panel>
                                )}
                            </>
                        )}
                    </ReactFlow>
                </div>

                {/* Right sidebar */}
                {journey.status !== "archived" && (
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
                                hasUnsavedChanges={hasUnsavedChanges}
                                onUpdate={updateEditNode}
                                onDelete={deleteNode}
                                onOpenUserModal={() => setIsUserModalOpen(true)}
                                onViewUsers={(stepId, stepType, stepName) =>
                                    setViewUsersStep({ stepId, stepType, stepName })
                                }
                            />
                        ) : (
                            <>
                                <div className="px-4 py-3 border-b">
                                    <h2 className="text-sm font-medium text-foreground">
                                        {t("components")}
                                    </h2>
                                    <p className="text-sm text-muted-foreground mt-1">
                                        {t("drag_to_canvas", "Drag components to the canvas")}
                                    </p>
                                </div>
                                <ScrollArea className="flex-1">
                                    <div className="p-4 space-y-1.5">
                                        {Object.entries(journeySteps)
                                            .sort(createComparator((x) => x[1].category))
                                            .map(([key, type]) => (
                                                <div
                                                    key={key}
                                                    className="group flex items-start gap-3 rounded-lg border p-3 cursor-grab active:cursor-grabbing hover:bg-muted/50 transition-colors"
                                                    draggable
                                                    onDragStart={(event) => {
                                                        const rect = (
                                                            event.target as HTMLDivElement
                                                        ).getBoundingClientRect()
                                                        event.dataTransfer.setData(
                                                            DATA_FORMAT,
                                                            JSON.stringify({
                                                                type: key,
                                                                x: event.clientX - rect.left,
                                                                y: event.clientY - rect.top,
                                                            }),
                                                        )
                                                    }}
                                                >
                                                    <div
                                                        className={cn(
                                                            "flex h-8 w-8 shrink-0 items-center justify-center rounded-md [&_svg]:h-4 [&_svg]:w-4",
                                                            stepCategoryColors[type.category] ??
                                                                "bg-muted text-muted-foreground",
                                                        )}
                                                    >
                                                        {type.icon}
                                                    </div>
                                                    <div className="flex-1 min-w-0">
                                                        <p className="text-sm font-medium leading-none">
                                                            {t(type.name)}
                                                        </p>
                                                        <p className="text-xs text-muted-foreground mt-1 leading-snug">
                                                            {t(type.description)}
                                                        </p>
                                                    </div>
                                                    <GripVertical className="h-4 w-4 text-muted-foreground/40 shrink-0 mt-0.5 opacity-0 group-hover:opacity-100 transition-opacity" />
                                                </div>
                                            ))}
                                    </div>
                                </ScrollArea>
                            </>
                        )}
                    </div>
                )}
            </div>

            <UserSelectionModal
                users={users}
                isOpen={isUserModalOpen}
                onClose={() => setIsUserModalOpen(false)}
                onSelect={(u) => {
                    setIsUserModalOpen(false)
                    if (editNode?.id) triggerUser(editNode.id, u.id)
                    onUserEnteredNode(editNode?.id ?? "")
                }}
            />

            <Dialog open={editOpen} onOpenChange={setEditOpen}>
                <DialogContent>
                    <DialogHeader>
                        <DialogTitle>{t("edit_journey_details")}</DialogTitle>
                    </DialogHeader>
                    <JourneyForm
                        journey={journey}
                        onSaved={async (j) => {
                            setEditOpen(false)
                            setJourney(j)
                        }}
                    />
                </DialogContent>
            </Dialog>

            {!!viewUsersStep && (
                <JourneyStepUsers
                    open={!!viewUsersStep}
                    onClose={(open) => {
                        if (!open) setViewUsersStep(null)
                    }}
                    stepType={viewUsersStep.stepType}
                    stepId={viewUsersStep.stepId}
                    stepName={viewUsersStep.stepName}
                />
            )}
        </div>
    )
}
