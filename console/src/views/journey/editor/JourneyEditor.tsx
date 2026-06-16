import { useCallback, useContext, useEffect, useMemo, useRef, useState } from "react"
import { useNavigate, useSearchParams as useRouterSearchParams } from "react-router"
import { JourneyContext, ProjectContext } from "../../../contexts"
import api from "../../../api"
import oapiClient from "@/oapi/client"
import { useResolver } from "@/hooks"
import { useIsMobile } from "@/hooks/use-mobile"
import { Button } from "@/components/ui/button"
import { useTranslation } from "react-i18next"
import { JourneyStepUsers } from "../JourneyStepUsers"
import type { UUID } from "@/types/common"
import { UserSelectionModal } from "../JourneyUserSelectionModal"

import { stepsToNodes } from "./JourneyEditor.utils"
import { JourneyEditorActionsContext } from "./JourneyEditorActions"
import { JourneyEditorToolbar } from "../components/JourneyEditorToolbar"
import { JourneyCanvas } from "../components/JourneyCanvas"

import "./JourneyEditor.css"
import "@xyflow/react/dist/style.css"
import { useJourneyPersistence } from "../hooks/useJourneyPersistence"
import { useUserSelection } from "../hooks/useUserSelection"
import { JourneyTriggerSetup } from "../components/JourneyTriggerSetup"
import { JourneyVariableProvider } from "../JourneyVariableContext"
import { JourneyHintsContext } from "./JourneyHints"
import { useJourneyEditorGraph } from "../hooks/useJourneyEditorGraph"

export default function JourneyEditor() {
    const navigate = useNavigate()
    const { t } = useTranslation()
    const [project] = useContext(ProjectContext)
    const [journey, setJourney] = useContext(JourneyContext)
    const [routerSearchParams] = useRouterSearchParams()
    const entranceId = routerSearchParams.get("entrance")
    const replayUserId = routerSearchParams.get("user")
    const [viewUsersStep, setViewUsersStep] = useState<null | {
        stepId: UUID
        stepType: string
        stepName: string
    }>(null)
    const [userModalEntranceId, setUserModalEntranceId] = useState<string | null>(null)
    const [sidebarTab, setSidebarTab] = useState<"components" | "actions">("components")
    const isMobile = useIsMobile()

    const [stepsLoaded, setStepsLoaded] = useState(false)
    const [stepsLoadError, setStepsLoadError] = useState<string | null>(null)

    const isArchived = journey.status === "archived"
    const isEditable = !isArchived && !isMobile
    const isFollowing = !!routerSearchParams.get("follow")
    const graph = useJourneyEditorGraph({
        isEditable,
        stepsLoaded,
        followingUserId: isFollowing ? routerSearchParams.get("follow") : null,
        replayUserId,
    })
    const {
        nodes,
        edges,
        setNodes,
        setEdges,
        hasUnsavedChanges,
        setHasUnsavedChanges,
        clearHistory,
        onUserEnteredNode,
        onStepExecuted,
        resetRuntimeState,
    } = graph

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

    const { saving, publishing, saveSteps, publishJourney } = useJourneyPersistence(
        project,
        journey,
        setJourney,
        setNodes,
        setEdges,
        hasUnsavedChanges,
        setHasUnsavedChanges,
    )

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
        resetRuntimeState,
        entranceId,
    )

    const followingUserId = searchParams.get("follow")
    const prevFollowingRef = useRef(isFollowing)

    useEffect(() => {
        // When we transition from following → not following, reset the
        // visual state. This covers all exit paths: stop button, cancel
        // button, exit step reached, SSE error, etc.
        if (prevFollowingRef.current && !isFollowing) {
            resetRuntimeState()
        }
        prevFollowingRef.current = isFollowing
    }, [isFollowing, resetRuntimeState])

    const openUserModal = useCallback((nodeId: string) => setUserModalEntranceId(nodeId), [])

    const openViewUsersStep = useCallback(
        (step: { stepId: UUID; stepType: string; stepName?: string }) =>
            setViewUsersStep({
                stepId: step.stepId,
                stepType: step.stepType,
                stepName: step.stepName ?? "",
            }),
        [],
    )

    const nodeActions = useMemo(
        () => ({
            setViewUsersStep: openViewUsersStep,
            skipDelay: skipDelayForActiveUser,
            openUserModal,
        }),
        [openViewUsersStep, skipDelayForActiveUser, openUserModal],
    )

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
        if (stepsLoaded || stepsLoadError) return
        const load = async () => {
            try {
                const steps = await api.journeys.steps.get(project.id, journey.id)
                const { edges, nodes } = stepsToNodes(steps, nodeActions)
                setNodes(nodes)
                setEdges(edges)
                setStepsLoaded(true)
            } catch (e) {
                console.error("Failed to load journey steps:", e)
                setStepsLoadError(t("journey_steps_load_error", "Failed to load journey steps."))
            }
        }
        void load()
    }, [project.id, journey.id, setNodes, setEdges, stepsLoaded, stepsLoadError, nodeActions, t])

    const handleSaveDraft = useCallback(async () => {
        await saveSteps(nodes, edges, nodeActions)
        clearHistory()
    }, [saveSteps, nodes, edges, nodeActions, clearHistory])

    const handlePublish = useCallback(async () => {
        await publishJourney(nodes, edges, nodeActions)
        clearHistory()
    }, [publishJourney, nodes, edges, nodeActions, clearHistory])

    const showTriggerSetup = isEditable && stepsLoaded && nodes.length === 0

    return (
        <div className="journey-editor flex flex-col flex-1 h-svh min-h-0 overflow-hidden">
            <JourneyEditorToolbar
                projectId={project.id}
                journey={journey}
                isArchived={isArchived}
                isMobile={isMobile}
                hasUnsavedChanges={hasUnsavedChanges}
                saving={saving}
                publishing={publishing}
                onBack={() => navigate("../journeys")}
                onJourneyChange={setJourney}
                onSaveDraft={handleSaveDraft}
                onPublish={handlePublish}
            />

            {/* Main content: canvas + sidebar */}
            {stepsLoadError ? (
                <div className="flex flex-1 items-center justify-center p-6">
                    <div className="max-w-sm rounded-lg border bg-background p-5 text-center shadow-sm">
                        <p className="text-sm font-medium">{stepsLoadError}</p>
                        <p className="mt-1 text-sm text-muted-foreground">
                            {t(
                                "journey_steps_load_error_desc",
                                "Retry loading the editor before making changes.",
                            )}
                        </p>
                        <Button
                            type="button"
                            className="mt-4"
                            onClick={() => {
                                setStepsLoadError(null)
                                setStepsLoaded(false)
                            }}
                        >
                            {t("retry", "Retry")}
                        </Button>
                    </div>
                </div>
            ) : (
                <JourneyVariableProvider nodes={nodes} edges={edges}>
                    <JourneyEditorActionsContext.Provider value={graph.editorActions}>
                        <JourneyHintsContext.Provider value={graph.hintsValue}>
                            {showTriggerSetup ? (
                                <JourneyTriggerSetup
                                    onSelectTrigger={graph.createEntranceFromSetup}
                                />
                            ) : (
                                <JourneyCanvas
                                    project={project}
                                    journey={journey}
                                    actions={actions}
                                    graph={graph}
                                    sidebar={{
                                        tab: sidebarTab,
                                        onTabChange: setSidebarTab,
                                        onViewUsers: (stepId, stepType, stepName) =>
                                            setViewUsersStep({ stepId, stepType, stepName }),
                                        onSaveDraft: handleSaveDraft,
                                    }}
                                    runtime={{
                                        followingUserId,
                                        replayUserId,
                                        onStopFollowing: stopFollowing,
                                        onCancelExecution: cancelExecution,
                                        onDismissReplay: () => {
                                            resetRuntimeState()
                                            navigate(".", { replace: true })
                                        },
                                    }}
                                    isArchived={isArchived}
                                    isEditable={isEditable}
                                    isMobile={isMobile}
                                />
                            )}
                        </JourneyHintsContext.Provider>
                    </JourneyEditorActionsContext.Provider>
                </JourneyVariableProvider>
            )}

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
