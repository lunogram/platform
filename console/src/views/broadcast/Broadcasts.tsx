import { useCallback, useContext, useEffect, useState, useRef } from "react"
import { useNavigate } from "react-router"
import { useTranslation } from "react-i18next"
import {
    Search,
    ChevronLeft,
    ChevronRight,
    ArrowRight,
    Radio,
    Mail,
    Smartphone,
    MessageSquareDot,
    CalendarClock,
} from "lucide-react"

import oapiClient from "../../oapi/client"
import { useResolver } from "../../hooks"
import { formatDate } from "../../utils"
import { getRandomColor } from "@/lib/colors"
import { ProjectContext } from "../../contexts"
import { PreferencesContext } from "@/contexts/PreferencesContext"

import { CreateBroadcastDialog } from "./CreateBroadcastDialog"

import type { Broadcast, BroadcastState, ChannelType } from "@/types"

import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import {
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from "@/components/ui/table"
import { Skeleton } from "@/components/ui/skeleton"
import { Badge } from "@/components/ui/badge"

const channelIcons: Record<ChannelType, typeof Mail> = {
    email: Mail,
    text: Smartphone,
    push: MessageSquareDot,
}

function getStateBadge(state: BroadcastState, t: (key: string, fallback?: string) => string) {
    const config: Record<BroadcastState, { label: string; className: string }> = {
        scheduled: {
            label: t("scheduled", "Scheduled"),
            className: "bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400",
        },
        pending: {
            label: t("pending", "Pending"),
            className: "bg-secondary text-secondary-foreground",
        },
        sending: {
            label: t("sending", "Sending"),
            className: "bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400",
        },
        completed: {
            label: t("sent", "Sent"),
            className: "bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400",
        },
        failed: {
            label: t("failed", "Failed"),
            className: "bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400",
        },
        cancelled: {
            label: t("cancelled", "Cancelled"),
            className: "bg-orange-100 text-orange-700 dark:bg-orange-900/30 dark:text-orange-400",
        },
    }
    const { label, className } = config[state] ?? config.pending
    return (
        <Badge variant="outline" className={`border-0 ${className}`}>
            {label}
        </Badge>
    )
}

export default function Broadcasts() {
    const [preferences] = useContext(PreferencesContext)
    const [project] = useContext(ProjectContext)
    const navigate = useNavigate()
    const { t } = useTranslation()

    const [searchQuery, setSearchQuery] = useState("")
    const [debouncedQuery, setDebouncedQuery] = useState("")
    const [isCreateOpen, setIsCreateOpen] = useState(false)
    const searchTimeoutRef = useRef<ReturnType<typeof setTimeout>>()

    const pageSize = 15
    const [offset, setOffset] = useState(0)

    const handleSearch = useCallback((value: string) => {
        setSearchQuery(value)
        setOffset(0)
        clearTimeout(searchTimeoutRef.current)
        searchTimeoutRef.current = setTimeout(() => {
            setDebouncedQuery(value)
        }, 300)
    }, [])

    const [result, , reload] = useResolver(
        useCallback(async () => {
            const { data } = await oapiClient.GET("/api/admin/projects/{projectID}/broadcasts", {
                params: {
                    path: { projectID: project.id },
                    query: {
                        limit: pageSize,
                        offset,
                        search: debouncedQuery || undefined,
                    },
                },
            })
            return data
                ? { results: (data.results ?? []) as Broadcast[], total: data.total ?? 0 }
                : { results: [] as Broadcast[], total: 0 }
        }, [project.id, debouncedQuery, offset]),
    )

    const broadcasts = result?.results
    const total = result?.total ?? 0
    const hasNextPage = offset + pageSize < total
    const hasPrevPage = offset > 0

    // Poll every 5s when any broadcast is in "sending" or "scheduled" state
    useEffect(() => {
        if (!broadcasts?.some((b) => b.state === "sending" || b.state === "scheduled")) return
        const interval = setInterval(() => {
            reload()
        }, 5000)
        return () => clearInterval(interval)
    }, [broadcasts, reload])

    const handleNextPage = () => {
        setOffset((prev) => prev + pageSize)
    }

    const handlePrevPage = () => {
        setOffset((prev) => Math.max(0, prev - pageSize))
    }

    const handleRowClick = (broadcast: Broadcast) => {
        navigate(`/projects/${project.id}/broadcasts/${broadcast.id.toString()}`)
    }

    return (
        <div className="flex flex-col gap-4 sm:gap-6 p-4 sm:p-6">
            {/* Header */}
            <div className="flex items-start gap-4">
                <div className="flex h-14 w-14 items-center justify-center rounded-xl shrink-0 bg-muted [&>svg]:h-7 [&>svg]:w-7 [&>svg]:text-muted-foreground">
                    <Radio />
                </div>
                <div className="space-y-1">
                    <h1 className="text-2xl font-semibold tracking-tight">
                        {t("broadcasts", "Broadcasts")}
                    </h1>
                    <p className="text-sm text-muted-foreground">
                        {t(
                            "broadcasts_description",
                            "Send one-time campaign messages to an entire list of users.",
                        )}
                    </p>
                </div>
            </div>

            {/* Search and Actions */}
            <div className="flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-3 sm:gap-4">
                <div className="relative sm:max-w-sm flex-1">
                    <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                    <Input
                        placeholder={t("search_broadcasts", "Search broadcasts...")}
                        value={searchQuery}
                        onChange={(e) => handleSearch(e.target.value)}
                        className="pl-9"
                    />
                </div>
                <CreateBroadcastDialog
                    open={isCreateOpen}
                    onOpenChange={setIsCreateOpen}
                    onCreated={async (broadcast) => {
                        setIsCreateOpen(false)
                        await navigate(
                            `/projects/${project.id}/broadcasts/${broadcast.id.toString()}`,
                        )
                    }}
                />
                <Button onClick={() => setIsCreateOpen(true)}>
                    <Radio className="mr-2 h-4 w-4" />
                    {t("send_broadcast", "Send Broadcast")}
                </Button>
            </div>

            {/* Table */}
            <div className="rounded-lg border bg-card">
                <Table>
                    <TableHeader>
                        <TableRow>
                            <TableHead>{t("campaign.singular", "Campaign")}</TableHead>
                            <TableHead className="hidden sm:table-cell">
                                {t("list", "List")}
                            </TableHead>
                            <TableHead>{t("state", "Status")}</TableHead>
                            <TableHead className="hidden sm:table-cell">
                                {t("sent", "Sent")}
                            </TableHead>
                            <TableHead className="hidden md:table-cell">
                                {t("date", "Date")}
                            </TableHead>
                        </TableRow>
                    </TableHeader>
                    <TableBody>
                        {!broadcasts ? (
                            Array.from({ length: 5 }).map((_, i) => (
                                <TableRow key={i}>
                                    <TableCell>
                                        <div className="flex items-center gap-3">
                                            <Skeleton className="h-8 w-8 rounded-md" />
                                            <div className="space-y-1">
                                                <Skeleton className="h-4 w-36" />
                                                <Skeleton className="h-3 w-20" />
                                            </div>
                                        </div>
                                    </TableCell>
                                    <TableCell className="hidden sm:table-cell">
                                        <Skeleton className="h-4 w-24" />
                                    </TableCell>
                                    <TableCell>
                                        <Skeleton className="h-5 w-16 rounded-md" />
                                    </TableCell>
                                    <TableCell className="hidden sm:table-cell">
                                        <Skeleton className="h-4 w-16" />
                                    </TableCell>
                                    <TableCell className="hidden md:table-cell">
                                        <Skeleton className="h-4 w-28" />
                                    </TableCell>
                                </TableRow>
                            ))
                        ) : broadcasts.length === 0 ? (
                            <TableRow>
                                <TableCell colSpan={5} className="h-32 text-center">
                                    <div className="flex flex-col items-center gap-2 text-muted-foreground">
                                        <Radio className="h-8 w-8" />
                                        <p>
                                            {debouncedQuery
                                                ? t("no_broadcasts_found", "No broadcasts found")
                                                : t("no_broadcasts_yet", "No broadcasts yet")}
                                        </p>
                                        {!debouncedQuery && (
                                            <Button
                                                variant="outline"
                                                size="sm"
                                                onClick={() => setIsCreateOpen(true)}
                                                className="mt-2"
                                            >
                                                <Radio className="mr-2 h-4 w-4" />
                                                {t("create_broadcast", "Send Broadcast")}
                                            </Button>
                                        )}
                                    </div>
                                </TableCell>
                            </TableRow>
                        ) : (
                            broadcasts.map((broadcast) => {
                                const campaignName =
                                    broadcast.campaign?.name ?? broadcast.campaign_id
                                const campaignColor = getRandomColor(campaignName ?? broadcast.id)
                                const ChannelIcon = broadcast.campaign?.channel
                                    ? (channelIcons[broadcast.campaign.channel] ?? Mail)
                                    : Radio
                                return (
                                    <TableRow
                                        key={broadcast.id}
                                        className="cursor-pointer"
                                        tabIndex={0}
                                        onClick={() => handleRowClick(broadcast)}
                                        onKeyDown={(e) => {
                                            if (e.key === "Enter" || e.key === " ") {
                                                e.preventDefault()
                                                handleRowClick(broadcast)
                                            }
                                        }}
                                    >
                                        <TableCell>
                                            <div className="flex items-center gap-3">
                                                <div
                                                    className="flex h-8 w-8 items-center justify-center rounded-md shrink-0"
                                                    style={{
                                                        backgroundColor: campaignColor,
                                                    }}
                                                >
                                                    <ChannelIcon className="h-4 w-4 text-white" />
                                                </div>
                                                <div>
                                                    <div className="font-medium">
                                                        {broadcast.campaign?.name ?? "—"}
                                                    </div>
                                                    <div className="text-sm text-muted-foreground">
                                                        {broadcast.campaign?.channel ?? ""}
                                                    </div>
                                                </div>
                                            </div>
                                        </TableCell>
                                        <TableCell className="hidden sm:table-cell text-muted-foreground">
                                            {broadcast.list_name || "—"}
                                        </TableCell>
                                        <TableCell>{getStateBadge(broadcast.state, t)}</TableCell>
                                        <TableCell className="hidden sm:table-cell text-muted-foreground">
                                            {broadcast.sent > 0
                                                ? broadcast.sent.toLocaleString()
                                                : "—"}
                                        </TableCell>
                                        <TableCell className="hidden md:table-cell text-muted-foreground">
                                            {broadcast.scheduled_at ? (
                                                <span className="inline-flex items-center gap-1.5">
                                                    <CalendarClock className="h-3.5 w-3.5 text-amber-500" />
                                                    {formatDate(
                                                        preferences,
                                                        broadcast.scheduled_at,
                                                        "PPp",
                                                    )}
                                                </span>
                                            ) : (
                                                formatDate(preferences, broadcast.created_at, "PP")
                                            )}
                                        </TableCell>
                                    </TableRow>
                                )
                            })
                        )}
                    </TableBody>
                </Table>

                {/* Pagination */}
                {broadcasts && broadcasts.length > 0 && (
                    <div className="flex items-center justify-between border-t px-4 py-3">
                        <p className="text-sm text-muted-foreground">
                            {total} {t("broadcasts", "Broadcasts").toLowerCase()}
                        </p>
                        {(hasPrevPage || hasNextPage) && (
                            <div className="flex items-center gap-2">
                                <Button
                                    variant="outline"
                                    size="sm"
                                    onClick={handlePrevPage}
                                    disabled={!hasPrevPage}
                                    aria-label={t("previous")}
                                >
                                    <ChevronLeft className="h-4 w-4 sm:mr-1" />
                                    <span className="hidden sm:inline">{t("previous")}</span>
                                </Button>
                                <Button
                                    variant="outline"
                                    size="sm"
                                    onClick={handleNextPage}
                                    disabled={!hasNextPage}
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

            {/* Tip Card */}
            <div className="group relative overflow-hidden rounded-lg bg-gradient-to-br from-primary/10 via-primary/5 to-transparent border p-4 sm:p-6">
                <div className="relative z-10 max-w-md">
                    <h3 className="font-semibold text-foreground">
                        {t("broadcast_tip_title", "Send campaigns to lists")}
                    </h3>
                    <p className="mt-1 text-sm text-muted-foreground">
                        {t(
                            "broadcast_tip_description",
                            "Broadcasts let you send a campaign to an entire list at once. Perfect for newsletters, announcements, and promotions.",
                        )}
                    </p>
                    <Button
                        variant="link"
                        className="mt-2 h-auto p-0 text-primary"
                        onClick={() => window.open("/api/", "_blank")}
                    >
                        {t("view_api_docs", "View API documentation")}
                        <ArrowRight className="ml-1 h-3 w-3 transition-transform duration-300 group-hover:translate-x-1" />
                    </Button>
                </div>

                {/* Decorative elements */}
                <div className="absolute -right-6 -bottom-6 flex gap-4">
                    <div className="hidden sm:flex h-20 w-20 items-center justify-center rounded-xl bg-primary/10 rotate-12 transition-all duration-500 ease-out group-hover:rotate-6 group-hover:-translate-y-2 group-hover:bg-primary/15">
                        <Radio
                            className="h-10 w-10 text-primary/40 transition-all duration-500 group-hover:text-primary/60 group-hover:scale-110"
                            strokeWidth={1.25}
                        />
                    </div>
                    <div className="flex h-20 w-20 items-center justify-center rounded-xl bg-primary/10 -rotate-6 translate-y-4 transition-all duration-500 ease-out delay-75 group-hover:rotate-3 group-hover:translate-y-0 group-hover:bg-primary/15">
                        <Mail
                            className="h-10 w-10 text-primary/40 transition-all duration-500 delay-75 group-hover:text-primary/60 group-hover:scale-110"
                            strokeWidth={1.25}
                        />
                    </div>
                    <div className="flex h-20 w-20 items-center justify-center rounded-xl bg-primary/10 rotate-12 -translate-y-2 transition-all duration-500 ease-out delay-150 group-hover:-rotate-6 group-hover:-translate-y-4 group-hover:bg-primary/15">
                        <Smartphone
                            className="h-10 w-10 text-primary/40 transition-all duration-500 delay-150 group-hover:text-primary/60 group-hover:scale-110"
                            strokeWidth={1.25}
                        />
                    </div>
                </div>
            </div>
        </div>
    )
}
