import { useCallback, useContext, useEffect, useMemo, useRef, useState } from "react"
import { useNavigate, useSearchParams as useRouterSearchParams } from "react-router"
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
import oapiClient, { type Action } from "@/oapi/client"
import { useResolver } from "@/hooks"
import { useIsMobile } from "@/hooks/use-mobile"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { NavTabs } from "@/components/ui/nav-tabs"
import { ScrollArea } from "@/components/ui/scroll-area"
import { InlineEdit } from "@/components/ui/inline-edit"
import { useTranslation } from "react-i18next"
import { JourneyStepUsers } from "../JourneyStepUsers"
import type { UUID } from "@/types/common"
import { UserSelectionModal } from "../JourneyUserSelectionModal"
import {
    ChevronLeft,
    GripVertical,
    Info,
    Zap,
    Webhook,
    Blocks,
    SquareFunction,
    Eye,
    EyeOff,
    XCircle,
    PenLine,
} from "lucide-react"

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
import { JourneyVariableProvider } from "../JourneyVariableContext"

const nodeTypes = { step: JourneyStepNode }
const edgeTypes = { step: JourneyStepEdge }

export default function JourneyEditor() {
    const navigate = useNavigate()
    const { t } = useTranslation()
    const wrapper = useRef<HTMLDivElement>(null)
    const [project] = useContext(ProjectContext)
    const [journey, setJourney] = useContext(JourneyContext)
    const [flowInstance, setFlowInstance] = useState<null | ReactFlowInstance>(null)
    const [routerSearchParams] = useRouterSearchParams()
    const entranceId = routerSearchParams.get("entrance")
    const replayUserId = routerSearchParams.get("user")

    const [nodes, setNodes, onNodesChange] = useNodesState<JourneyNodeData>([])
    const [edges, setEdges, onEdgesChange] = useEdgesState([])
    const [viewUsersStep, setViewUsersStep] = useState<null | {
        stepId: UUID
        stepType: string
        stepName: string
    }>(null)
    const [userModalEntranceId, setUserModalEntranceId] = useState<string | null>(null)
    const [sidebarTab, setSidebarTab] = useState<"components" | "actions">("components")
    const isMobile = useIsMobile()

    const [stepsLoaded, setStepsLoaded] = useState(false)

    // Fetch project actions for the sidebar
    const [actions] = useResolver(
        useCallback(async () => {
            const { data } = await oapiClient.GET("/api/admin/projects/{projectID}/actions", {
                params: {
                    path: { projectID: project.id },
                    query: { limit: 100 },
                },
            })
            return data?.results ?? []
        }, [project.id]),
    )

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

    const resetFollowingState = useCallback(() => {
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

    const {
        triggerUser,
        skipDelayForActiveUser,
        searchParams,
        followUser,
        stopFollowing,
        cancelExecution,
        STORAGE_KEY,
    } = useUserSelection(
        project.id,
        journey.id,
        onUserEnteredNode,
        onStepExecuted,
        resetFollowingState,
        entranceId,
    )

    const isFollowing = !!searchParams.get("follow")
    const prevFollowingRef = useRef(isFollowing)

    useEffect(() => {
        // When we transition from following → not following, reset the
        // visual state. This covers all exit paths: stop button, cancel
        // button, exit step reached, SSE error, etc.
        if (prevFollowingRef.current && !isFollowing) {
            resetFollowingState()
        }
        prevFollowingRef.current = isFollowing
    }, [isFollowing, resetFollowingState])

    const openUserModal = useCallback((nodeId: string) => setUserModalEntranceId(nodeId), [])

    const nodeActions = useMemo(
        () => ({
            setViewUsersStep,
            skipDelay: skipDelayForActiveUser,
            openUserModal,
        }),
        [setViewUsersStep, skipDelayForActiveUser, openUserModal],
    )

    const handleSaveDraft = useCallback(async () => {
        await saveSteps(nodes, edges, nodeActions)
    }, [saveSteps, nodes, edges, nodeActions])

    useEffect(() => {
        if (!stepsLoaded) return

        const userId =
            searchParams.get("follow") ??
            sessionStorage.getItem(STORAGE_KEY(project.id, journey.id))
        if (!userId) return

        const restore = async () => {
            try {
                const states = await api.journeys.users.getState(
                    project.id,
                    journey.id,
                    userId,
                    entranceId ?? undefined,
                )
                for (const state of states) {
                    // Entrance steps complete instantly — treat them as
                    // visited even if legacy data has is_completed=false.
                    if (state.is_completed || state.step_type === "entrance") {
                        onStepExecuted(state.external_step_id)
                    } else {
                        onUserEnteredNode(state.external_step_id)
                    }
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

    // Replay: restore the path a user took through a completed journey
    // (no SSE stream — the journey has already ended).
    useEffect(() => {
        if (!stepsLoaded || !replayUserId || !entranceId) return

        const restore = async () => {
            try {
                const states = await api.journeys.users.getState(
                    project.id,
                    journey.id,
                    replayUserId,
                    entranceId,
                )
                for (const state of states) {
                    if (state.is_completed || state.step_type === "entrance") {
                        onStepExecuted(state.external_step_id)
                    } else {
                        onUserEnteredNode(state.external_step_id)
                    }
                }
            } catch (e) {
                console.error("Failed to restore completed journey state:", e)
            }
        }

        void restore()
        // stepsLoaded is the trigger — adding other deps would re-run this on every render cycle
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [stepsLoaded])

    useEffect(() => {
        if (stepsLoaded) return
        const load = async () => {
            const steps = await api.journeys.steps.get(project.id, journey.id)
            const { edges, nodes } = stepsToNodes(steps, nodeActions)
            setNodes(nodes)
            setEdges(edges)
            setStepsLoaded(true)
        }
        void load()
    }, [
        project.id,
        journey.id,
        setNodes,
        setEdges,
        stepsLoaded,
        nodeActions,
    ])

    const onPaneClick = useCallback(() => {
        if (editNode) setNodes(nodes.map((n) => ({ ...n, data: { ...n.data, editing: false } })))
    }, [editNode, nodes, setNodes])

    // Keep node data in sync with hasUnsavedChanges so the Run button can read it
    useEffect(() => {
        setNodes((nds) =>
            nds.map((n) => ({
                ...n,
                data: { ...n.data, hasUnsavedChanges },
            })),
        )
    }, [hasUnsavedChanges, setNodes])

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

    const isArchived = journey.status === "archived"
    const isEditable = !isArchived && !isMobile

    return (
        <div className="flex flex-col flex-1 h-svh min-h-0 overflow-hidden">
            {/* Header toolbar */}
            <div className="flex items-center gap-2 sm:gap-3 px-3 sm:px-4 py-2.5 sm:py-2.5 border-b bg-background shrink-0">
                <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => navigate("../journeys")}
                    className="gap-1 text-muted-foreground hover:text-foreground"
                >
                    <ChevronLeft className="h-4 w-4" />
                    <span className="hidden sm:inline">{t("journeys")}</span>
                </Button>

                <div className="h-4 w-px bg-border hidden sm:block" />

                <div className="flex-1 min-w-0">
                    <InlineEdit
                        value={journey.name}
                        onSave={async (name) => {
                            const updated = await api.journeys.update(project.id, journey.id, {
                                name,
                            })
                            setJourney(updated)
                        }}
                        required
                        triggerClassName="gap-1.5 max-w-full"
                        pencilSize="h-3.5 w-3.5"
                    >
                        <h1 className="text-sm sm:text-base font-semibold truncate">
                            {journey.name}
                        </h1>
                    </InlineEdit>
                </div>

                <div className="flex items-center gap-1.5 sm:gap-2 shrink-0">
                    {isArchived ? (
                        <Badge
                            variant="secondary"
                            className="bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400 border-transparent"
                        >
                            {t("archived")}
                        </Badge>
                    ) : (
                        <>
                            {hasUnsavedChanges ? (
                                <Badge
                                    variant="secondary"
                                    className="bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400 border-transparent gap-1"
                                >
                                    <PenLine className="h-3 w-3" />
                                    {t("editing", "Editing")}
                                </Badge>
                            ) : journey.status === "published" ? (
                                <Badge
                                    variant="secondary"
                                    className="bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400 border-transparent"
                                >
                                    {t("published")}
                                </Badge>
                            ) : (
                                <Badge
                                    variant="secondary"
                                    className="bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400 border-transparent"
                                >
                                    {t("draft")}
                                </Badge>
                            )}
                            {!isMobile &&
                                (hasUnsavedChanges ? (
                                    <Button
                                        variant="outline"
                                        size="sm"
                                        onClick={() => saveSteps(nodes, edges, nodeActions)}
                                        isLoading={saving}
                                    >
                                        <span className="hidden sm:inline">
                                            {t("journey_draft_save")}
                                        </span>
                                        <span className="sm:hidden">{t("save", "Save")}</span>
                                    </Button>
                                ) : (
                                    <Button
                                        size="sm"
                                        onClick={() => publishJourney(nodes, edges, nodeActions)}
                                        isLoading={publishing}
                                    >
                                        {t("publish")}
                                    </Button>
                                ))}
                        </>
                    )}
                </div>
            </div>

            {/* Main content: canvas + sidebar */}
            <JourneyVariableProvider nodes={nodes} edges={edges}>
                <div className={cn("flex flex-1 min-h-0", editNode && "journey-editing")}>
                    <div className="flex-1 relative flex flex-col" ref={wrapper}>
                        {isMobile && !isArchived && (
                            <div className="flex items-center justify-center gap-2 bg-muted/90 border-b px-4 py-2.5 text-sm text-muted-foreground">
                                <Info className="h-4 w-4 shrink-0" />
                                {t("journey_view_only_mobile", "View only, edit on desktop")}
                            </div>
                        )}
                        <ReactFlow
                            nodeTypes={nodeTypes}
                            edgeTypes={edgeTypes}
                            nodes={nodes}
                            edges={edges}
                            onNodesChange={onNodesChange}
                            onEdgesChange={onEdgesChange}
                            onConnect={isEditable ? onConnect : undefined}
                            onInit={setFlowInstance}
                            onNodeDoubleClick={isEditable ? onNodeDoubleClick : undefined}
                            onDrop={isEditable ? onDrop : undefined}
                            onDragOver={
                                isEditable
                                    ? (e) => {
                                          e.preventDefault()
                                          e.dataTransfer.dropEffect = "move"
                                      }
                                    : undefined
                            }
                            onPaneClick={onPaneClick}
                            nodesDraggable={isEditable}
                            nodesConnectable={isEditable}
                            elementsSelectable={isEditable}
                            deleteKeyCode={isEditable ? ["Backspace", "Delete"] : []}
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
                                            nodeClassName={(n) =>
                                                `journey-minimap ${getStepType(n.data.type)?.category ?? "unknown"}`
                                            }
                                        />
                                    )}

                                    {/* Following user control bar */}
                                    {searchParams.get("follow") && (
                                        <Panel position="top-center">
                                            <div className="flex items-center gap-2 bg-background/95 backdrop-blur-sm border rounded-lg shadow-lg px-3 py-1.5">
                                                <Eye className="h-4 w-4 text-orange-500 animate-pulse" />
                                                <span className="text-sm font-medium">
                                                    {t("following_user", "Following user")}
                                                </span>
                                                <div className="h-4 w-px bg-border" />
                                                <Button
                                                    variant="ghost"
                                                    size="sm"
                                                    onClick={stopFollowing}
                                                    className="h-7 gap-1.5 text-muted-foreground hover:text-foreground"
                                                >
                                                    <EyeOff className="h-3.5 w-3.5" />
                                                    {t("stop_following", "Stop")}
                                                </Button>
                                                <Button
                                                    variant="ghost"
                                                    size="sm"
                                                    onClick={() => {
                                                        const userId = searchParams.get("follow")
                                                        if (userId) cancelExecution(userId)
                                                    }}
                                                    className="h-7 gap-1.5 text-destructive hover:text-destructive hover:bg-destructive/10"
                                                >
                                                    <XCircle className="h-3.5 w-3.5" />
                                                    {t("cancel_execution", "Cancel")}
                                                </Button>
                                            </div>
                                        </Panel>
                                    )}

                                    {/* Replay: viewing a completed journey path */}
                                    {replayUserId && !searchParams.get("follow") && (
                                        <Panel position="top-center">
                                            <div className="flex items-center gap-2 bg-background/95 backdrop-blur-sm border rounded-lg shadow-lg px-3 py-1.5">
                                                <Eye className="h-4 w-4 text-emerald-500" />
                                                <span className="text-sm font-medium">
                                                    {t("viewing_user_path", "Viewing user path")}
                                                </span>
                                                <div className="h-4 w-px bg-border" />
                                                <Button
                                                    variant="ghost"
                                                    size="sm"
                                                    onClick={() => {
                                                        resetFollowingState()
                                                        navigate(".", { replace: true })
                                                    }}
                                                    className="h-7 gap-1.5 text-muted-foreground hover:text-foreground"
                                                >
                                                    <EyeOff className="h-3.5 w-3.5" />
                                                    {t("dismiss", "Dismiss")}
                                                </Button>
                                            </div>
                                        </Panel>
                                    )}

                                    {isEditable && (
                                        <Panel position="top-left">
                                            {selected.length ? (
                                                <div className="flex items-center gap-2 bg-background/95 backdrop-blur-sm border rounded-lg shadow-lg px-3 py-1.5">
                                                    <Button
                                                        onClick={() => {
                                                            const { nodeCopies, edgeCopies } =
                                                                cloneNodes(
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
                                                        variant="ghost"
                                                        className="h-7"
                                                    >
                                                        {`Duplicate Selected Steps (${selected.length})`}
                                                    </Button>
                                                </div>
                                            ) : (
                                                <div className="hidden sm:flex items-center bg-background/95 backdrop-blur-sm border rounded-lg shadow-lg px-3 py-1.5">
                                                    <span className="text-xs text-muted-foreground">
                                                        Shift+Drag to Multi Select
                                                    </span>
                                                </div>
                                            )}
                                        </Panel>
                                    )}
                                </>
                            )}
                        </ReactFlow>
                    </div>

                    {/* Desktop: inline right sidebar */}
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
                                    onUpdate={updateEditNode}
                                    onDelete={deleteNode}
                                    onViewUsers={(stepId, stepType, stepName) =>
                                        setViewUsersStep({ stepId, stepType, stepName })
                                    }
                                    onSaveDraft={handleSaveDraft}
                                />
                            ) : (
                                <>
                                    <NavTabs
                                        className="px-4 pt-3 border-b shrink-0"
                                        tabs={[
                                            {
                                                key: "components",
                                                label: t("components"),
                                                icon: Blocks,
                                            },
                                            {
                                                key: "actions",
                                                label: t("actions.plural", "Actions"),
                                                icon: SquareFunction,
                                                badge: actions?.length,
                                            },
                                        ]}
                                        value={sidebarTab}
                                        onChange={(key) =>
                                            setSidebarTab(key as "components" | "actions")
                                        }
                                    />
                                    <ScrollArea className="flex-1">
                                        {sidebarTab === "components" && (
                                            <div className="p-4 space-y-1.5">
                                                {Object.entries(journeySteps)
                                                    .filter(([key]) => key !== "action")
                                                    .sort(
                                                        createComparator((x) => {
                                                            const order = {
                                                                entrance: 0,
                                                                flow: 1,
                                                                delay: 2,
                                                                action: 3,
                                                                exit: 4,
                                                                info: 5,
                                                            }
                                                            return order[x[1].category] ?? 99
                                                        }),
                                                    )
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
                                                                        x:
                                                                            event.clientX -
                                                                            rect.left,
                                                                        y: event.clientY - rect.top,
                                                                    }),
                                                                )
                                                            }}
                                                        >
                                                            <div
                                                                className={cn(
                                                                    "flex h-8 w-8 shrink-0 items-center justify-center rounded-md [&_svg]:h-4 [&_svg]:w-4",
                                                                    stepCategoryColors[
                                                                        type.category
                                                                    ] ??
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
                                        )}
                                        {sidebarTab === "actions" && (
                                            <div className="p-4 space-y-2">
                                                {!actions ? (
                                                    Array.from({ length: 3 }).map((_, i) => (
                                                        <div
                                                            key={i}
                                                            className="rounded-lg border border-dashed p-3 animate-pulse"
                                                        >
                                                            <div className="flex items-center gap-3">
                                                                <div className="h-9 w-9 rounded-md bg-muted" />
                                                                <div className="flex-1 space-y-1.5">
                                                                    <div className="h-3.5 w-24 rounded bg-muted" />
                                                                    <div className="h-3 w-16 rounded bg-muted" />
                                                                </div>
                                                            </div>
                                                        </div>
                                                    ))
                                                ) : actions.length === 0 ? (
                                                    <div className="rounded-lg border border-dashed p-6 text-center">
                                                        <Zap className="h-6 w-6 text-muted-foreground/50 mx-auto mb-2" />
                                                        <p className="text-sm font-medium text-muted-foreground">
                                                            {t("no_actions_yet", "No actions yet")}
                                                        </p>
                                                        <p className="text-xs text-muted-foreground/70 mt-1">
                                                            {t(
                                                                "actions_drag_desc",
                                                                "Create an action to add it as a step",
                                                            )}
                                                        </p>
                                                    </div>
                                                ) : (
                                                    actions.map((action: Action) => {
                                                        const icon =
                                                            action.type === "webhook" ? (
                                                                <Webhook className="h-4 w-4" />
                                                            ) : (
                                                                <Zap className="h-4 w-4" />
                                                            )
                                                        return (
                                                            <div
                                                                key={action.id}
                                                                className="group flex items-center gap-3 rounded-lg border bg-card p-3 cursor-grab active:cursor-grabbing hover:bg-accent/50 hover:border-blue-300 dark:hover:border-blue-700 transition-colors"
                                                                draggable
                                                                onDragStart={(event) => {
                                                                    const rect = (
                                                                        event.target as HTMLDivElement
                                                                    ).getBoundingClientRect()
                                                                    event.dataTransfer.setData(
                                                                        DATA_FORMAT,
                                                                        JSON.stringify({
                                                                            type: "action",
                                                                            name: action.name,
                                                                            data: {
                                                                                action_id:
                                                                                    action.id,
                                                                            },
                                                                            x:
                                                                                event.clientX -
                                                                                rect.left,
                                                                            y:
                                                                                event.clientY -
                                                                                rect.top,
                                                                        }),
                                                                    )
                                                                }}
                                                            >
                                                                <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-md bg-blue-100 text-blue-600 dark:bg-blue-950 dark:text-blue-400 [&_svg]:h-4 [&_svg]:w-4">
                                                                    {icon}
                                                                </div>
                                                                <div className="flex-1 min-w-0">
                                                                    <p className="text-sm font-medium leading-none truncate">
                                                                        {action.name}
                                                                    </p>
                                                                    <p className="text-xs text-muted-foreground mt-1">
                                                                        {action.type}
                                                                    </p>
                                                                </div>
                                                                <GripVertical className="h-4 w-4 text-muted-foreground/40 shrink-0 opacity-0 group-hover:opacity-100 transition-opacity" />
                                                            </div>
                                                        )
                                                    })
                                                )}
                                            </div>
                                        )}
                                    </ScrollArea>
                                </>
                            )}
                        </div>
                    )}
                </div>
            </JourneyVariableProvider>

            <UserSelectionModal
                isOpen={!!userModalEntranceId}
                onClose={() => setUserModalEntranceId(null)}
                projectId={project.id}
                eventName={
                    userModalEntranceId
                        ? ((
                              nodes.find((n) => n.id === userModalEntranceId)?.data?.data as
                                  | Record<string, unknown>
                                  | undefined
                          )?.event_name as string | undefined)
                        : undefined
                }
                onSelect={(u, data) => {
                    const entranceId = userModalEntranceId
                    setUserModalEntranceId(null)
                    if (entranceId) triggerUser(entranceId, u.id, data)
                    onUserEnteredNode(entranceId ?? "")
                }}
            />

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
