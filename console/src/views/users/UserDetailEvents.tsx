import React, { useCallback, useContext, useMemo, useState, useRef } from "react"
import { useForm, Controller } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { useTranslation } from "react-i18next"
import type { EventFormValues } from "@/validation/event-form"
import { eventFormSchema } from "@/validation/event-form"
import {
    Activity,
    ChevronLeft,
    ChevronRight,
    ChevronDown,
    Plus,
    Zap,
    Clock,
    Search,
} from "lucide-react"
import { ProjectContext, UserContext } from "../../contexts"
import { PreferencesContext } from "@/contexts/PreferencesContext"
import { useResolver } from "../../hooks"
import { formatDate, cn } from "../../utils"
import { getRandomColor } from "@/lib/colors"
import { toast } from "sonner"
import api from "../../api"
import oapiClient from "../../oapi/client"
import type { SearchParams, UserEvent } from "../../types"
import Iframe from "@/components/iframe"

import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Skeleton } from "@/components/ui/skeleton"
import { Combobox } from "@/components/ui/combobox"
import { AttributeEditor } from "@/components/ui/attribute-editor"
import { Label } from "@/components/ui/label"
import { JsonView } from "@/components/ui/json-view"
import {
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from "@/components/ui/table"
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from "@/components/ui/dialog"

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
    event: UserEvent
}

function EventExpandedRow({ event }: EventExpandedRowProps) {
    const { t } = useTranslation()
    const [preferences] = useContext(PreferencesContext)
    const hasData = event.data && Object.keys(event.data).length > 0

    return (
        <TableRow className="bg-muted/30 hover:bg-muted/30">
            <TableCell colSpan={4} className="p-0">
                <div className="px-6 py-4 space-y-4">
                    {/* Event Info */}
                    <div className="flex flex-col sm:flex-row gap-4 sm:gap-8">
                        <div className="space-y-1">
                            <p className="text-xs font-medium text-muted-foreground uppercase tracking-wider">
                                {t("event_id", "Event ID")}
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
                            title={t("event_data", "Event data")}
                            defaultExpanded
                        />
                    )}
                </div>
            </TableCell>
        </TableRow>
    )
}

export default function UserDetailEvents() {
    const { t } = useTranslation()
    const [preferences] = useContext(PreferencesContext)
    const [project] = useContext(ProjectContext)
    const [user] = useContext(UserContext)

    const [page, setPage] = useState(1)
    const [searchQuery, setSearchQuery] = useState("")
    const [debouncedQuery, setDebouncedQuery] = useState("")
    const [expandedEventId, setExpandedEventId] = useState<string | null>(null)
    const [previewEvent, setPreviewEvent] = useState<UserEvent | null>(null)
    const [isCreateOpen, setIsCreateOpen] = useState(false)
    const [isCreating, setIsCreating] = useState(false)
    const [newEventData, setNewEventData] = useState<Record<string, unknown>>({})

    const eventForm = useForm<EventFormValues>({
        resolver: zodResolver(eventFormSchema),
        defaultValues: { event_name: "" },
    })
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

    const [result, , reload] = useResolver(
        useCallback(async () => {
            const params: SearchParams = {
                limit,
                offset: (page - 1) * limit,
                search: debouncedQuery || undefined,
            }
            return await api.users.events(project.id, user.id, params)
        }, [project.id, user.id, page, debouncedQuery]),
    )

    const events: UserEvent[] | undefined = result?.results
    const total = result?.total ?? 0
    const totalPages = Math.ceil(total / limit)
    const hasNextPage = page < totalPages
    const hasPrevPage = page > 1

    const [schemasResult] = useResolver(
        useCallback(async () => {
            try {
                const { data } = await oapiClient.GET(
                    "/api/admin/projects/{projectID}/subjects/user/events/schema",
                    { params: { path: { projectID: project.id } } },
                )
                return data?.results ?? []
            } catch {
                return []
            }
        }, [project.id]),
    )

    const eventOptions = useMemo(() => {
        if (!schemasResult) return []
        return schemasResult.map((s) => ({ path: s.name }))
    }, [schemasResult])

    const createEvent = async (formData: EventFormValues) => {
        setIsCreating(true)
        try {
            await oapiClient.POST(
                "/api/admin/projects/{projectID}/subjects/users/{userID}/events",
                {
                    params: { path: { projectID: project.id, userID: user.id } },
                    body: {
                        name: formData.event_name.trim(),
                        data: Object.keys(newEventData).length > 0 ? newEventData : undefined,
                    },
                },
            )
            // Reload events after a short delay to allow async processing
            setTimeout(() => reload(), 1000)
            setIsCreateOpen(false)
            eventForm.reset()
            setNewEventData({})
            toast.success(t("event_created", "Event created"))
        } catch {
            toast.error(t("failed_to_create_event", "Failed to create event"))
        } finally {
            setIsCreating(false)
        }
    }

    const toggleExpand = (event: UserEvent) => {
        const hasPreview = !!event.data?.result?.message?.html
        if (hasPreview) {
            setPreviewEvent(event)
        } else {
            setExpandedEventId(expandedEventId === event.id ? null : event.id)
        }
    }

    return (
        <div className="space-y-4">
            {/* Search */}
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
                <Button className="flex-1 sm:flex-initial" onClick={() => setIsCreateOpen(true)}>
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
                            <TableHead>{t("event_name", "Event")}</TableHead>
                            <TableHead className="hidden sm:table-cell">{t("timestamp")}</TableHead>
                            <TableHead className="w-24">{t("data")}</TableHead>
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
                                    <TableCell>
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
                                            onClick={() => toggleExpand(event)}
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
                                            <TableCell>
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

            {/* Email Preview Dialog (for email_sent events with HTML content) */}
            <Dialog
                open={previewEvent !== null}
                onOpenChange={(open) => !open && setPreviewEvent(null)}
            >
                <DialogContent className="max-w-5xl max-h-[90vh]">
                    <DialogHeader>
                        <DialogTitle className="font-mono">{previewEvent?.name}</DialogTitle>
                    </DialogHeader>
                    {previewEvent && (
                        <div className="grid grid-cols-1 md:grid-cols-2 gap-4 min-h-[60vh]">
                            <div className="space-y-3 overflow-auto">
                                <p className="text-sm text-muted-foreground">
                                    {formatDate(preferences, previewEvent.created_at, "PPpp")}
                                </p>
                                <JsonView
                                    data={{
                                        name: previewEvent.name,
                                        ...previewEvent.data,
                                        created_at: previewEvent.created_at,
                                    }}
                                    title={t("event_data", "Event data")}
                                    defaultExpanded
                                />
                            </div>
                            <div className="border rounded-lg overflow-hidden">
                                {previewEvent.data?.result?.message?.html && (
                                    <Iframe
                                        content={previewEvent.data.result.message.html as string}
                                        fullHeight={true}
                                        width="100%"
                                    />
                                )}
                            </div>
                        </div>
                    )}
                </DialogContent>
            </Dialog>

            {/* Create Event Dialog */}
            <Dialog open={isCreateOpen} onOpenChange={setIsCreateOpen}>
                <DialogContent className="sm:max-w-2xl max-h-[90vh] overflow-y-auto">
                    <DialogHeader>
                        <DialogTitle>{t("create_event", "Create Event")}</DialogTitle>
                        <DialogDescription>
                            {t("create_event_description", "Create a new event for this user.")}
                        </DialogDescription>
                    </DialogHeader>
                    <div className="grid gap-4 py-4">
                        <div className="grid gap-2">
                            <Label>{t("event_name", "Event")} *</Label>
                            <Controller
                                control={eventForm.control}
                                name="event_name"
                                render={({ field }) => (
                                    <Combobox
                                        options={eventOptions}
                                        value={field.value}
                                        onValueChange={field.onChange}
                                        placeholder={t(
                                            "enter_event_name",
                                            "Type or select an event name...",
                                        )}
                                        emptyText={t(
                                            "no_events_found",
                                            "No matching events. Type a name to create one.",
                                        )}
                                    />
                                )}
                            />
                            {eventForm.formState.errors.event_name && (
                                <p className="text-sm text-destructive">
                                    {eventForm.formState.errors.event_name.message}
                                </p>
                            )}
                        </div>
                        <div className="grid gap-2">
                            <Label>{t("data", "Data")}</Label>
                            <AttributeEditor
                                value={newEventData}
                                onChange={setNewEventData}
                                emptyTitle={t("no_data", "No data")}
                                emptyDescription={t(
                                    "no_data_description_event",
                                    "Add custom data to this event.",
                                )}
                            />
                        </div>
                    </div>
                    <DialogFooter>
                        <Button
                            variant="outline"
                            onClick={() => setIsCreateOpen(false)}
                            disabled={isCreating}
                        >
                            {t("cancel")}
                        </Button>
                        <Button onClick={eventForm.handleSubmit(createEvent)} disabled={isCreating}>
                            {isCreating ? t("creating") : t("create")}
                        </Button>
                    </DialogFooter>
                </DialogContent>
            </Dialog>
        </div>
    )
}
