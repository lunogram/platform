import { useCallback, useContext, useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
import { Search, Zap, MoreHorizontal, Trash2, ChevronRight } from "lucide-react"
import { ProjectContext } from "../../contexts"
import { useResolver } from "../../hooks"
import oapiClient from "../../oapi/client"
import { client } from "../../api"
import type { components } from "../../oapi/management.generated"

import { Input } from "@/components/ui/input"
import {
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from "@/components/ui/table"
import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from "@/components/ui/dialog"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { Badge } from "@/components/ui/badge"
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/components/ui/collapsible"

type EventWithSchema = components["schemas"]["EventWithSchema"]

interface EventSchemaItem extends EventWithSchema {
    subject_type: "user" | "organization" | "scheduled"
}

export default function EventSchemas() {
    const { t } = useTranslation()
    const [project] = useContext(ProjectContext)
    const [searchQuery, setSearchQuery] = useState("")
    const [expandedRows, setExpandedRows] = useState<Set<string>>(new Set())
    const [deleteTarget, setDeleteTarget] = useState<EventSchemaItem | null>(null)

    const toggleRow = (id: string) => {
        setExpandedRows((prev) => {
            const next = new Set(prev)
            if (next.has(id)) {
                next.delete(id)
            } else {
                next.add(id)
            }
            return next
        })
    }

    const [userResult, , reloadUser] = useResolver(
        useCallback(async () => {
            const { data } = await oapiClient.GET(
                "/api/admin/projects/{projectID}/subjects/user/events/schema",
                { params: { path: { projectID: project.id } } },
            )
            return data
        }, [project.id]),
    )

    const [orgResult, , reloadOrg] = useResolver(
        useCallback(async () => {
            const { data } = await oapiClient.GET(
                "/api/admin/projects/{projectID}/subjects/organization/events/schema",
                { params: { path: { projectID: project.id } } },
            )
            return data
        }, [project.id]),
    )

    const [scheduledResult, , reloadScheduled] = useResolver(
        useCallback(async () => {
            try {
                const { data } = await client.get<{ results: EventWithSchema[] }>(
                    `/admin/projects/${project.id}/subjects/user/scheduled/schema`,
                )
                return data
            } catch {
                return { results: [] as EventWithSchema[] }
            }
        }, [project.id]),
    )

    const reload = useCallback(async () => {
        await Promise.all([reloadUser(), reloadOrg(), reloadScheduled()])
    }, [reloadUser, reloadOrg, reloadScheduled])

    const isLoading = !userResult && !orgResult && !scheduledResult

    const events = useMemo(() => {
        const userEvents: EventSchemaItem[] = (userResult?.results ?? []).map((e) => ({
            ...e,
            subject_type: "user" as const,
        }))
        const orgEvents: EventSchemaItem[] = (orgResult?.results ?? []).map((e) => ({
            ...e,
            subject_type: "organization" as const,
        }))
        const scheduledEvents: EventSchemaItem[] = (scheduledResult?.results ?? []).map((e) => ({
            ...e,
            subject_type: "scheduled" as const,
        }))
        const all = [...userEvents, ...orgEvents, ...scheduledEvents].sort((a, b) =>
            a.name.localeCompare(b.name),
        )
        if (!searchQuery) return all
        const query = searchQuery.toLowerCase()
        return all.filter(
            (e) =>
                e.name.toLowerCase().includes(query) ||
                e.subject_type.toLowerCase().includes(query),
        )
    }, [userResult, orgResult, scheduledResult, searchQuery])

    const handleDelete = async (e: React.MouseEvent, event: EventSchemaItem) => {
        e.stopPropagation()
        setDeleteTarget(event)
    }

    const confirmDelete = async () => {
        if (!deleteTarget) return
        const event = deleteTarget
        setDeleteTarget(null)
        if (event.subject_type === "user") {
            await oapiClient.DELETE(
                "/api/admin/projects/{projectID}/subjects/user/events/schema/{eventID}",
                {
                    params: {
                        path: { projectID: project.id, eventID: event.id },
                    },
                },
            )
        } else if (event.subject_type === "organization") {
            await oapiClient.DELETE(
                "/api/admin/projects/{projectID}/subjects/organization/events/schema/{eventID}",
                {
                    params: {
                        path: { projectID: project.id, eventID: event.id },
                    },
                },
            )
        } else {
            await client.delete(
                `/admin/projects/${project.id}/subjects/user/scheduled/schema/${event.id}`,
            )
        }
        await reload()
    }

    return (
        <div className="flex flex-col gap-6">
            {/* Header */}
            <div>
                <h2 className="text-2xl font-semibold tracking-tight">{t("schemas", "Schemas")}</h2>
                <p className="text-sm text-muted-foreground mt-1">
                    {t(
                        "schemas_description",
                        "Schemas are automatically discovered based on the events and scheduled data published into the platform.",
                    )}
                </p>
            </div>

            {/* Search */}
            <div className="flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-3 sm:gap-4">
                <div className="relative sm:max-w-sm flex-1">
                    <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                    <Input
                        placeholder={t("search")}
                        value={searchQuery}
                        onChange={(e) => setSearchQuery(e.target.value)}
                        className="pl-9"
                    />
                </div>
            </div>

            {/* Table */}
            <div className="rounded-lg border bg-card">
                <Table>
                    <TableHeader>
                        <TableRow>
                            <TableHead className="w-[30px]" />
                            <TableHead>{t("name")}</TableHead>
                            <TableHead>{t("type", "Type")}</TableHead>
                            <TableHead className="hidden sm:table-cell">
                                {t("fields", "Fields")}
                            </TableHead>
                            <TableHead className="w-[70px]" />
                        </TableRow>
                    </TableHeader>
                    <TableBody>
                        {isLoading ? (
                            Array.from({ length: 3 }).map((_, i) => (
                                <TableRow key={i}>
                                    <TableCell />
                                    <TableCell>
                                        <Skeleton className="h-4 w-28" />
                                    </TableCell>
                                    <TableCell>
                                        <Skeleton className="h-4 w-16" />
                                    </TableCell>
                                    <TableCell className="hidden sm:table-cell">
                                        <Skeleton className="h-4 w-10" />
                                    </TableCell>
                                    <TableCell>
                                        <Skeleton className="h-4 w-8" />
                                    </TableCell>
                                </TableRow>
                            ))
                        ) : events.length === 0 ? (
                            <TableRow>
                                <TableCell colSpan={5} className="h-32 text-center">
                                    <div className="flex flex-col items-center gap-2 text-muted-foreground">
                                        <Zap className="h-8 w-8" />
                                        <p>
                                            {searchQuery
                                                ? t("no_results")
                                                : t(
                                                      "no_schemas_yet",
                                                      "No schemas discovered yet. Schemas are automatically created when events or scheduled data are received.",
                                                  )}
                                        </p>
                                    </div>
                                </TableCell>
                            </TableRow>
                        ) : (
                            events.map((event) => {
                                const isExpanded = expandedRows.has(event.id)
                                return (
                                    <Collapsible
                                        key={event.id}
                                        asChild
                                        open={isExpanded}
                                        onOpenChange={() => toggleRow(event.id)}
                                    >
                                        <>
                                            <CollapsibleTrigger asChild>
                                                <TableRow className="cursor-pointer">
                                                    <TableCell className="w-[30px] pr-0">
                                                        <ChevronRight
                                                            className={`h-4 w-4 text-muted-foreground transition-transform ${isExpanded ? "rotate-90" : ""}`}
                                                        />
                                                    </TableCell>
                                                    <TableCell className="font-medium">
                                                        {event.name}
                                                    </TableCell>
                                                    <TableCell>
                                                        <Badge variant="secondary">
                                                            {event.subject_type}
                                                        </Badge>
                                                    </TableCell>
                                                    <TableCell className="hidden sm:table-cell text-muted-foreground">
                                                        {event.schema.length}
                                                    </TableCell>
                                                    <TableCell>
                                                        <DropdownMenu>
                                                            <DropdownMenuTrigger asChild>
                                                                <Button
                                                                    variant="ghost"
                                                                    className="h-8 w-8 p-0"
                                                                    onClick={(e) =>
                                                                        e.stopPropagation()
                                                                    }
                                                                    aria-label={t("options")}
                                                                >
                                                                    <MoreHorizontal className="h-4 w-4" />
                                                                </Button>
                                                            </DropdownMenuTrigger>
                                                            <DropdownMenuContent align="end">
                                                                <DropdownMenuItem
                                                                    onClick={(e) =>
                                                                        handleDelete(e, event)
                                                                    }
                                                                    className="text-destructive focus:text-destructive"
                                                                >
                                                                    <Trash2 className="mr-2 h-4 w-4" />
                                                                    {t("delete")}
                                                                </DropdownMenuItem>
                                                            </DropdownMenuContent>
                                                        </DropdownMenu>
                                                    </TableCell>
                                                </TableRow>
                                            </CollapsibleTrigger>
                                            <CollapsibleContent asChild>
                                                <tr>
                                                    <td colSpan={5} className="p-0">
                                                        <div className="border-t bg-muted/30 px-6 py-4">
                                                            {event.schema.length === 0 ? (
                                                                <p className="text-sm text-muted-foreground">
                                                                    {t(
                                                                        "no_schema_fields",
                                                                        "No fields discovered yet.",
                                                                    )}
                                                                </p>
                                                            ) : (
                                                                <div className="rounded-md border bg-card">
                                                                    <table className="w-full text-sm">
                                                                        <thead>
                                                                            <tr className="border-b">
                                                                                <th className="px-4 py-2 text-left font-medium text-muted-foreground">
                                                                                    {t(
                                                                                        "path",
                                                                                        "Path",
                                                                                    )}
                                                                                </th>
                                                                                <th className="px-4 py-2 text-left font-medium text-muted-foreground">
                                                                                    {t(
                                                                                        "types",
                                                                                        "Types",
                                                                                    )}
                                                                                </th>
                                                                            </tr>
                                                                        </thead>
                                                                        <tbody>
                                                                            {event.schema.map(
                                                                                (field) => (
                                                                                    <tr
                                                                                        key={
                                                                                            field.path
                                                                                        }
                                                                                        className="border-b last:border-0"
                                                                                    >
                                                                                        <td className="px-4 py-2 font-mono text-xs">
                                                                                            {
                                                                                                field.path
                                                                                            }
                                                                                        </td>
                                                                                        <td className="px-4 py-2">
                                                                                            <div className="flex gap-1.5 flex-wrap">
                                                                                                {field.types.map(
                                                                                                    (
                                                                                                        type,
                                                                                                    ) => (
                                                                                                        <Badge
                                                                                                            key={
                                                                                                                type
                                                                                                            }
                                                                                                            variant="outline"
                                                                                                            className="text-xs font-mono"
                                                                                                        >
                                                                                                            {
                                                                                                                type
                                                                                                            }
                                                                                                        </Badge>
                                                                                                    ),
                                                                                                )}
                                                                                            </div>
                                                                                        </td>
                                                                                    </tr>
                                                                                ),
                                                                            )}
                                                                        </tbody>
                                                                    </table>
                                                                </div>
                                                            )}
                                                        </div>
                                                    </td>
                                                </tr>
                                            </CollapsibleContent>
                                        </>
                                    </Collapsible>
                                )
                            })
                        )}
                    </TableBody>
                </Table>

                {events.length > 0 && (
                    <div className="flex items-center justify-between border-t px-4 py-3">
                        <p className="text-sm text-muted-foreground">
                            {events.length}{" "}
                            {events.length === 1 ? t("schema", "schema") : t("schemas", "schemas")}
                        </p>
                    </div>
                )}
            </div>

            {/* Delete Confirmation Dialog */}
            <Dialog
                open={deleteTarget !== null}
                onOpenChange={(open) => {
                    if (!open) setDeleteTarget(null)
                }}
            >
                <DialogContent>
                    <DialogHeader>
                        <DialogTitle>{t("delete_schema", "Delete Schema")}</DialogTitle>
                        <DialogDescription>
                            {t(
                                "delete_schema_confirmation",
                                `Delete schema "${deleteTarget?.name}"?`,
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
        </div>
    )
}
