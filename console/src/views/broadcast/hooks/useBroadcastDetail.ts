import { useCallback, useContext, useEffect, useRef, useState } from "react"
import { useQuery, useQueryClient } from "@tanstack/react-query"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"

import { ProjectContext } from "@/contexts"
import { parseDateAndTime, splitLocalDateTimeValue } from "@/lib/date-time"

import type { Broadcast } from "@/types"
import type { UUID } from "@/types/common"

import { isSentState, type RecipientRow } from "../broadcast-state"
import {
    RECIPIENTS_PAGE_SIZE,
    broadcastKeys,
    broadcastQuery,
    recipientsQuery,
    useCancelBroadcast,
    useScheduleBroadcast,
    useSendBroadcast,
} from "../queries"
import { useBroadcastProgress } from "./useBroadcastProgress"

export interface UseBroadcastDetailResult {
    // Data
    broadcast: Broadcast | null
    /** The broadcast couldn't be loaded, so the page can only show an error. */
    broadcastError: boolean
    retryBroadcast: () => void
    users: RecipientRow[] | null
    usersTotal: number | null
    usersError: boolean
    retryUsers: () => void
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
    const queryClient = useQueryClient()

    const [scheduleValue, setScheduleValue] = useState("")

    // Recipients pagination & search
    const [usersOffset, setUsersOffset] = useState(0)
    const [usersSearch, setUsersSearch] = useState("")
    const [usersDebouncedSearch, setUsersDebouncedSearch] = useState("")
    const usersSearchTimeoutRef = useRef<ReturnType<typeof setTimeout> | undefined>(undefined)

    const broadcastResult = useQuery(broadcastQuery(project.id, broadcastId))
    const broadcast = broadcastResult.data ?? null

    // The page and search are part of the query key, so a slow response for an
    // earlier page or search can never replace the current one.
    const recipients = useQuery(
        recipientsQuery(project.id, broadcastResult.data, {
            offset: usersOffset,
            search: usersDebouncedSearch,
        }),
    )
    const users = recipients.data?.results ?? null
    const usersTotal = recipients.data?.total ?? null

    // Whether the recipients table shows a preview (list users) or actual sends
    const isPreview = broadcast ? !isSentState(broadcast.state) : true
    const broadcastState = broadcast?.state

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
            void queryClient.invalidateQueries({ queryKey: broadcastKeys.all(project.id) })
        }, [queryClient, project.id]),
    })

    // Re-fetch the recipients table as sends trickle in from the SSE stream.
    // Invalidating replaces a request still in flight instead of racing it.
    const prevStreamedSent = useRef<number | null>(null)
    useEffect(() => {
        if (
            broadcastState === "sending" &&
            streamedSent != null &&
            streamedSent !== prevStreamedSent.current
        ) {
            prevStreamedSent.current = streamedSent
            void queryClient.invalidateQueries({
                queryKey: broadcastKeys.recipients(project.id, broadcastId),
            })
        }
    }, [broadcastState, streamedSent, queryClient, project.id, broadcastId])

    // The broadcast total is the canonical audience size.
    // During sending, broadcast.total may still be 0 before the backend has
    // finished enqueuing, so fall back to the SSE streamedTotal or the
    // paginated usersTotal so the UI doesn't show "0 recipients".
    const displayTotal = broadcast?.total || streamedTotal || usersTotal

    // Failures are toasted by the mutation cache; the success toasts below
    // only run once the server accepted the change.
    const send = useSendBroadcast(broadcastId)
    const cancel = useCancelBroadcast(broadcastId)
    const reschedule = useScheduleBroadcast(
        broadcastId,
        t("broadcast_reschedule_error", "Failed to reschedule broadcast"),
    )
    const setSchedule = useScheduleBroadcast(
        broadcastId,
        t("broadcast_schedule_error", "Failed to schedule broadcast"),
    )
    const removeSchedule = useScheduleBroadcast(
        broadcastId,
        t("broadcast_schedule_remove_error", "Failed to remove schedule"),
    )

    const handleSend = async () => {
        if (!broadcast) return
        send.mutate(undefined)
    }

    const handleReschedule = async (newIso: string) => {
        if (!broadcast) return

        if (new Date(newIso) <= new Date()) {
            toast.error(t("scheduled_at_must_be_future", "Scheduled time must be in the future"))
            throw new Error("scheduled time must be in the future")
        }

        // Rejects on failure, which keeps the date editor open.
        await reschedule.mutateAsync(newIso)
        toast.success(t("broadcast_rescheduled", "Broadcast rescheduled"))
    }

    const handleSetSchedule = async () => {
        if (!broadcast || !scheduleValue) return

        const { dateValue, timeValue } = splitLocalDateTimeValue(scheduleValue)
        const scheduledDate = parseDateAndTime(dateValue, timeValue)

        if (!scheduledDate || scheduledDate <= new Date()) {
            toast.error(t("scheduled_at_must_be_future", "Scheduled time must be in the future"))
            return
        }

        setSchedule.mutate(scheduledDate.toISOString(), {
            onSuccess: () => {
                setScheduleValue("")
                toast.success(t("broadcast_scheduled", "Broadcast scheduled"))
            },
        })
    }

    const handleRemoveSchedule = async () => {
        if (!broadcast) return
        removeSchedule.mutate(null, {
            onSuccess: () => toast.success(t("broadcast_schedule_removed", "Schedule removed")),
        })
    }

    const handleCancel = async () => {
        if (!broadcast) return
        cancel.mutate(undefined, {
            onSuccess: () => toast.success(t("broadcast_cancelled", "Broadcast cancelled")),
        })
    }

    return {
        broadcast,
        broadcastError: broadcastResult.isError,
        retryBroadcast: () => void broadcastResult.refetch(),
        users,
        usersTotal,
        usersError: recipients.isError,
        retryUsers: () => void recipients.refetch(),
        displayTotal,
        isPreview,

        usersOffset,
        usersSearch,
        usersPageSize: RECIPIENTS_PAGE_SIZE,
        setUsersOffset,
        handleUsersSearch,

        streamedSent,
        streamedFailed,
        streamedTotal,

        isSending: send.isPending,
        isCancelling: cancel.isPending,
        isScheduling: setSchedule.isPending || removeSchedule.isPending,
        scheduleValue,
        setScheduleValue,

        handleSend,
        handleReschedule,
        handleSetSchedule,
        handleRemoveSchedule,
        handleCancel,
    }
}
