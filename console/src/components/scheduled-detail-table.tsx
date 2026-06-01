import React, { useCallback, useContext, useState, useMemo, useRef } from "react"
import { useForm, Controller } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { useTranslation } from "react-i18next"
import { scheduledCreateSchema } from "@/validation/scheduled/scheduled-create"
import type { ScheduledCreateFormValues } from "@/validation/scheduled/scheduled-create"
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
import { toast } from "sonner"
import { ProjectContext } from "../contexts"
import { PreferencesContext } from "@/contexts/PreferencesContext"
import { useResolver } from "../hooks"
import { formatDate, cn, getPageNumbers } from "../utils"
import { getRandomColor } from "@/lib/colors"
import { client } from "../api"
import oapiClient from "../oapi/client"
import type { ScheduledSchema } from "../types"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Skeleton } from "@/components/ui/skeleton"
import { JsonView } from "@/components/ui/json-view"
import { DateTimeEdit } from "@/components/ui/datetime-edit"
import { AttributeEditor } from "@/components/ui/attribute-editor"
import { Label } from "@/components/ui/label"
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
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from "@/components/ui/dialog"
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select"
import { Combobox } from "@/components/ui/combobox"

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
    paused_at: string | null
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
            <TableCell colSpan={5} className="p-0">
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
                            <div className="flex items-center gap-2">
                                <DateTimeEdit
                                    value={item.scheduled_at}
                                    onSave={handleScheduledAtSave}
                                >
                                    <span className="text-sm">
                                        {formatDate(preferences, item.scheduled_at, "PPpp")}
                                    </span>
                                </DateTimeEdit>
                                {!item.paused_at &&
                                    item.scheduled_at &&
                                    new Date(item.scheduled_at) <= new Date() && (
                                        <Tooltip>
                                            <TooltipTrigger asChild>
                                                <span className="inline-flex items-center gap-1 text-xs text-blue-600 dark:text-blue-400">
                                                    <Loader2 className="h-3 w-3 animate-spin" />
                                                </span>
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
    const [isCreateOpen, setIsCreateOpen] = useState(false)
    const [isCreating, setIsCreating] = useState(false)
    const [newScheduledData, setNewScheduledData] = useState<Record<string, unknown>>({})

    const scheduledForm = useForm<ScheduledCreateFormValues>({
        resolver: zodResolver(scheduledCreateSchema),
        defaultValues: {
            scheduled_name: "",
            scheduled_at: "",
            start_at: "",
            interval: "",
        },
    })

    // Dialog state for delete, pause, resume actions
    const [deleteTarget, setDeleteTarget] = useState<ScheduledItem | null>(null)
    const [pauseTarget, setPauseTarget] = useState<ScheduledItem | null>(null)
    const [pauseMode, setPauseMode] = useState<"immediately" | "after_next_interval">("immediately")
    const [resumeTarget, setResumeTarget] = useState<ScheduledItem | null>(null)
    const [resumeMode, setResumeMode] = useState<"immediately" | "at_next_interval">("immediately")

    const searchTimeoutRef = useRef<ReturnType<typeof setTimeout>>(setTimeout(() => {}, 0))
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

    const scheduleOptions = useMemo(() => {
        if (!schemasResult) return []
        return schemasResult.map((s) => ({ path: s.name }))
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

    const createScheduled = async (formData: ScheduledCreateFormValues) => {
        setIsCreating(true)
        try {
            const body = {
                scheduled_name: formData.scheduled_name.trim(),
                scheduled_at: formData.scheduled_at || undefined,
                start_at: formData.start_at || undefined,
                interval: formData.interval?.trim() || undefined,
                data: Object.keys(newScheduledData).length > 0 ? newScheduledData : undefined,
            }

            if (subjectType === "users") {
                await oapiClient.PUT(
                    "/api/admin/projects/{projectID}/subjects/users/{userID}/scheduled",
                    {
                        params: { path: { projectID: project.id, userID: subjectId } },
                        body,
                    },
                )
            } else {
                await oapiClient.PUT(
                    "/api/admin/projects/{projectID}/subjects/organizations/{organizationID}/scheduled",
                    {
                        params: { path: { projectID: project.id, organizationID: subjectId } },
                        body,
                    },
                )
            }

            await reloadScheduled()
            setIsCreateOpen(false)
            scheduledForm.reset()
            setNewScheduledData({})
            toast.success(t("scheduled_created", "Scheduled item created"))
        } catch {
            toast.error(t("failed_to_create_scheduled", "Failed to create scheduled item"))
        } finally {
            setIsCreating(false)
        }
    }

    const confirmDelete = async () => {
        if (!deleteTarget) return
        const item = deleteTarget
        setDeleteTarget(null)
        try {
            await client.delete(
                `/admin/projects/${project.id}/subjects/${subjectType}/${subjectId}/scheduled/${item.id}`,
            )
            await reloadScheduled()
        } catch {
            toast.error(t("failed_to_delete_scheduled", "Failed to delete scheduled item"))
        }
    }

    const confirmPause = async () => {
        if (!pauseTarget) return
        const item = pauseTarget
        setPauseTarget(null)
        try {
            await client.patch(
                `/admin/projects/${project.id}/subjects/${subjectType}/${subjectId}/scheduled/${item.id}`,
                { pause: pauseMode },
            )
            toast.success(t("scheduled_paused", "Schedule paused"))
            await reloadScheduled()
        } catch {
            toast.error(t("failed_to_pause_scheduled", "Failed to pause schedule"))
        }
    }

    const confirmResume = async () => {
        if (!resumeTarget) return
        const item = resumeTarget
        setResumeTarget(null)
        try {
            await client.patch(
                `/admin/projects/${project.id}/subjects/${subjectType}/${subjectId}/scheduled/${item.id}`,
                { resume: resumeMode },
            )
            toast.success(t("scheduled_resumed", "Schedule resumed"))
            await reloadScheduled()
        } catch {
            toast.error(t("failed_to_resume_scheduled", "Failed to resume schedule"))
        }
    }

    return (
        <div className="space-y-4">
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
                                    <TableCell>
                                        <Skeleton className="h-4 w-4" />
                                    </TableCell>
                                </TableRow>
                            ))
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
                                                    {item.paused_at && (
                                                        <Badge
                                                            variant="outline"
                                                            className="border-0 bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400"
                                                        >
                                                            {t("paused", "Paused")}
                                                        </Badge>
                                                    )}
                                                    {!item.paused_at &&
                                                        item.scheduled_at &&
                                                        new Date(item.scheduled_at) <=
                                                            new Date() && (
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
                                                                    setResumeTarget(item)
                                                                }}
                                                            >
                                                                <Play className="mr-2 h-4 w-4" />
                                                                {t("resume", "Resume")}
                                                            </DropdownMenuItem>
                                                        ) : (
                                                            <DropdownMenuItem
                                                                onClick={(e) => {
                                                                    e.stopPropagation()
                                                                    setPauseTarget(item)
                                                                }}
                                                            >
                                                                <Pause className="mr-2 h-4 w-4" />
                                                                {t("pause", "Pause")}
                                                            </DropdownMenuItem>
                                                        )}
                                                        <DropdownMenuItem
                                                            onClick={(e) => {
                                                                e.stopPropagation()
                                                                setDeleteTarget(item)
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

            {/* Create Scheduled Dialog */}
            <Dialog open={isCreateOpen} onOpenChange={setIsCreateOpen}>
                <DialogContent className="sm:max-w-2xl max-h-[90vh] overflow-y-auto">
                    <DialogHeader>
                        <DialogTitle>
                            {t("create_scheduled_item", "Create Scheduled Item")}
                        </DialogTitle>
                        <DialogDescription>
                            {t(
                                "create_scheduled_description",
                                "Create a new scheduled item for this subject.",
                            )}
                        </DialogDescription>
                    </DialogHeader>
                    <div className="grid gap-4 py-4">
                        <div className="grid sm:grid-cols-2 gap-4">
                            <div className="grid gap-2 content-start">
                                <Label>{t("scheduled_schema", "Schedule")} *</Label>
                                <Controller
                                    control={scheduledForm.control}
                                    name="scheduled_name"
                                    render={({ field }) => (
                                        <Combobox
                                            options={scheduleOptions}
                                            value={field.value}
                                            onValueChange={field.onChange}
                                            placeholder={t(
                                                "enter_schedule_name",
                                                "Type or select a schedule name...",
                                            )}
                                            emptyText={t(
                                                "no_schedules_found",
                                                "No matching schedules. Type a name to create one.",
                                            )}
                                        />
                                    )}
                                />
                                {scheduledForm.formState.errors.scheduled_name && (
                                    <p className="text-sm text-destructive">
                                        {scheduledForm.formState.errors.scheduled_name.message}
                                    </p>
                                )}
                            </div>
                            <div className="grid gap-2 content-start">
                                <Label htmlFor="interval">{t("interval", "Interval")}</Label>
                                <Input
                                    id="interval"
                                    placeholder={t(
                                        "enter_interval",
                                        "e.g., 24 hours, 7 days, 1 week",
                                    )}
                                    {...scheduledForm.register("interval")}
                                />
                                <p className="text-xs text-muted-foreground">
                                    {t("interval_help", "Makes this a recurring schedule")}
                                </p>
                            </div>
                        </div>
                        <div className="grid sm:grid-cols-2 gap-4">
                            <div className="grid gap-2 content-start">
                                <Label htmlFor="scheduled_at">
                                    {t("scheduled_at", "Scheduled At")}
                                </Label>
                                <Controller
                                    control={scheduledForm.control}
                                    name="scheduled_at"
                                    render={({ field }) => (
                                        <Input
                                            id="scheduled_at"
                                            type="datetime-local"
                                            value={field.value || ""}
                                            onChange={(e) =>
                                                field.onChange(
                                                    e.target.value
                                                        ? new Date(e.target.value).toISOString()
                                                        : "",
                                                )
                                            }
                                        />
                                    )}
                                />
                                <p className="text-xs text-muted-foreground">
                                    {t("scheduled_at_help", "Trigger time for single schedules")}
                                </p>
                            </div>
                            <div className="grid gap-2 content-start">
                                <Label htmlFor="start_at">{t("start_at", "Start At")}</Label>
                                <Controller
                                    control={scheduledForm.control}
                                    name="start_at"
                                    render={({ field }) => (
                                        <Input
                                            id="start_at"
                                            type="datetime-local"
                                            value={field.value || ""}
                                            onChange={(e) =>
                                                field.onChange(
                                                    e.target.value
                                                        ? new Date(e.target.value).toISOString()
                                                        : "",
                                                )
                                            }
                                        />
                                    )}
                                />
                                <p className="text-xs text-muted-foreground">
                                    {t("start_at_help", "Start time for recurring schedules")}
                                </p>
                            </div>
                        </div>
                        <div className="grid gap-2">
                            <Label>{t("data", "Data")}</Label>
                            <AttributeEditor
                                value={newScheduledData}
                                onChange={setNewScheduledData}
                                emptyTitle={t("no_data", "No data")}
                                emptyDescription={t(
                                    "no_data_description_scheduled",
                                    "Add custom data to this scheduled item.",
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
                        <Button
                            onClick={scheduledForm.handleSubmit(createScheduled)}
                            disabled={isCreating}
                        >
                            {isCreating ? t("creating") : t("create")}
                        </Button>
                    </DialogFooter>
                </DialogContent>
            </Dialog>

            {/* Delete Confirmation Dialog */}
            <Dialog
                open={deleteTarget !== null}
                onOpenChange={(open) => {
                    if (!open) setDeleteTarget(null)
                }}
            >
                <DialogContent>
                    <DialogHeader>
                        <DialogTitle>{t("delete_scheduled", "Delete Schedule")}</DialogTitle>
                        <DialogDescription>
                            {t(
                                "delete_scheduled_confirmation",
                                `Are you sure you want to delete "${deleteTarget ? (schemaMap.get(deleteTarget.scheduled_id) ?? deleteTarget.scheduled_id) : ""}"? This action cannot be undone.`,
                            )}
                        </DialogDescription>
                    </DialogHeader>
                    <DialogFooter>
                        <Button variant="ghost" onClick={() => setDeleteTarget(null)}>
                            {t("cancel")}
                        </Button>
                        <Button variant="destructive" onClick={confirmDelete}>
                            {t("delete")}
                        </Button>
                    </DialogFooter>
                </DialogContent>
            </Dialog>

            {/* Pause Dialog */}
            <Dialog
                open={pauseTarget !== null}
                onOpenChange={(open) => {
                    if (!open) setPauseTarget(null)
                }}
            >
                <DialogContent>
                    <DialogHeader>
                        <DialogTitle>{t("pause_scheduled", "Pause Schedule")}</DialogTitle>
                        <DialogDescription>
                            {t("pause_scheduled_description", "Choose how to pause this schedule.")}
                        </DialogDescription>
                    </DialogHeader>
                    <div className="py-4">
                        <Label className="mb-2 block">{t("pause_mode", "Pause mode")}</Label>
                        <Select
                            value={pauseMode}
                            onValueChange={(v) =>
                                setPauseMode(v as "immediately" | "after_next_interval")
                            }
                        >
                            <SelectTrigger>
                                <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                                <SelectItem value="immediately">
                                    {t("pause_immediately", "Immediately")}
                                </SelectItem>
                                <SelectItem value="after_next_interval">
                                    {t("pause_after_next_interval", "After next interval")}
                                </SelectItem>
                            </SelectContent>
                        </Select>
                        <p className="text-xs text-muted-foreground mt-2">
                            {pauseMode === "immediately"
                                ? t(
                                      "pause_immediately_help",
                                      "Deletes all pending scheduled events right away.",
                                  )
                                : t(
                                      "pause_after_next_interval_help",
                                      "Keeps existing scheduled events but prevents advancement to the next occurrence.",
                                  )}
                        </p>
                    </div>
                    <DialogFooter>
                        <Button variant="ghost" onClick={() => setPauseTarget(null)}>
                            {t("cancel")}
                        </Button>
                        <Button onClick={confirmPause}>{t("pause", "Pause")}</Button>
                    </DialogFooter>
                </DialogContent>
            </Dialog>

            {/* Resume Dialog */}
            <Dialog
                open={resumeTarget !== null}
                onOpenChange={(open) => {
                    if (!open) setResumeTarget(null)
                }}
            >
                <DialogContent>
                    <DialogHeader>
                        <DialogTitle>{t("resume_scheduled", "Resume Schedule")}</DialogTitle>
                        <DialogDescription>
                            {t(
                                "resume_scheduled_description",
                                "Choose how to resume this schedule.",
                            )}
                        </DialogDescription>
                    </DialogHeader>
                    <div className="py-4">
                        <Label className="mb-2 block">{t("resume_mode", "Resume mode")}</Label>
                        <Select
                            value={resumeMode}
                            onValueChange={(v) =>
                                setResumeMode(v as "immediately" | "at_next_interval")
                            }
                        >
                            <SelectTrigger>
                                <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                                <SelectItem value="immediately">
                                    {t("resume_immediately", "Immediately")}
                                </SelectItem>
                                <SelectItem value="at_next_interval">
                                    {t("resume_at_next_interval", "At next interval")}
                                </SelectItem>
                            </SelectContent>
                        </Select>
                        <p className="text-xs text-muted-foreground mt-2">
                            {resumeMode === "immediately"
                                ? t(
                                      "resume_immediately_help",
                                      "Resumes and recomputes the next occurrence from now.",
                                  )
                                : t(
                                      "resume_at_next_interval_help",
                                      "Resumes and recomputes the next natural interval from the existing anchor.",
                                  )}
                        </p>
                    </div>
                    <DialogFooter>
                        <Button variant="ghost" onClick={() => setResumeTarget(null)}>
                            {t("cancel")}
                        </Button>
                        <Button onClick={confirmResume}>{t("resume", "Resume")}</Button>
                    </DialogFooter>
                </DialogContent>
            </Dialog>
        </div>
    )
}
