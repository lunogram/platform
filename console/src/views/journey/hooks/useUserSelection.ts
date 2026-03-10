import { useCallback, useEffect, useRef } from "react"
import { apiUrl } from "@/api"
import oapiClient from "@/oapi/client"
import { toast } from "sonner"
import { useTranslation } from "react-i18next"
import { useSearchParams } from "react-router"

const STORAGE_KEY = (projectId: string, journeyId: string) =>
    `journey_follow_${projectId}_${journeyId}`

export function useUserSelection(
    projectId: string,
    journeyId: string,
    onUserEnteredNode: (external_id: string) => void,
    onStepExecuted: (external_id: string) => void,
) {
    const { t } = useTranslation()
    const [searchParams, setSearchParams] = useSearchParams()

    const eventSourceRef = useRef<EventSource | null>(null)
    const activeUserIdRef = useRef<string | null>(null)
    const onUserEnteredNodeRef = useRef(onUserEnteredNode)
    onUserEnteredNodeRef.current = onUserEnteredNode
    const onStepExecutedRef = useRef(onStepExecuted)
    onStepExecutedRef.current = onStepExecuted

    const stopFollowing = useCallback(() => {
        if (eventSourceRef.current) {
            eventSourceRef.current.close()
            eventSourceRef.current = null
        }
        activeUserIdRef.current = null
        sessionStorage.removeItem(STORAGE_KEY(projectId, journeyId))
        setSearchParams({})
        // NOTE: adding followUser to deps is physically impossible because it has this function in it's own deps
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [projectId, journeyId])

    useEffect(() => {
        return () => {
            if (eventSourceRef.current) {
                eventSourceRef.current.close()
            }
        }
    }, [])

    const followUser = useCallback(
        (userId: string) => {
            activeUserIdRef.current = userId
            sessionStorage.setItem(STORAGE_KEY(projectId, journeyId), userId)
            setSearchParams({ follow: userId })

            if (eventSourceRef.current) {
                eventSourceRef.current.close()
            }

            const es = new EventSource(apiUrl(projectId, `journeys/${journeyId}/users/${userId}`), {
                withCredentials: true,
            })

            es.addEventListener("step", (e) => {
                const data = JSON.parse(e.data)
                onUserEnteredNodeRef.current(data.external_step_id)

                if (data.step_type === "exit") {
                    stopFollowing()
                }
            })

            es.addEventListener("step_executed", (e) => {
                const data = JSON.parse(e.data)
                onStepExecutedRef.current(data.external_step_id)
            })

            es.onerror = (e) => {
                console.error("EventSource error:", e)
                toast.error("Connection lost. Please try following the user again.")

                if (eventSourceRef.current) {
                    eventSourceRef.current.close()
                }
            }

            eventSourceRef.current = es
        },
        [projectId, journeyId, stopFollowing, setSearchParams],
    )

    const triggerUser = useCallback(
        async (stepId: string, userId: string, data?: Record<string, unknown>) => {
            try {
                followUser(userId)
                const { error } = await oapiClient.POST(
                    "/api/admin/projects/{projectID}/journeys/{journeyID}/users/{userID}",
                    {
                        params: {
                            path: {
                                projectID: projectId,
                                journeyID: journeyId,
                                userID: userId,
                            },
                        },
                        body: { externalStepID: stepId, data },
                    },
                )
                if (error) throw new Error(error.detail ?? "Failed to trigger user")
                toast.success(t("user_triggered"))
            } catch (e) {
                toast.error(`Error: ${e}`)
            }
        },
        [projectId, journeyId, t, followUser],
    )

    const skipDelay = useCallback(
        async (stepId: string, userId: string) => {
            try {
                const { error } = await oapiClient.PUT(
                    "/api/admin/projects/{projectID}/journeys/{journeyID}/users/{userID}",
                    {
                        params: {
                            path: {
                                projectID: projectId,
                                journeyID: journeyId,
                                userID: userId,
                            },
                        },
                        body: { externalStepID: stepId },
                    },
                )
                if (error) throw new Error(error.detail ?? "Failed to skip delay")
                toast.success(t("user_skipped"))
            } catch (e) {
                toast.error(`Error: ${e}`)
            }
        },
        [projectId, journeyId, t],
    )

    const skipDelayForActiveUser = useCallback(
        async (stepId: string) => {
            const userId = activeUserIdRef.current
            if (!userId) {
                toast.error("No active user selected")
                return
            }
            await skipDelay(stepId, userId)
        },
        [skipDelay],
    )

    return {
        triggerUser,
        followUser,
        skipDelay,
        skipDelayForActiveUser,
        searchParams,
        STORAGE_KEY,
    }
}
