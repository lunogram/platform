import React, { useContext, useState, useRef } from "react"
import { useTranslation } from "react-i18next"
import { useQuery } from "@tanstack/react-query"
import { Activity, ChevronRight, ChevronDown, Zap, Clock, Plus, Search } from "lucide-react"
import { ProjectContext, OrganizationContext } from "../../contexts"
import { PreferencesContext } from "@/contexts/PreferencesContext"
import { formatDate } from "../../utils"
import { getRandomColor } from "@/lib/colors"
import { CreateOrganizationEventDialog } from "./CreateOrganizationEventDialog"
import { EventExpandedRow } from "./EventExpandedRow"
import { LoadErrorRow } from "./LoadErrorRow"
import { PageNumbers } from "./PageNumbers"
import { organizationEventsQuery } from "./queries"

import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Skeleton } from "@/components/ui/skeleton"
import {
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from "@/components/ui/table"
import { cn } from "@/utils"

export default function OrganizationDetailEvents() {
    const { t } = useTranslation()
    const [preferences] = useContext(PreferencesContext)
    const [project] = useContext(ProjectContext)
    const [organization] = useContext(OrganizationContext)

    const [page, setPage] = useState(1)
    const [searchQuery, setSearchQuery] = useState("")
    const [debouncedQuery, setDebouncedQuery] = useState("")
    const [expandedEventId, setExpandedEventId] = useState<string | null>(null)
    const searchTimeoutRef = useRef<ReturnType<typeof setTimeout>>(undefined)
    const limit = 25

    const [isCreateOpen, setIsCreateOpen] = useState(false)

    const handleSearch = (value: string) => {
        setSearchQuery(value)
        setPage(1)
        clearTimeout(searchTimeoutRef.current)
        searchTimeoutRef.current = setTimeout(() => {
            setDebouncedQuery(value)
        }, 300)
    }

    const list = useQuery(
        organizationEventsQuery(project.id, organization.id, {
            limit,
            offset: (page - 1) * limit,
            search: debouncedQuery,
        }),
    )

    const events = list.data?.results ?? []
    const total = list.data?.total ?? 0
    const totalPages = Math.ceil(total / limit)

    const toggleExpand = (eventId: string) => {
        setExpandedEventId(expandedEventId === eventId ? null : eventId)
    }

    return (
        <div className="space-y-4">
            {/* Search / Create Bar */}
            <div className="flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-3 sm:gap-4">
                <div className="relative sm:max-w-sm flex-1">
                    <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                    <Input
                        placeholder={t("search_events", "Search events...")}
                        value={searchQuery}
                        onChange={(e) => handleSearch(e.target.value)}
                        className="pl-9"
                    />
                </div>
                <Button onClick={() => setIsCreateOpen(true)} className="flex-1 sm:flex-initial">
                    <Plus className="mr-2 h-4 w-4" />
                    {t("create")}
                </Button>
            </div>

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
                                    <TableCell className="hidden sm:table-cell">
                                        <Skeleton className="h-4 w-28" />
                                    </TableCell>
                                    <TableCell className="hidden md:table-cell">
                                        <Skeleton className="h-4 w-12" />
                                    </TableCell>
                                </TableRow>
                            ))
                        ) : list.isError ? (
                            <LoadErrorRow
                                colSpan={4}
                                message={t(
                                    "organization_events_load_failed",
                                    "Couldn't load events.",
                                )}
                                onRetry={() => list.refetch()}
                            />
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
                        <PageNumbers page={page} totalPages={totalPages} onPageChange={setPage} />
                    )}
                </div>
            </div>

            <CreateOrganizationEventDialog open={isCreateOpen} onOpenChange={setIsCreateOpen} />
        </div>
    )
}
