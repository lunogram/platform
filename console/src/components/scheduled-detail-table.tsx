import React, { useContext, useState, useMemo, useRef } from "react"
import { useQuery } from "@tanstack/react-query"
import { useTranslation } from "react-i18next"
import {
    CalendarClock,
    ChevronLeft,
    ChevronRight,
    ChevronDown,
    Clock,
    Loader2,
    MoreHorizontal,
    Pause,
    Play,
    Plus,
    Repeat,
    Search,
    Trash2,
} from "lucide-react"
import { ProjectContext } from "../contexts"
import { PreferencesContext } from "@/contexts/PreferencesContext"
import { formatDate, cn, getPageNumbers } from "../utils"
import { getRandomColor } from "@/lib/colors"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Skeleton } from "@/components/ui/skeleton"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"
import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import {
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from "@/components/ui/table"
import { ScheduledActionDialogs, type ScheduledAction } from "./scheduled/action-dialogs"
import { CreateScheduledDialog, SchemasLoadError } from "./scheduled/create-dialog"
import { ScheduledExpandedRow } from "./scheduled/expanded-row"
import {
    SCHEDULED_PAGE_SIZE,
    scheduledListQuery,
    scheduledSchemasQuery,
    type ScheduledItem,
} from "./scheduled/queries"

interface ScheduledDetailTableProps {
    /**
     * The subject ID (user ID or organization ID).
     */
    subjectId: string
    /**
     * The subject type, used to build the API URL segment.
     * "users" or "organizations".
     */
    subjectType: "users" | "organizations"
}

export default function ScheduledDetailTable({
    subjectId,
    subjectType,
}: ScheduledDetailTableProps) {
    const { t } = useTranslation()
    const [preferences] = useContext(PreferencesContext)
    const [project] = useContext(ProjectContext)

    const [page, setPage] = useState(1)
    const [searchQuery, setSearchQuery] = useState("")
    const [debouncedQuery, setDebouncedQuery] = useState("")
    const [expandedId, setExpandedId] = useState<string | null>(null)
    const [isCreateOpen, setIsCreateOpen] = useState(false)
    const [actionTarget, setActionTarget] = useState<{
        item: ScheduledItem
        action: ScheduledAction
    } | null>(null)

    const searchTimeoutRef = useRef<ReturnType<typeof setTimeout>>(undefined)
    const limit = SCHEDULED_PAGE_SIZE

    const handleSearch = (value: string) => {
        setSearchQuery(value)
        setPage(1)
        clearTimeout(searchTimeoutRef.current)
        searchTimeoutRef.current = setTimeout(() => {
            setDebouncedQuery(value)
        }, 300)
    }

    // The schemas give scheduled items their names. Without them rows can only
    // show raw ids, so a failed load is shown rather than hidden.
    const schemas = useQuery(scheduledSchemasQuery(project.id))

    const schemaMap = useMemo(() => {
        const map = new Map<string, string>()
        for (const s of schemas.data ?? []) {
            map.set(s.id, s.name)
        }
        return map
    }, [schemas.data])

    const scheduleOptions = useMemo(
        () => (schemas.data ?? []).map((s) => ({ path: s.name })),
        [schemas.data],
    )

    const nameOf = (item: ScheduledItem) => schemaMap.get(item.scheduled_id) ?? item.scheduled_id

    // Keyed by page and search, so a slow response for an old search or page
    // can't replace the current one.
    const list = useQuery(
        scheduledListQuery(project.id, subjectType, subjectId, { page, search: debouncedQuery }),
    )

    const items = list.data?.results ?? []
    const total = list.data?.total ?? 0
    const totalPages = Math.ceil(total / limit)
    const hasNextPage = page < totalPages
    const hasPrevPage = page > 1

    return (
        <div className="space-y-4">
            {schemas.isError && <SchemasLoadError onRetry={() => schemas.refetch()} />}

            {/* Search + Create */}
            <div className="flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-3 sm:gap-4">
                <div className="relative sm:max-w-sm flex-1">
                    <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                    <Input
                        placeholder={t("search_scheduled", "Search scheduled...")}
                        value={searchQuery}
                        onChange={(e) => handleSearch(e.target.value)}
                        className="pl-9"
                    />
                </div>
                <Button onClick={() => setIsCreateOpen(true)} className="flex-1 sm:flex-initial">
                    <Plus className="mr-2 h-4 w-4" />
                    {t("create_scheduled", "Create")}
                </Button>
            </div>

            {/* Scheduled Table */}
            <div className="border rounded-lg">
                <Table>
                    <TableHeader>
                        <TableRow>
                            <TableHead className="w-8 p-0"></TableHead>
                            <TableHead>{t("name")}</TableHead>
                            <TableHead>{t("start_at", "Start At")}</TableHead>
                            <TableHead>{t("interval", "Interval")}</TableHead>
                            <TableHead className="w-10"></TableHead>
                        </TableRow>
                    </TableHeader>
                    <TableBody>
                        {list.isPending ? (
                            Array.from({ length: 5 }).map((_, i) => (
                                <TableRow key={i}>
                                    <TableCell className="p-0 pl-2">
                                        <Skeleton className="h-4 w-4" />
                                    </TableCell>
                                    <TableCell>
                                        <div className="flex items-center gap-3">
                                            <Skeleton className="h-8 w-8 rounded-lg" />
                                            <Skeleton className="h-4 w-32" />
                                        </div>
                                    </TableCell>
                                    <TableCell>
                                        <Skeleton className="h-4 w-28" />
                                    </TableCell>
                                    <TableCell>
                                        <Skeleton className="h-4 w-16" />
                                    </TableCell>
                                    <TableCell>
                                        <Skeleton className="h-4 w-4" />
                                    </TableCell>
                                </TableRow>
                            ))
                        ) : list.isError ? (
                            <TableRow>
                                <TableCell colSpan={5} className="h-32 text-center">
                                    <div className="flex flex-col items-center gap-3 text-muted-foreground">
                                        <p>
                                            {t(
                                                "scheduled_load_failed",
                                                "Couldn't load scheduled items.",
                                            )}
                                        </p>
                                        <Button
                                            variant="outline"
                                            size="sm"
                                            onClick={() => list.refetch()}
                                        >
                                            {t("retry", "Retry")}
                                        </Button>
                                    </div>
                                </TableCell>
                            </TableRow>
                        ) : items.length === 0 ? (
                            <TableRow>
                                <TableCell colSpan={5} className="h-48">
                                    <div className="flex flex-col items-center justify-center">
                                        <div className="flex h-12 w-12 items-center justify-center rounded-full bg-muted mb-4">
                                            <CalendarClock className="h-6 w-6 text-muted-foreground" />
                                        </div>
                                        <p className="font-medium mb-1">
                                            {t("no_scheduled_yet", "No scheduled items yet")}
                                        </p>
                                        <p className="text-sm text-muted-foreground max-w-xs text-center">
                                            {t(
                                                "no_scheduled_description",
                                                "Scheduled items will appear here when they are created",
                                            )}
                                        </p>
                                    </div>
                                </TableCell>
                            </TableRow>
                        ) : (
                            items.map((item) => {
                                const isExpanded = expandedId === item.id
                                const name = nameOf(item)

                                return (
                                    <React.Fragment key={item.id}>
                                        <TableRow
                                            className={cn(
                                                "cursor-pointer",
                                                isExpanded && "bg-muted/50",
                                            )}
                                            onClick={() =>
                                                setExpandedId(isExpanded ? null : item.id)
                                            }
                                        >
                                            <TableCell className="p-0 pl-3">
                                                {isExpanded ? (
                                                    <ChevronDown className="h-4 w-4 text-muted-foreground" />
                                                ) : (
                                                    <ChevronRight className="h-4 w-4 text-muted-foreground" />
                                                )}
                                            </TableCell>
                                            <TableCell>
                                                <div className="flex items-center gap-3">
                                                    <div
                                                        className="flex h-8 w-8 items-center justify-center rounded-lg text-white shrink-0"
                                                        style={{
                                                            backgroundColor: getRandomColor(name),
                                                        }}
                                                    >
                                                        <CalendarClock className="h-4 w-4" />
                                                    </div>
                                                    <span className="font-medium font-mono text-sm">
                                                        {name}
                                                    </span>
                                                    {item.paused_at && (
                                                        <Badge
                                                            variant="outline"
                                                            className="border-0 bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400"
                                                        >
                                                            {t("paused", "Paused")}
                                                        </Badge>
                                                    )}
                                                    {!item.paused_at && item.has_pending_events && (
                                                        <Tooltip>
                                                            <TooltipTrigger asChild>
                                                                <Badge
                                                                    variant="outline"
                                                                    className="border-0 bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400 gap-1"
                                                                >
                                                                    <Loader2 className="h-3 w-3 animate-spin" />
                                                                    {t("sending", "Sending")}
                                                                </Badge>
                                                            </TooltipTrigger>
                                                            <TooltipContent>
                                                                {t(
                                                                    "scheduled_sending_tooltip",
                                                                    "This message is scheduled to be sent out. It could take up to 1 minute to process.",
                                                                )}
                                                            </TooltipContent>
                                                        </Tooltip>
                                                    )}
                                                </div>
                                            </TableCell>
                                            <TableCell className="text-muted-foreground">
                                                <div className="flex items-center gap-1.5">
                                                    <Clock className="h-3.5 w-3.5" />
                                                    {item.start_at
                                                        ? formatDate(
                                                              preferences,
                                                              item.start_at,
                                                              "Pp",
                                                          )
                                                        : "—"}
                                                </div>
                                            </TableCell>
                                            <TableCell>
                                                {item.interval != null ? (
                                                    <span className="inline-flex items-center gap-1 text-xs text-muted-foreground bg-muted px-2 py-0.5 rounded">
                                                        <Repeat className="h-3 w-3" />
                                                        {item.interval}
                                                    </span>
                                                ) : (
                                                    <span className="text-muted-foreground">—</span>
                                                )}
                                            </TableCell>
                                            <TableCell>
                                                <DropdownMenu modal={false}>
                                                    <DropdownMenuTrigger asChild>
                                                        <Button
                                                            variant="ghost"
                                                            size="sm"
                                                            className="h-8 w-8 p-0"
                                                            onClick={(e) => e.stopPropagation()}
                                                        >
                                                            <MoreHorizontal className="h-4 w-4" />
                                                        </Button>
                                                    </DropdownMenuTrigger>
                                                    <DropdownMenuContent align="end">
                                                        {item.paused_at ? (
                                                            <DropdownMenuItem
                                                                onClick={(e) => {
                                                                    e.stopPropagation()
                                                                    setActionTarget({
                                                                        item,
                                                                        action: "resume",
                                                                    })
                                                                }}
                                                            >
                                                                <Play className="mr-2 h-4 w-4" />
                                                                {t("resume", "Resume")}
                                                            </DropdownMenuItem>
                                                        ) : (
                                                            <DropdownMenuItem
                                                                onClick={(e) => {
                                                                    e.stopPropagation()
                                                                    setActionTarget({
                                                                        item,
                                                                        action: "pause",
                                                                    })
                                                                }}
                                                            >
                                                                <Pause className="mr-2 h-4 w-4" />
                                                                {t("pause", "Pause")}
                                                            </DropdownMenuItem>
                                                        )}
                                                        <DropdownMenuItem
                                                            onClick={(e) => {
                                                                e.stopPropagation()
                                                                setActionTarget({
                                                                    item,
                                                                    action: "delete",
                                                                })
                                                            }}
                                                            className="text-destructive focus:text-destructive"
                                                        >
                                                            <Trash2 className="mr-2 h-4 w-4" />
                                                            {t("delete")}
                                                        </DropdownMenuItem>
                                                    </DropdownMenuContent>
                                                </DropdownMenu>
                                            </TableCell>
                                        </TableRow>

                                        {/* Expanded Row */}
                                        {isExpanded && (
                                            <ScheduledExpandedRow
                                                key={`${item.id}-expanded`}
                                                item={item}
                                                subjectType={subjectType}
                                                subjectId={subjectId}
                                            />
                                        )}
                                    </React.Fragment>
                                )
                            })
                        )}
                    </TableBody>
                </Table>

                {/* Pagination */}
                <div className="flex items-center justify-between border-t px-4 py-3">
                    <p className="text-sm text-muted-foreground">
                        {total} {t("scheduled", "scheduled")}
                    </p>
                    {totalPages > 1 && (
                        <div className="flex items-center gap-1">
                            <Button
                                variant="ghost"
                                size="sm"
                                onClick={() => setPage((p) => p - 1)}
                                disabled={!hasPrevPage}
                                className="h-8 w-8 p-0"
                            >
                                <ChevronLeft className="h-4 w-4" />
                            </Button>

                            {getPageNumbers(page, totalPages).map((pageNum, idx) =>
                                pageNum === "..." ? (
                                    <span
                                        key={`ellipsis-${idx}`}
                                        className="px-1 text-muted-foreground"
                                    >
                                        ...
                                    </span>
                                ) : (
                                    <Button
                                        key={pageNum}
                                        variant={page === pageNum ? "default" : "ghost"}
                                        size="sm"
                                        onClick={() => setPage(pageNum as number)}
                                        className="h-8 w-8 p-0"
                                    >
                                        {pageNum}
                                    </Button>
                                ),
                            )}

                            <Button
                                variant="ghost"
                                size="sm"
                                onClick={() => setPage((p) => p + 1)}
                                disabled={!hasNextPage}
                                className="h-8 w-8 p-0"
                            >
                                <ChevronRight className="h-4 w-4" />
                            </Button>
                        </div>
                    )}
                </div>
            </div>

            <CreateScheduledDialog
                open={isCreateOpen}
                onOpenChange={setIsCreateOpen}
                subjectType={subjectType}
                subjectId={subjectId}
                scheduleOptions={scheduleOptions}
                onRetrySchemas={schemas.isError ? () => schemas.refetch() : undefined}
            />

            <ScheduledActionDialogs
                subjectType={subjectType}
                subjectId={subjectId}
                target={actionTarget}
                onClose={() => setActionTarget(null)}
                nameOf={nameOf}
            />
        </div>
    )
}
