import React, { useCallback, useContext, useState, useMemo, useRef } from "react"
import { useTranslation } from "react-i18next"
import {
    CalendarClock,
    ChevronLeft,
    ChevronRight,
    ChevronDown,
    Clock,
    Repeat,
    Search,
} from "lucide-react"
import { toast } from "sonner"
import { ProjectContext } from "../contexts"
import { PreferencesContext } from "@/contexts/PreferencesContext"
import { useResolver } from "../hooks"
import { formatDate, cn, getPageNumbers } from "../utils"
import { getRandomColor } from "@/lib/colors"
import { client } from "../api"
import type { ScheduledSchema } from "../types"

import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Skeleton } from "@/components/ui/skeleton"
import { JsonView } from "@/components/ui/json-view"
import { DateTimeEdit } from "@/components/ui/datetime-edit"
import {
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from "@/components/ui/table"

/**
 * Common interface for a scheduled instance row.
 * Both user and organization scheduled instances share this shape.
 */
interface ScheduledItem {
    id: string
    scheduled_id: string
    scheduled_at: string
    start_at: string | null
    anchor_at: string | null
    interval: string | null
    data: Record<string, unknown> | null
    created_at: string
    updated_at: string
}

interface ScheduledExpandedRowProps {
    item: ScheduledItem
    patchUrl: string
    onSaved: () => void
}

function ScheduledExpandedRow({ item, patchUrl, onSaved }: ScheduledExpandedRowProps) {
    const { t } = useTranslation()
    const [preferences] = useContext(PreferencesContext)

    const handleScheduledAtSave = async (newIso: string) => {
        try {
            await client.patch(patchUrl, { scheduled_at: newIso })
            toast.success(t("scheduled_updated", "Scheduled time updated"))
            onSaved()
        } catch (error) {
            toast.error(t("failed_to_update_scheduled", "Failed to update scheduled time"))
            throw error
        }
    }

    return (
        <TableRow className="bg-muted/30 hover:bg-muted/30">
            <TableCell colSpan={4} className="p-0">
                <div className="px-6 py-4 space-y-4">
                    {/* Info */}
                    <div className="flex flex-col sm:flex-row gap-4 sm:gap-8">
                        <div className="space-y-1">
                            <p className="text-xs font-medium text-muted-foreground uppercase tracking-wider">
                                {t("id", "ID")}
                            </p>
                            <code className="text-sm">{item.id}</code>
                        </div>
                        <div className="space-y-1">
                            <p className="text-xs font-medium text-muted-foreground uppercase tracking-wider">
                                {t("scheduled_at", "Scheduled At")}
                            </p>
                            <DateTimeEdit value={item.scheduled_at} onSave={handleScheduledAtSave}>
                                <span className="text-sm">
                                    {formatDate(preferences, item.scheduled_at, "PPpp")}
                                </span>
                            </DateTimeEdit>
                        </div>
                        <div className="space-y-1">
                            <p className="text-xs font-medium text-muted-foreground uppercase tracking-wider">
                                {t("created_at", "Created At")}
                            </p>
                            <p className="text-sm">
                                {formatDate(preferences, item.created_at, "PPpp")}
                            </p>
                        </div>
                    </div>

                    {/* Data */}
                    <JsonView
                        data={(item.data as Record<string, unknown>) ?? {}}
                        title={t("scheduled_data", "Scheduled data")}
                        defaultExpanded
                    />
                </div>
            </TableCell>
        </TableRow>
    )
}

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

    const searchTimeoutRef = useRef<ReturnType<typeof setTimeout>>()
    const limit = 15

    const handleSearch = (value: string) => {
        setSearchQuery(value)
        setPage(1)
        clearTimeout(searchTimeoutRef.current)
        searchTimeoutRef.current = setTimeout(() => {
            setDebouncedQuery(value)
        }, 300)
    }

    // Fetch scheduled schemas to get id→name mapping
    const [schemasResult] = useResolver(
        useCallback(async () => {
            try {
                const { data } = await client.get<{ results: ScheduledSchema[] }>(
                    `/admin/projects/${project.id}/subjects/user/scheduled/schema`,
                )
                return data.results
            } catch {
                return [] as ScheduledSchema[]
            }
        }, [project.id]),
    )

    const schemaMap = useMemo(() => {
        const map = new Map<string, string>()
        if (schemasResult) {
            for (const s of schemasResult) {
                map.set(s.id, s.name)
            }
        }
        return map
    }, [schemasResult])

    const [result, , reloadScheduled] = useResolver(
        useCallback(async () => {
            const params: Record<string, unknown> = {
                limit,
                offset: (page - 1) * limit,
                search: debouncedQuery || undefined,
            }
            const { data } = await client.get<{
                results: ScheduledItem[]
                total: number
                limit: number
                offset: number
            }>(`/admin/projects/${project.id}/subjects/${subjectType}/${subjectId}/scheduled`, {
                params,
            })
            return data
        }, [project.id, subjectId, subjectType, page, debouncedQuery]),
    )

    const items = result?.results
    const total = result?.total ?? 0
    const totalPages = Math.ceil(total / limit)
    const hasNextPage = page < totalPages
    const hasPrevPage = page > 1

    return (
        <div className="space-y-4">
            {/* Search */}
            <div className="flex items-center gap-4">
                <div className="relative max-w-sm flex-1">
                    <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                    <Input
                        placeholder={t("search_scheduled", "Search scheduled...")}
                        value={searchQuery}
                        onChange={(e) => handleSearch(e.target.value)}
                        className="pl-9"
                    />
                </div>
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
                        </TableRow>
                    </TableHeader>
                    <TableBody>
                        {items === undefined ? (
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
                                </TableRow>
                            ))
                        ) : items.length === 0 ? (
                            <TableRow>
                                <TableCell colSpan={4} className="h-48">
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
                                const name = schemaMap.get(item.scheduled_id) ?? item.scheduled_id

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
                                        </TableRow>

                                        {/* Expanded Row */}
                                        {isExpanded && (
                                            <ScheduledExpandedRow
                                                key={`${item.id}-expanded`}
                                                item={item}
                                                patchUrl={`/admin/projects/${project.id}/subjects/${subjectType}/${subjectId}/scheduled/${item.id}`}
                                                onSaved={() => reloadScheduled()}
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
        </div>
    )
}
