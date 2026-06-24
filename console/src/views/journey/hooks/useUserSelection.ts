import { useCallback, useEffect, useRef } from "react"
import { apiUrl } from "@/api"
import oapiClient from "@/oapi/client"
import { toast } from "sonner"
import { useTranslation } from "react-i18next"
import { useSearchParams } from "react-router"
import { fetchEventSource } from "@microsoft/fetch-event-source"
import { client } from "@/api"

const STORAGE_KEY = (projectId: string, journeyId: string) =>
    `journey_follow_${projectId}_${journeyId}`

export function useUserSelection(
    projectId: string,
    journeyId: string,
    onUserEnteredNode: (external_id: string) => void,
    onStepExecuted: (external_id: string) => void,
    onStopFollowing?: () => void,
    entranceId?: string | null,
) {
    const { t } = useTranslation()
    const [searchParams, setSearchParams] = useSearchParams()

    const abortControllerRef = useRef<AbortController | null>(null)
    const activeUserIdRef = useRef<string | null>(null)
    const onUserEnteredNodeRef = useRef(onUserEnteredNode)
    onUserEnteredNodeRef.current = onUserEnteredNode
    const onStepExecutedRef = useRef(onStepExecuted)
    onStepExecutedRef.current = onStepExecuted
    const onStopFollowingRef = useRef(onStopFollowing)
    onStopFollowingRef.current = onStopFollowing
    const entranceIdRef = useRef(entranceId)
    entranceIdRef.current = entranceId

    const stopFollowing = useCallback(() => {
        if (abortControllerRef.current) {
            abortControllerRef.current.abort()
            abortControllerRef.current = null
        }
        activeUserIdRef.current = null
        sessionStorage.removeItem(STORAGE_KEY(projectId, journeyId))
        // Reset visual state before clearing the search param, so the
        // node/edge updates batch together in a single render cycle
        // rather than being split across a React Router navigation.
        onStopFollowingRef.current?.()
        setSearchParams({})
        // NOTE: adding followUser to deps is physically impossible because it has this function in it's own deps
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [projectId, journeyId])

    useEffect(() => {
        return () => {
            if (abortControllerRef.current) {
                abortControllerRef.current.abort()
            }
        }
    }, [])

    /**
     * Opens an SSE stream and returns a promise that resolves once the
     * connection is established (onopen fires with 200 OK). This lets
     * callers wait for the stream to be ready before triggering work
     * that would publish events the stream must not miss.
     */
    const followUser = useCallback(
        (userId: string): Promise<void> => {
            activeUserIdRef.current = userId
            sessionStorage.setItem(STORAGE_KEY(projectId, journeyId), userId)
            setSearchParams((prev) => {
                const next = new URLSearchParams(prev)
                next.set("follow", userId)
                return next
            })

            if (abortControllerRef.current) {
                abortControllerRef.current.abort()
            }

            const abortController = new AbortController()
            abortControllerRef.current = abortController

            const url = apiUrl(projectId, `journeys/${journeyId}/users/${userId}`)

            return new Promise<void>((resolve, reject) => {
                fetchEventSource(url, {
                    signal: abortController.signal,
                    credentials: "include",
                    onopen: async (response) => {
                        if (!response.ok) {
                            reject(new Error(`SSE connection failed: ${response.status}`))
                            return
                        }
                        resolve()
                    },
                    onmessage: (event) => {
                        switch (event.event) {
                            case "step": {
                                const data = JSON.parse(event.data)
                                // When scoped to a specific entrance, ignore
                                // events from other concurrent entrances.
                                if (
                                    entranceIdRef.current &&
                                    data.journey_entry_id &&
                                    data.journey_entry_id !== entranceIdRef.current
                                ) {
                                    break
                                }
                                onUserEnteredNodeRef.current(data.external_step_id)

                                if (data.step_type === "exit") {
                                    stopFollowing()
                                }
                                break
                            }
                            case "step_executed": {
                                const data = JSON.parse(event.data)
                                if (
                                    entranceIdRef.current &&
                                    data.journey_entry_id &&
                                    data.journey_entry_id !== entranceIdRef.current
                                ) {
                                    break
                                }
                                onStepExecutedRef.current(data.external_step_id)
                                break
                            }
                            case "cancelled": {
                                stopFollowing()
                                break
                            }
                            case "error": {
                                console.error("SSE server error:", event.data)
                                toast.error("Connection lost. Please try following the user again.")
                                stopFollowing()
                                break
                            }
                        }
                    },
                    onerror: (err) => {
                        console.error("SSE connection error:", err)
                        toast.error("Connection lost. Please try following the user again.")
                        reject(err)
                        // Throw to prevent automatic retry
                        throw err
                    },
                })
            })
        },
        [projectId, journeyId, stopFollowing, setSearchParams],
    )

    const triggerUser = useCallback(
        async (stepId: string, userId: string, data?: Record<string, unknown>) => {
            try {
                // 1. Open SSE stream first and wait for connection to be established
                //    so we never miss events that fire immediately after the trigger POST.
                await followUser(userId)

                // 2. Now that the stream is open, trigger the journey
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

                if (error) {
                    // POST failed — close the SSE stream we just opened
                    stopFollowing()
                    throw new Error(error.detail ?? "Failed to trigger user")
                }

                toast.success(t("user_triggered"))
            } catch (e) {
                toast.error(`Error: ${e}`)
            }
        },
        [projectId, journeyId, t, followUser, stopFollowing],
    )

    const cancelExecution = useCallback(
        async (userId: string) => {
            try {
                await client.delete(
                    `/admin/projects/${projectId}/journeys/${journeyId}/users/${userId}`,
                )
                stopFollowing()
            } catch (e) {
                toast.error(`Error cancelling execution: ${e}`)
            }
        },
        [projectId, journeyId, stopFollowing],
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
        stopFollowing,
        cancelExecution,
        skipDelay,
        skipDelayForActiveUser,
        searchParams,
        STORAGE_KEY,
    }
}
