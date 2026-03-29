import { useCallback, useContext, useEffect, useState, useRef } from "react"
import { Link, useNavigate, useParams } from "react-router"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"
import { AxiosError } from "axios"
import {
    Radio,
    ChevronLeft,
    ChevronRight,
    RefreshCw,
    AlertCircle,
    Mail,
    Smartphone,
    MessageSquareDot,
    Users,
    Clock,
    CheckCircle2,
    XCircle,
    Play,
    CalendarClock,
    Ban,
    Search,
    ExternalLink,
    Eye,
    FileText,
} from "lucide-react"

import oapiClient from "../../oapi/client"
import { formatDate, snakeToTitle } from "../../utils"
import { getRandomColor } from "@/lib/colors"
import { getUserDisplayName, getUserInitials } from "@/lib/name"
import { ProjectContext } from "../../contexts"
import { PreferencesContext } from "@/contexts/PreferencesContext"

import type { Broadcast, BroadcastState, BroadcastUser, ChannelType, User } from "@/types"
import type { UUID } from "@/types/common"

import { useBroadcastProgress } from "./hooks/useBroadcastProgress"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { Input } from "@/components/ui/input"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { DateTimeEdit } from "@/components/ui/datetime-edit"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"
import {
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from "@/components/ui/table"
import { NavTabs } from "@/components/ui/nav-tabs"
import { BroadcastMessagePreview } from "./BroadcastMessagePreview"

/** Subset of user fields used by the recipient list (from list preview). */
interface ListUser {
    id: string
    full_name?: string
    identifier?: Array<{
        source: string
        external_id: string
        metadata?: Record<string, unknown> | null
    }>
    email?: string
    phone?: string
}

/** Union type for rows in the recipients table. */
type RecipientRow = ListUser | BroadcastUser

/** States where the broadcast has been sent (or attempted). */
const SENT_STATES: BroadcastState[] = ["sending", "completed", "failed", "cancelled"]

function isSentState(state: BroadcastState): boolean {
    return SENT_STATES.includes(state)
}

const channelIcons: Record<ChannelType, typeof Mail> = {
    email: Mail,
    text: Smartphone,
    push: MessageSquareDot,
}

const stateConfig: Record<
    BroadcastState,
    { icon: typeof Clock; label: string; className: string }
> = {
    scheduled: {
        icon: CalendarClock,
        label: "Scheduled",
        className: "bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400",
    },
    pending: {
        icon: Clock,
        label: "Pending",
        className: "bg-secondary text-secondary-foreground",
    },
    sending: {
        icon: RefreshCw,
        label: "Sending",
        className: "bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400",
    },
    completed: {
        icon: CheckCircle2,
        label: "Sent",
        className: "bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400",
    },
    failed: {
        icon: XCircle,
        label: "Failed",
        className: "bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400",
    },
    cancelled: {
        icon: Ban,
        label: "Cancelled",
        className: "bg-orange-100 text-orange-700 dark:bg-orange-900/30 dark:text-orange-400",
    },
}

function getStateBadge(state: BroadcastState, t: (key: string, fallback?: string) => string) {
    const { label, className } = stateConfig[state] ?? stateConfig.pending
    return (
        <Badge variant="outline" className={`border-0 ${className}`}>
            {t(label.toLowerCase(), label)}
        </Badge>
    )
}

/**
 * Build a lookup of grid positions sorted by Euclidean distance from the
 * center tile.  Positions at the same distance are ordered deterministically
 * (top-to-bottom, left-to-right) so the fill pattern stays symmetrical.
 */
function spiralPositions(rows: number, cols: number) {
    const centerRow = Math.floor(rows / 2)
    const centerCol = Math.floor(cols / 2)

    const positions: Array<{ row: number; col: number; dist: number }> = []
    for (let r = 0; r < rows; r++) {
        for (let c = 0; c < cols; c++) {
            positions.push({
                row: r,
                col: c,
                dist: Math.sqrt((r - centerRow) ** 2 + (c - centerCol) ** 2),
            })
        }
    }

    // Sort by distance, then row, then column for a stable, balanced order
    positions.sort((a, b) => a.dist - b.dist || a.row - b.row || a.col - b.col)
    return positions
}

/**
 * Ambient decorative grid for the broadcast detail header.
 * Fills tiles from the center outward with user initials when a `users`
 * array is provided.  Remaining tiles are rendered as empty decorative cells.
 */
function BroadcastMosaic({ color, users }: { color: string; users?: RecipientRow[] | null }) {
    const cols = 10
    const rows = 4
    const centerRow = Math.floor(rows / 2)
    const centerCol = Math.floor(cols / 2)
    const cellSize = 72
    const gap = 8

    const gridHeight = rows * cellSize + (rows - 1) * gap
    const centerTileMiddle = centerRow * (cellSize + gap) + cellSize / 2
    const offsetY = centerTileMiddle - gridHeight / 2
    const maskCenterY = (centerTileMiddle / gridHeight) * 100
    const maxDist = Math.sqrt(centerRow ** 2 + centerCol ** 2)

    // Map grid positions → user (if any), filled from center outward
    const positions = spiralPositions(rows, cols)
    const userAt = new Map<string, RecipientRow>()
    if (users?.length) {
        for (let i = 0; i < Math.min(users.length, positions.length); i++) {
            const { row, col } = positions[i]
            userAt.set(`${row},${col}`, users[i])
        }
    }

    return (
        <div className="relative flex items-center justify-center select-none w-full">
            <div
                className="pointer-events-none absolute inset-0"
                style={{
                    background: `radial-gradient(circle at 50% 50%, ${color}10 0%, transparent 60%)`,
                }}
            />
            <div
                className="relative flex flex-col gap-2"
                style={{
                    transform: `translateY(-${offsetY}px)`,
                    maskImage: `radial-gradient(ellipse 70% 70% at 50% ${maskCenterY}%, black 30%, transparent 100%)`,
                    WebkitMaskImage: `radial-gradient(ellipse 70% 70% at 50% ${maskCenterY}%, black 30%, transparent 100%)`,
                }}
            >
                {Array.from({ length: rows }, (_, row) => {
                    const isOffset = row % 2 === 1
                    return (
                        <div
                            key={row}
                            className="flex gap-2"
                            style={{ paddingLeft: isOffset ? 36 : 0 }}
                        >
                            {Array.from({ length: cols }, (_, col) => {
                                const dist = Math.sqrt(
                                    (row - centerRow) ** 2 + (col - centerCol) ** 2,
                                )
                                const opacity = Math.max(0.35, 1 - (dist / maxDist) * 0.7)

                                const user = userAt.get(`${row},${col}`)
                                if (user) {
                                    const userColor = getRandomColor(
                                        user.full_name ?? user.email ?? user.id,
                                    )
                                    const initials = getUserInitials(user)
                                    const isCenter = row === centerRow && col === centerCol
                                    return (
                                        <div
                                            key={col}
                                            className="flex items-center justify-center rounded-2xl"
                                            style={{
                                                width: cellSize,
                                                height: cellSize,
                                                background: `${userColor}20`,
                                                border: `1.5px solid ${userColor}40`,
                                                ...(isCenter
                                                    ? {
                                                          boxShadow: `0 0 32px ${color}25, 0 4px 20px ${color}15`,
                                                      }
                                                    : {}),
                                            }}
                                        >
                                            <span
                                                className="font-semibold leading-none select-none"
                                                style={{
                                                    color: userColor,
                                                    fontSize: 20,
                                                }}
                                            >
                                                {initials}
                                            </span>
                                        </div>
                                    )
                                }
                                return (
                                    <div
                                        key={col}
                                        className="rounded-2xl border border-border/80 bg-background shadow-sm"
                                        style={{
                                            width: cellSize,
                                            height: cellSize,
                                            opacity,
                                        }}
                                    />
                                )
                            })}
                        </div>
                    )
                })}
            </div>
        </div>
    )
}

interface BroadcastDetailProps {
    broadcastId: UUID
}

export default function BroadcastDetail({ broadcastId }: BroadcastDetailProps) {
    const { t } = useTranslation()
    const navigate = useNavigate()
    const [project] = useContext(ProjectContext)
    const [preferences] = useContext(PreferencesContext)
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
    const usersSearchTimeoutRef = useRef<ReturnType<typeof setTimeout>>()
    const usersPageSize = 25

    // Tab state for mobile/tablet (< lg) — desktop shows both panels side-by-side
    const [mobileTab, setMobileTab] = useState<string>("recipients")
    const mobileTabs = [
        { key: "recipients", label: t("recipients", "Recipients"), icon: Users },
        { key: "preview", label: t("message_preview", "Message Preview"), icon: FileText },
    ]

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
                                limit: usersPageSize,
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
                                limit: usersPageSize,
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
    const { sent: streamedSent, total: streamedTotal } = useBroadcastProgress({
        projectId: project.id,
        broadcastId,
        enabled: broadcastState === "sending",
        onTerminal: useCallback(() => {
            // Broadcast finished — re-fetch to get final state + recipients.
            loadBroadcast()
            loadUsers()
        }, [loadBroadcast, loadUsers]),
    })

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

        // Validate scheduled time is in the future
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

        // Validate scheduled time is in the future
        const scheduledDate = new Date(scheduleValue)
        if (scheduledDate <= new Date()) {
            toast.error(t("scheduled_at_must_be_future", "Scheduled time must be in the future"))
            return
        }

        setIsScheduling(true)
        try {
            const newIso = new Date(scheduleValue).toISOString()
            await oapiClient.PATCH("/api/admin/projects/{projectID}/broadcasts/{broadcastID}", {
                params: {
                    path: { projectID: project.id, broadcastID: broadcastId },
                },
                body: { scheduled_at: newIso },
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

    // Loading skeleton
    if (!broadcast) {
        return (
            <div className="flex flex-col min-h-full">
                <div className="border-b bg-card/50">
                    <div className="p-4 sm:p-6">
                        <Skeleton className="h-4 w-40 mb-4" />
                        <div className="flex items-start gap-4">
                            <Skeleton className="h-14 w-14 rounded-xl" />
                            <div className="space-y-2">
                                <Skeleton className="h-6 w-48" />
                                <Skeleton className="h-4 w-72" />
                            </div>
                        </div>
                    </div>
                </div>
                <div className="flex-1 p-4 sm:p-6 space-y-6">
                    <Skeleton className="h-32 rounded-lg" />
                    <Skeleton className="h-64 rounded-lg" />
                </div>
            </div>
        )
    }

    const campaignName = broadcast.campaign?.name ?? broadcast.campaign_id
    const campaignColor = getRandomColor(campaignName ?? broadcast.id)
    const ChannelIcon = broadcast.campaign?.channel
        ? (channelIcons[broadcast.campaign.channel] ?? Radio)
        : Radio
    const isEditable = broadcast.state === "pending" || broadcast.state === "scheduled"

    return (
        <div className="flex flex-col min-h-full">
            {/* Header Section — with ambient mosaic background */}
            <div className="border-b bg-card/50 relative overflow-hidden">
                {/* Ambient mosaic — faded right-side background */}
                <div
                    className="absolute top-1/2 -translate-y-1/2 left-[50%] xl:left-[30%] right-0 hidden lg:block pointer-events-none opacity-[0.8]"
                    style={{
                        maskImage: "linear-gradient(to right, transparent 0%, black 40%)",
                        WebkitMaskImage: "linear-gradient(to right, transparent 0%, black 40%)",
                    }}
                >
                    <BroadcastMosaic color={campaignColor} users={users} />
                </div>

                <div className="p-4 sm:p-6 relative z-20">
                    {/* Breadcrumb */}
                    <nav className="flex items-center gap-1.5 text-sm text-muted-foreground mb-4">
                        <Link
                            to={`/projects/${project.id}/broadcasts`}
                            className="hover:text-foreground transition-colors"
                        >
                            {t("broadcasts", "Broadcasts")}
                        </Link>
                        <ChevronRight className="h-3.5 w-3.5" />
                        <span className="text-foreground font-medium">
                            {broadcast.campaign?.name ?? broadcast.id}
                        </span>
                    </nav>

                    {/* Broadcast Identity */}
                    <div className="flex flex-col sm:flex-row sm:items-start justify-between gap-4 sm:gap-6">
                        <div className="flex items-start gap-4 min-w-0">
                            <div
                                className="flex h-14 w-14 items-center justify-center rounded-xl shrink-0"
                                style={{ backgroundColor: campaignColor }}
                            >
                                <ChannelIcon className="h-7 w-7 text-white" />
                            </div>
                            <div className="space-y-1 min-w-0">
                                <div className="flex items-center gap-3">
                                    <h1 className="text-2xl font-semibold tracking-tight">
                                        <Link
                                            to={`/projects/${project.id}/campaigns/${broadcast.campaign_id}`}
                                            className="hover:underline"
                                        >
                                            {broadcast.campaign?.name ?? "—"}
                                        </Link>
                                    </h1>
                                    {getStateBadge(broadcast.state, t)}
                                </div>
                                <p className="text-sm text-muted-foreground flex items-center flex-wrap gap-x-2 gap-y-1">
                                    {broadcast.campaign?.channel && (
                                        <>
                                            <span className="inline-flex items-center gap-1">
                                                <ChannelIcon className="h-3 w-3" />
                                                {snakeToTitle(broadcast.campaign.channel)}
                                            </span>
                                            <span>·</span>
                                        </>
                                    )}
                                    <Link
                                        to={`/projects/${project.id}/lists/${broadcast.list_id}`}
                                        className="hover:text-foreground transition-colors hover:underline"
                                    >
                                        {broadcast.list_name || "—"}
                                    </Link>
                                    <span>·</span>
                                    <span>
                                        {streamedSent != null
                                            ? `${streamedSent.toLocaleString()}${streamedTotal != null && streamedTotal > 0 ? ` / ${streamedTotal.toLocaleString()}` : ""} ${t("sent", "sent")}`
                                            : broadcast.total > 0
                                              ? `${broadcast.total.toLocaleString()} ${t("sent", "sent")}`
                                              : usersTotal != null
                                                ? `${usersTotal.toLocaleString()} ${t("recipients", "recipients")}`
                                                : t("no_recipients", "No recipients")}
                                    </span>
                                    <span>·</span>
                                    <span>
                                        {broadcast.completed_at
                                            ? `${t("completed", "Completed")} ${formatDate(preferences, broadcast.completed_at, "PP")}`
                                            : broadcast.started_at
                                              ? `${t("started", "Started")} ${formatDate(preferences, broadcast.started_at, "PP")}`
                                              : `${t("created")} ${formatDate(preferences, broadcast.created_at, "PP")}`}
                                    </span>
                                </p>
                            </div>
                        </div>

                        {/* Action buttons for editable broadcasts */}
                        {isEditable && (
                            <div className="flex items-center gap-2 shrink-0">
                                <Button
                                    variant="destructive"
                                    size="sm"
                                    onClick={handleCancel}
                                    disabled={isCancelling}
                                >
                                    {isCancelling ? (
                                        <RefreshCw className="mr-2 h-3.5 w-3.5 animate-spin" />
                                    ) : (
                                        <Ban className="mr-2 h-3.5 w-3.5" />
                                    )}
                                    {t("cancel_broadcast", "Cancel")}
                                </Button>
                                <Tooltip>
                                    <TooltipTrigger asChild>
                                        <div>
                                            <Button
                                                onClick={handleSend}
                                                disabled={
                                                    isSending || broadcast.state === "scheduled"
                                                }
                                                size="sm"
                                            >
                                                {isSending ? (
                                                    <RefreshCw className="mr-2 h-3.5 w-3.5 animate-spin" />
                                                ) : (
                                                    <Play className="mr-2 h-3.5 w-3.5" />
                                                )}
                                                {t("send_now", "Send Now")}
                                            </Button>
                                        </div>
                                    </TooltipTrigger>
                                    {broadcast.state === "scheduled" && (
                                        <TooltipContent>
                                            {t(
                                                "send_now_disabled_scheduled",
                                                "Remove the schedule to send manually",
                                            )}
                                        </TooltipContent>
                                    )}
                                </Tooltip>
                            </div>
                        )}
                    </div>
                </div>
            </div>

            {/* Progress Bar for Sending State */}
            {broadcast.state === "sending" && (
                <div className="border-b bg-blue-50/50 dark:bg-blue-950/20 px-4 sm:px-6 py-3">
                    <div className="flex items-center gap-3">
                        <RefreshCw className="h-4 w-4 animate-spin text-blue-600 dark:text-blue-400" />
                        <div className="flex-1">
                            <div className="flex items-center justify-between text-sm mb-1">
                                <span className="text-blue-700 dark:text-blue-300 font-medium">
                                    {t("sending_broadcast", "Sending broadcast...")}
                                </span>
                                {streamedSent != null && (
                                    <span className="text-blue-600/70 dark:text-blue-400/70 text-xs tabular-nums">
                                        {streamedSent.toLocaleString()}
                                        {streamedTotal != null && streamedTotal > 0
                                            ? ` / ${streamedTotal.toLocaleString()}`
                                            : ""}{" "}
                                        {t("sent", "sent")}
                                    </span>
                                )}
                            </div>
                            <div className="h-1.5 rounded-full bg-blue-200/60 dark:bg-blue-800/40 overflow-hidden">
                                {streamedSent != null &&
                                streamedTotal != null &&
                                streamedTotal > 0 ? (
                                    <div
                                        className="h-full rounded-full bg-blue-600 dark:bg-blue-400 transition-all duration-500"
                                        style={{
                                            width: `${Math.min(100, (streamedSent / streamedTotal) * 100)}%`,
                                        }}
                                    />
                                ) : (
                                    <div className="h-full rounded-full bg-blue-600 dark:bg-blue-400 animate-pulse w-full" />
                                )}
                            </div>
                        </div>
                    </div>
                </div>
            )}

            {/* Schedule Banner — colored bar below header for editable broadcasts */}
            {isEditable && (
                <div
                    className={`border-b px-4 sm:px-6 py-3 ${
                        broadcast.scheduled_at
                            ? "bg-amber-50/50 dark:bg-amber-950/20"
                            : "bg-muted/30"
                    }`}
                >
                    {broadcast.scheduled_at ? (
                        <div className="flex items-center justify-between gap-4">
                            <div className="flex items-center gap-3">
                                <CalendarClock className="h-4 w-4 text-amber-600 dark:text-amber-400" />
                                <span className="text-sm text-amber-700 dark:text-amber-300">
                                    {t("scheduled_for", "Scheduled for")}
                                </span>
                                <DateTimeEdit
                                    value={broadcast.scheduled_at}
                                    onSave={handleReschedule}
                                >
                                    <span className="text-sm font-medium text-amber-800 dark:text-amber-200">
                                        {formatDate(preferences, broadcast.scheduled_at, "PPpp")}
                                    </span>
                                </DateTimeEdit>
                            </div>
                            <Button
                                variant="outline"
                                size="sm"
                                onClick={handleRemoveSchedule}
                                disabled={isScheduling}
                            >
                                {t("remove_schedule", "Remove schedule")}
                            </Button>
                        </div>
                    ) : scheduleValue !== "" ? (
                        <form
                            className="flex items-center gap-3 justify-end"
                            onSubmit={(e) => {
                                e.preventDefault()
                                handleSetSchedule()
                            }}
                        >
                            <CalendarClock className="h-4 w-4 text-muted-foreground" />
                            <input
                                type="datetime-local"
                                aria-label={t("schedule_date_time", "Schedule date and time")}
                                className="h-8 rounded-md border border-input bg-background px-2 text-sm"
                                value={scheduleValue}
                                onChange={(e) => setScheduleValue(e.target.value)}
                                autoFocus
                                required
                                disabled={isScheduling}
                            />
                            <Button
                                type="submit"
                                size="sm"
                                className="h-7 text-xs"
                                disabled={isScheduling}
                            >
                                {isScheduling ? t("saving", "Saving...") : t("save")}
                            </Button>
                            <Button
                                type="button"
                                variant="ghost"
                                size="sm"
                                className="h-7 text-xs"
                                onClick={() => setScheduleValue("")}
                                disabled={isScheduling}
                            >
                                {t("cancel")}
                            </Button>
                        </form>
                    ) : (
                        <div className="flex items-center justify-between gap-4">
                            <div className="flex items-center gap-2 text-sm text-muted-foreground">
                                <Clock className="h-4 w-4" />
                                <span>
                                    {t(
                                        "no_schedule_set_short",
                                        "No schedule — send manually or set a time",
                                    )}
                                </span>
                            </div>
                            <Button
                                variant="ghost"
                                size="sm"
                                className="h-7 text-xs"
                                onClick={() => {
                                    // Pre-fill with tomorrow at 9am
                                    const tomorrow = new Date()
                                    tomorrow.setDate(tomorrow.getDate() + 1)
                                    tomorrow.setHours(9, 0, 0, 0)
                                    const y = tomorrow.getFullYear()
                                    const m = String(tomorrow.getMonth() + 1).padStart(2, "0")
                                    const d = String(tomorrow.getDate()).padStart(2, "0")
                                    setScheduleValue(`${y}-${m}-${d}T09:00`)
                                }}
                            >
                                <CalendarClock className="mr-1.5 h-3 w-3" />
                                {t("schedule_send", "Schedule")}
                            </Button>
                        </div>
                    )}
                </div>
            )}

            {/* Content Area */}
            <div className="flex-1 flex flex-col lg:flex-row overflow-hidden">
                {/* Mobile/Tablet tabs — hidden on lg+ where both panels are visible */}
                <div className="border-b lg:hidden">
                    <div className="px-4 sm:px-6">
                        <NavTabs tabs={mobileTabs} value={mobileTab} onChange={setMobileTab} />
                    </div>
                </div>

                {/* Left panel — Recipients */}
                <div
                    className={`flex-1 lg:w-1/2 overflow-y-auto p-4 sm:p-6 space-y-4 ${mobileTab !== "recipients" ? "hidden lg:block" : ""}`}
                >
                    {/* Error Alert */}
                    {broadcast.state === "failed" && broadcast.error && (
                        <Alert variant="destructive">
                            <AlertCircle className="h-4 w-4" />
                            <AlertTitle>{t("broadcast_failed", "Broadcast Failed")}</AlertTitle>
                            <AlertDescription>{broadcast.error}</AlertDescription>
                        </Alert>
                    )}

                    <div className="flex items-center gap-3 flex-wrap">
                        <div className="relative flex-1 min-w-[180px] sm:max-w-sm">
                            <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                            <Input
                                placeholder={t("search_recipients", "Search recipients...")}
                                value={usersSearch}
                                onChange={(e) => handleUsersSearch(e.target.value)}
                                className="pl-9"
                            />
                        </div>
                        {usersTotal != null && (
                            <Badge variant="secondary" className="font-normal">
                                {usersTotal.toLocaleString()} {t("users").toLowerCase()}
                            </Badge>
                        )}
                        {isPreview && (
                            <Tooltip>
                                <TooltipTrigger asChild>
                                    <div className="flex items-center gap-1.5 rounded-md border border-amber-200 bg-amber-50/50 dark:border-amber-800 dark:bg-amber-950/20 px-2.5 py-1.5">
                                        <Eye className="h-3.5 w-3.5 text-amber-600 dark:text-amber-400 shrink-0" />
                                        <span className="text-xs text-amber-700 dark:text-amber-300 whitespace-nowrap">
                                            {t("preview", "Preview")}
                                        </span>
                                    </div>
                                </TooltipTrigger>
                                <TooltipContent>
                                    {t(
                                        "broadcast_preview_tooltip",
                                        "Showing current list members. The actual recipients will be determined when the broadcast is sent.",
                                    )}
                                </TooltipContent>
                            </Tooltip>
                        )}
                        {broadcast.list_id && (
                            <Button variant="outline" size="sm" asChild className="ml-auto">
                                <Link to={`/projects/${project.id}/lists/${broadcast.list_id}`}>
                                    <ExternalLink className="mr-1.5 h-3.5 w-3.5" />
                                    {t("view_list", "View list")}
                                </Link>
                            </Button>
                        )}
                    </div>

                    <div className="rounded-lg border bg-card">
                        <Table>
                            <TableHeader>
                                <TableRow>
                                    <TableHead>{t("name")}</TableHead>
                                    <TableHead>{t("email")}</TableHead>
                                    <TableHead className="hidden sm:table-cell">
                                        {t("phone")}
                                    </TableHead>
                                    {!isPreview && (
                                        <TableHead className="hidden md:table-cell">
                                            {t("status", "Status")}
                                        </TableHead>
                                    )}
                                </TableRow>
                            </TableHeader>
                            <TableBody>
                                {!users ? (
                                    Array.from({ length: 5 }).map((_, i) => (
                                        <TableRow key={i}>
                                            <TableCell>
                                                <Skeleton className="h-4 w-32" />
                                            </TableCell>
                                            <TableCell>
                                                <Skeleton className="h-4 w-36" />
                                            </TableCell>
                                            <TableCell className="hidden sm:table-cell">
                                                <Skeleton className="h-4 w-24" />
                                            </TableCell>
                                            {!isPreview && (
                                                <TableCell className="hidden md:table-cell">
                                                    <Skeleton className="h-4 w-16" />
                                                </TableCell>
                                            )}
                                        </TableRow>
                                    ))
                                ) : users.length === 0 ? (
                                    <TableRow>
                                        <TableCell
                                            colSpan={isPreview ? 3 : 4}
                                            className="h-32 text-center"
                                        >
                                            <div className="flex flex-col items-center gap-2 text-muted-foreground">
                                                <Users className="h-8 w-8" />
                                                <p>
                                                    {usersDebouncedSearch
                                                        ? t(
                                                              "no_recipients_found",
                                                              "No recipients found",
                                                          )
                                                        : isPreview
                                                          ? t(
                                                                "no_recipients_in_list",
                                                                "No users in this list",
                                                            )
                                                          : broadcast?.state === "sending"
                                                            ? t(
                                                                  "waiting_for_sends",
                                                                  "Waiting for messages to be sent...",
                                                              )
                                                            : t(
                                                                  "no_recipients_sent",
                                                                  "No recipients were sent to",
                                                              )}
                                                </p>
                                            </div>
                                        </TableCell>
                                    </TableRow>
                                ) : (
                                    users.map((user) => {
                                        // BroadcastUser has user_id, ListUser has id directly
                                        const userId = "user_id" in user ? user.user_id : user.id
                                        const sendState =
                                            "state" in user && !isPreview ? user.state : null
                                        const goToUser = () =>
                                            navigate(`/projects/${project.id}/users/${userId}`)
                                        return (
                                            <TableRow
                                                key={user.id}
                                                className="cursor-pointer"
                                                tabIndex={0}
                                                onClick={goToUser}
                                                onKeyDown={(e) => {
                                                    if (e.key === "Enter" || e.key === " ") {
                                                        e.preventDefault()
                                                        goToUser()
                                                    }
                                                }}
                                            >
                                                <TableCell className="font-medium">
                                                    {getUserDisplayName(user, "—")}
                                                </TableCell>
                                                <TableCell className="text-muted-foreground">
                                                    {user.email || "—"}
                                                </TableCell>
                                                <TableCell className="text-muted-foreground hidden sm:table-cell">
                                                    {user.phone || "—"}
                                                </TableCell>
                                                {!isPreview && (
                                                    <TableCell className="hidden md:table-cell">
                                                        {sendState ? (
                                                            <Badge
                                                                variant="outline"
                                                                className="text-xs font-normal"
                                                            >
                                                                {snakeToTitle(sendState)}
                                                            </Badge>
                                                        ) : (
                                                            "—"
                                                        )}
                                                    </TableCell>
                                                )}
                                            </TableRow>
                                        )
                                    })
                                )}
                            </TableBody>
                        </Table>

                        {/* Pagination Footer */}
                        {users && users.length > 0 && usersTotal != null && (
                            <div className="flex items-center justify-between border-t px-4 py-3">
                                <p className="text-sm text-muted-foreground">
                                    {t(
                                        "showing_of_total",
                                        `Showing ${users.length} of ${usersTotal.toLocaleString()} recipients`,
                                        {
                                            count: users.length,
                                            total: usersTotal.toLocaleString(),
                                        },
                                    )}
                                </p>
                                {(usersOffset > 0 || usersOffset + usersPageSize < usersTotal) && (
                                    <div className="flex items-center gap-2">
                                        <Button
                                            variant="outline"
                                            size="sm"
                                            onClick={() =>
                                                setUsersOffset((prev) =>
                                                    Math.max(0, prev - usersPageSize),
                                                )
                                            }
                                            disabled={usersOffset <= 0}
                                            aria-label={t("previous")}
                                        >
                                            <ChevronLeft className="h-4 w-4 sm:mr-1" />
                                            <span className="hidden sm:inline">
                                                {t("previous")}
                                            </span>
                                        </Button>
                                        <Button
                                            variant="outline"
                                            size="sm"
                                            onClick={() =>
                                                setUsersOffset((prev) => prev + usersPageSize)
                                            }
                                            disabled={usersOffset + usersPageSize >= usersTotal}
                                            aria-label={t("next")}
                                        >
                                            <span className="hidden sm:inline">{t("next")}</span>
                                            <ChevronRight className="h-4 w-4 sm:ml-1" />
                                        </Button>
                                    </div>
                                )}
                            </div>
                        )}
                    </div>
                </div>

                {/* Right panel — Message Preview */}
                <div
                    className={`lg:w-1/2 lg:border-l overflow-y-auto p-4 sm:p-6 ${mobileTab !== "preview" ? "hidden lg:block" : ""}`}
                >
                    <BroadcastMessagePreview
                        campaignId={broadcast.campaign_id}
                        defaultUser={users?.[0] as User | undefined}
                    />
                </div>
            </div>
        </div>
    )
}

export function BroadcastDetailRoute() {
    const { broadcastId = "" } = useParams<{ broadcastId: string }>()
    return <BroadcastDetail broadcastId={broadcastId as UUID} />
}
