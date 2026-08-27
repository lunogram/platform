import { useCallback, useContext, useEffect, useRef, useState } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"
import { AxiosError } from "axios"

import oapiClient from "@/oapi/client"
import { ProjectContext } from "@/contexts"
import { parseDateAndTime, splitLocalDateTimeValue } from "@/lib/date-time"

import type { Broadcast, BroadcastUser } from "@/types"
import type { UUID } from "@/types/common"

import { isSentState, type ListUser, type RecipientRow } from "../broadcast-state"
import { useBroadcastProgress } from "./useBroadcastProgress"

const USERS_PAGE_SIZE = 25

export interface UseBroadcastDetailResult {
    // Data
    broadcast: Broadcast | null
    users: RecipientRow[] | null
    usersTotal: number | null
    displayTotal: number | null
    isPreview: boolean

    // Pagination & search
    usersOffset: number
    usersSearch: string
    usersPageSize: number
    setUsersOffset: React.Dispatch<React.SetStateAction<number>>
    handleUsersSearch: (value: string) => void

    // SSE progress
    streamedSent: number | null
    streamedFailed: number | null
    streamedTotal: number | null

    // Action states
    isSending: boolean
    isCancelling: boolean
    isScheduling: boolean
    scheduleValue: string
    setScheduleValue: React.Dispatch<React.SetStateAction<string>>

    // Actions
    handleSend: () => Promise<void>
    handleReschedule: (newIso: string) => Promise<void>
    handleSetSchedule: () => Promise<void>
    handleRemoveSchedule: () => Promise<void>
    handleCancel: () => Promise<void>
}

export function useBroadcastDetail(broadcastId: UUID): UseBroadcastDetailResult {
    const { t } = useTranslation()
    const [project] = useContext(ProjectContext)

    const [broadcast, setBroadcast] = useState<Broadcast | null>(null)
    const [isSending, setIsSending] = useState(false)
    const [isCancelling, setIsCancelling] = useState(false)
    const [isScheduling, setIsScheduling] = useState(false)
    const [scheduleValue, setScheduleValue] = useState("")

    // Recipients state
    const [users, setUsers] = useState<RecipientRow[] | null>(null)
    const [usersTotal, setUsersTotal] = useState<number | null>(null)
    const [usersOffset, setUsersOffset] = useState(0)
    const [usersSearch, setUsersSearch] = useState("")
    const [usersDebouncedSearch, setUsersDebouncedSearch] = useState("")
    const usersSearchTimeoutRef = useRef<ReturnType<typeof setTimeout> | undefined>(undefined)

    // Whether the recipients table shows a preview (list users) or actual sends
    const isPreview = broadcast ? !isSentState(broadcast.state) : true

    const loadBroadcast = useCallback(async () => {
        try {
            const { data } = await oapiClient.GET(
                "/api/admin/projects/{projectID}/broadcasts/{broadcastID}",
                {
                    params: {
                        path: { projectID: project.id, broadcastID: broadcastId },
                    },
                },
            )
            if (data) setBroadcast(data as Broadcast)
        } catch {
            // leave broadcast null on error
        }
    }, [project.id, broadcastId])

    // Extract primitives for stable dependency tracking
    const broadcastState = broadcast?.state
    const broadcastListId = broadcast?.list_id

    const loadUsers = useCallback(async () => {
        if (!broadcastState || !broadcastListId) return
        try {
            if (isSentState(broadcastState)) {
                // After send: load actual recipients from campaign_sends
                const { data } = await oapiClient.GET(
                    "/api/admin/projects/{projectID}/broadcasts/{broadcastID}/users",
                    {
                        params: {
                            path: { projectID: project.id, broadcastID: broadcastId },
                            query: {
                                limit: USERS_PAGE_SIZE,
                                offset: usersOffset,
                                search: usersDebouncedSearch || undefined,
                            },
                        },
                    },
                )
                if (data) {
                    setUsers(data.results as BroadcastUser[])
                    setUsersTotal(data.total ?? data.results?.length ?? 0)
                }
            } else {
                // Before send: preview list membership
                const { data } = await oapiClient.GET(
                    "/api/admin/projects/{projectID}/lists/{listID}/users",
                    {
                        params: {
                            path: { projectID: project.id, listID: broadcastListId },
                            query: {
                                limit: USERS_PAGE_SIZE,
                                offset: usersOffset,
                                search: usersDebouncedSearch || undefined,
                            },
                        },
                    },
                )
                if (data) {
                    setUsers(data.results as ListUser[])
                    setUsersTotal(data.total ?? data.results?.length ?? 0)
                }
            }
        } catch {
            setUsers([])
            setUsersTotal(null)
        }
    }, [
        project.id,
        broadcastId,
        broadcastState,
        broadcastListId,
        usersOffset,
        usersDebouncedSearch,
    ])

    useEffect(() => {
        loadBroadcast()
    }, [loadBroadcast])

    // Load users once broadcast is available (re-runs on search/pagination/state changes).
    useEffect(() => {
        if (broadcastState) {
            loadUsers()
        }
    }, [broadcastState, loadUsers])

    const handleUsersSearch = useCallback((value: string) => {
        setUsersSearch(value)
        setUsersOffset(0)
        clearTimeout(usersSearchTimeoutRef.current)
        usersSearchTimeoutRef.current = setTimeout(() => {
            setUsersDebouncedSearch(value)
        }, 300)
    }, [])

    // SSE: stream sent count while broadcast is sending.
    const {
        sent: streamedSent,
        failed: streamedFailed,
        total: streamedTotal,
    } = useBroadcastProgress({
        projectId: project.id,
        broadcastId,
        enabled: broadcastState === "sending",
        onTerminal: useCallback(() => {
            // Broadcast finished — re-fetch to get final state + recipients.
            loadBroadcast()
            loadUsers()
        }, [loadBroadcast, loadUsers]),
    })

    // Re-fetch the recipients table as sends trickle in from the SSE stream.
    const prevStreamedSent = useRef<number | null>(null)
    useEffect(() => {
        if (
            broadcastState === "sending" &&
            streamedSent != null &&
            streamedSent !== prevStreamedSent.current
        ) {
            prevStreamedSent.current = streamedSent
            loadUsers()
        }
    }, [broadcastState, streamedSent, loadUsers])

    // The broadcast total is the canonical audience size.
    // During sending, broadcast.total may still be 0 before the backend has
    // finished enqueuing, so fall back to the SSE streamedTotal or the
    // paginated usersTotal so the UI doesn't show "0 recipients".
    const displayTotal = broadcast?.total || streamedTotal || usersTotal

    const handleSend = async () => {
        if (!broadcast) return
        setIsSending(true)
        try {
            await oapiClient.POST("/api/admin/projects/{projectID}/broadcasts/{broadcastID}/send", {
                params: {
                    path: { projectID: project.id, broadcastID: broadcastId },
                },
            })
            loadBroadcast()
        } catch (err) {
            const detail =
                err instanceof AxiosError && typeof err.response?.data?.detail === "string"
                    ? err.response.data.detail
                    : null
            toast.error(detail || t("broadcast_send_error", "Failed to send broadcast"))
        } finally {
            setIsSending(false)
        }
    }

    const handleReschedule = async (newIso: string) => {
        if (!broadcast) return

        if (new Date(newIso) <= new Date()) {
            toast.error(t("scheduled_at_must_be_future", "Scheduled time must be in the future"))
            throw new Error("scheduled time must be in the future")
        }

        try {
            await oapiClient.PATCH("/api/admin/projects/{projectID}/broadcasts/{broadcastID}", {
                params: {
                    path: { projectID: project.id, broadcastID: broadcastId },
                },
                body: { scheduled_at: newIso },
            })
            loadBroadcast()
            toast.success(t("broadcast_rescheduled", "Broadcast rescheduled"))
        } catch {
            toast.error(t("broadcast_reschedule_error", "Failed to reschedule broadcast"))
            throw new Error("reschedule failed")
        }
    }

    const handleSetSchedule = async () => {
        if (!broadcast || !scheduleValue) return

        const { dateValue, timeValue } = splitLocalDateTimeValue(scheduleValue)
        const scheduledDate = parseDateAndTime(dateValue, timeValue)

        if (!scheduledDate || scheduledDate <= new Date()) {
            toast.error(t("scheduled_at_must_be_future", "Scheduled time must be in the future"))
            return
        }

        setIsScheduling(true)
        try {
            await oapiClient.PATCH("/api/admin/projects/{projectID}/broadcasts/{broadcastID}", {
                params: {
                    path: { projectID: project.id, broadcastID: broadcastId },
                },
                body: { scheduled_at: scheduledDate.toISOString() },
            })
            loadBroadcast()
            setScheduleValue("")
            toast.success(t("broadcast_scheduled", "Broadcast scheduled"))
        } catch {
            toast.error(t("broadcast_schedule_error", "Failed to schedule broadcast"))
        } finally {
            setIsScheduling(false)
        }
    }

    const handleRemoveSchedule = async () => {
        if (!broadcast) return
        setIsScheduling(true)
        try {
            await oapiClient.PATCH("/api/admin/projects/{projectID}/broadcasts/{broadcastID}", {
                params: {
                    path: { projectID: project.id, broadcastID: broadcastId },
                },
                body: { scheduled_at: null },
            })
            loadBroadcast()
            toast.success(t("broadcast_schedule_removed", "Schedule removed"))
        } catch {
            toast.error(t("broadcast_schedule_remove_error", "Failed to remove schedule"))
        } finally {
            setIsScheduling(false)
        }
    }

    const handleCancel = async () => {
        if (!broadcast) return
        setIsCancelling(true)
        try {
            await oapiClient.DELETE("/api/admin/projects/{projectID}/broadcasts/{broadcastID}", {
                params: {
                    path: { projectID: project.id, broadcastID: broadcastId },
                },
            })
            toast.success(t("broadcast_cancelled", "Broadcast cancelled"))
            loadBroadcast()
        } catch {
            toast.error(t("broadcast_cancel_error", "Failed to cancel broadcast"))
        } finally {
            setIsCancelling(false)
        }
    }

    return {
        broadcast,
        users,
        usersTotal,
        displayTotal,
        isPreview,

        usersOffset,
        usersSearch,
        usersPageSize: USERS_PAGE_SIZE,
        setUsersOffset,
        handleUsersSearch,

        streamedSent,
        streamedFailed,
        streamedTotal,

        isSending,
        isCancelling,
        isScheduling,
        scheduleValue,
        setScheduleValue,

        handleSend,
        handleReschedule,
        handleSetSchedule,
        handleRemoveSchedule,
        handleCancel,
    }
}
