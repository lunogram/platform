import React, { useCallback, useContext, useState } from "react"
import { useTranslation } from "react-i18next"
import { Activity, ChevronLeft, ChevronRight, ChevronDown, Zap, Clock } from "lucide-react"
import { ProjectContext, OrganizationContext } from "../../contexts"
import { PreferencesContext } from "@/contexts/PreferencesContext"
import { useResolver } from "../../hooks"
import { formatDate } from "../../utils"
import { getRandomColor } from "@/lib/colors"
import oapiClient from "../../oapi/client"
import type { components } from "../../oapi/management.generated"

import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { JsonView } from "@/components/ui/json-view"
import {
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from "@/components/ui/table"
import { cn } from "@/utils"

type OrganizationEvent = components["schemas"]["OrganizationEvent"]

function getPageNumbers(current: number, total: number): (number | "...")[] {
    if (total <= 7) {
        return Array.from({ length: total }, (_, i) => i + 1)
    }

    if (current <= 3) {
        return [1, 2, 3, 4, 5, "...", total]
    }

    if (current >= total - 2) {
        return [1, "...", total - 4, total - 3, total - 2, total - 1, total]
    }

    return [1, "...", current - 1, current, current + 1, "...", total]
}

interface EventExpandedRowProps {
    event: OrganizationEvent
}

function EventExpandedRow({ event }: EventExpandedRowProps) {
    const { t } = useTranslation()
    const [preferences] = useContext(PreferencesContext)
    const hasData = event.data && Object.keys(event.data).length > 0

    return (
        <TableRow className="bg-muted/30 hover:bg-muted/30">
            <TableCell colSpan={4} className="p-0">
                <div className="px-4 sm:px-6 py-4 space-y-4">
                    {/* Event Info */}
                    <div className="flex flex-col sm:flex-row gap-4 sm:gap-8">
                        <div className="space-y-1">
                            <p className="text-xs font-medium text-muted-foreground uppercase tracking-wider">
                                {t("event_id")}
                            </p>
                            <code className="text-sm">{event.id}</code>
                        </div>
                        <div className="space-y-1">
                            <p className="text-xs font-medium text-muted-foreground uppercase tracking-wider">
                                {t("timestamp")}
                            </p>
                            <p className="text-sm">
                                {formatDate(preferences, event.created_at, "PPpp")}
                            </p>
                        </div>
                    </div>

                    {/* Event Data */}
                    {hasData && (
                        <JsonView
                            data={event.data as Record<string, unknown>}
                            title={t("event_data")}
                            defaultExpanded
                        />
                    )}
                </div>
            </TableCell>
        </TableRow>
    )
}

export default function OrganizationDetailEvents() {
    const { t } = useTranslation()
    const [preferences] = useContext(PreferencesContext)
    const [project] = useContext(ProjectContext)
    const [organization] = useContext(OrganizationContext)

    const [page, setPage] = useState(1)
    const [expandedEventId, setExpandedEventId] = useState<string | null>(null)
    const limit = 25

    const [result] = useResolver(
        useCallback(async () => {
            const response = await oapiClient.GET(
                "/api/admin/projects/{projectID}/subjects/organizations/{organizationID}/events",
                {
                    params: {
                        path: {
                            projectID: project.id,
                            organizationID: organization.id,
                        },
                        query: {
                            limit,
                            offset: (page - 1) * limit,
                        },
                    },
                },
            )

            if (response.error || !response.data) {
                return null
            }

            return {
                events: response.data.results,
                total: response.data.total,
            }
        }, [project.id, organization.id, page]),
    )

    const events = result?.events
    const total = result?.total ?? 0
    const totalPages = Math.ceil(total / limit)
    const hasNextPage = page < totalPages
    const hasPrevPage = page > 1

    const toggleExpand = (eventId: string) => {
        setExpandedEventId(expandedEventId === eventId ? null : eventId)
    }

    return (
        <div className="space-y-4">
            {/* Events Table */}
            <div className="border rounded-lg">
                <Table>
                    <TableHeader>
                        <TableRow>
                            <TableHead className="w-8 p-0"></TableHead>
                            <TableHead>{t("event_name")}</TableHead>
                            <TableHead className="hidden sm:table-cell">{t("timestamp")}</TableHead>
                            <TableHead className="hidden md:table-cell w-24">{t("data")}</TableHead>
                        </TableRow>
                    </TableHeader>
                    <TableBody>
                        {events === undefined ? (
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
                                    <TableCell className="hidden sm:table-cell">
                                        <Skeleton className="h-4 w-28" />
                                    </TableCell>
                                    <TableCell className="hidden md:table-cell">
                                        <Skeleton className="h-4 w-12" />
                                    </TableCell>
                                </TableRow>
                            ))
                        ) : events.length === 0 ? (
                            <TableRow>
                                <TableCell colSpan={4} className="h-48">
                                    <div className="flex flex-col items-center justify-center">
                                        <div className="flex h-12 w-12 items-center justify-center rounded-full bg-muted mb-4">
                                            <Activity className="h-6 w-6 text-muted-foreground" />
                                        </div>
                                        <p className="font-medium mb-1">{t("no_events_yet")}</p>
                                        <p className="text-sm text-muted-foreground max-w-xs text-center">
                                            {t(
                                                "no_events_description",
                                                "Events will appear here when activity occurs",
                                            )}
                                        </p>
                                    </div>
                                </TableCell>
                            </TableRow>
                        ) : (
                            events.map((event) => {
                                const isExpanded = expandedEventId === event.id
                                const hasData = event.data && Object.keys(event.data).length > 0

                                return (
                                    <React.Fragment key={event.id}>
                                        <TableRow
                                            className={cn(
                                                "cursor-pointer",
                                                isExpanded && "bg-muted/50",
                                            )}
                                            onClick={() => toggleExpand(event.id)}
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
                                                            backgroundColor: getRandomColor(
                                                                event.name,
                                                            ),
                                                        }}
                                                    >
                                                        <Zap className="h-4 w-4" />
                                                    </div>
                                                    <span className="font-medium font-mono text-sm">
                                                        {event.name}
                                                    </span>
                                                </div>
                                            </TableCell>
                                            <TableCell className="hidden sm:table-cell text-muted-foreground">
                                                <div className="flex items-center gap-1.5">
                                                    <Clock className="h-3.5 w-3.5" />
                                                    {formatDate(
                                                        preferences,
                                                        event.created_at,
                                                        "Pp",
                                                    )}
                                                </div>
                                            </TableCell>
                                            <TableCell className="hidden md:table-cell">
                                                {hasData ? (
                                                    <span className="text-xs text-muted-foreground bg-muted px-2 py-0.5 rounded">
                                                        {Object.keys(event.data!).length}
                                                    </span>
                                                ) : (
                                                    <span className="text-muted-foreground">—</span>
                                                )}
                                            </TableCell>
                                        </TableRow>

                                        {/* Expanded Row */}
                                        {isExpanded && (
                                            <EventExpandedRow
                                                key={`${event.id}-expanded`}
                                                event={event}
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
                        {total} {t("events", "events")}
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
