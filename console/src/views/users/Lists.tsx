import { useCallback, useContext, useState, useRef } from "react"
import { useNavigate, useParams } from "react-router"
import { useTranslation } from "react-i18next"
import {
    Plus,
    Search,
    ChevronLeft,
    ChevronRight,
    ArrowRight,
    ListFilter,
    Users,
    Settings,
    MoreHorizontal,
    Copy,
    Archive,
    ArchiveRestore,
} from "lucide-react"
import { NIL } from "uuid"

import api from "../../api"
import { oapiClient } from "@/oapi/client"
import { useResolver } from "../../hooks"
import { formatDate, snakeToTitle } from "../../utils"
import { getRandomColor } from "@/lib/colors"
import { PreferencesContext } from "@/contexts/PreferencesContext"
import { ListCreateForm } from "./ListCreateForm"
import { ListsIcon as ListsPageIcon } from "@/components/icons"

import type { List, ListState } from "../../types"
import type { UUID } from "@/types/common"

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
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogHeader,
    DialogTitle,
} from "@/components/ui/dialog"
import { Skeleton } from "@/components/ui/skeleton"
import { Badge } from "@/components/ui/badge"
import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"

function getStateBadge(state: ListState, t: (key: string) => string) {
    const config: Record<ListState, { label: string; className: string }> = {
        draft: { label: t("draft"), className: "bg-secondary text-secondary-foreground" },
        loading: { label: t("loading"), className: "bg-blue-100 text-blue-700" },
        ready: { label: t("ready"), className: "bg-green-100 text-green-700" },
    }
    const { label, className } = config[state] ?? config.draft
    return (
        <Badge variant="outline" className={`border-0 ${className}`}>
            {label}
        </Badge>
    )
}

export default function Lists() {
    const { projectId = NIL as UUID } = useParams<{ projectId: UUID }>()
    const [preferences] = useContext(PreferencesContext)
    const { t } = useTranslation()
    const navigate = useNavigate()

    const pageSize = 15
    const [searchQuery, setSearchQuery] = useState("")
    const [debouncedQuery, setDebouncedQuery] = useState("")
    const [isCreateOpen, setIsCreateOpen] = useState(false)
    const [offset, setOffset] = useState(0)
    const [archivedOpen, setArchivedOpen] = useState(false)
    const searchTimeoutRef = useRef<ReturnType<typeof setTimeout> | undefined>(undefined)

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
            return await api.lists.search(projectId, {
                limit: pageSize,
                offset,
                search: debouncedQuery || undefined,
            })
        }, [projectId, debouncedQuery, offset]),
    )

    const [archivedResult, , reloadArchived] = useResolver(
        useCallback(async () => {
            if (!archivedOpen) return null
            const response = await oapiClient.GET("/api/admin/projects/{projectID}/lists", {
                params:
                 { 
                    path: { 
                        projectID: projectId 
                    }, 
                    query: { 
                        limit: 100, 
                        include_deleted: true 
                    } 
                },
            })
            if (response.error || !response.data) return null
            return response.data.results?.filter((l) => l.archived) ?? []
        }, [projectId, archivedOpen]),
    )

    const lists = result?.results
    const total = result?.total ?? 0
    const hasNextPage = offset + pageSize < total
    const hasPrevPage = offset > 0

    const handleNextPage = () => {
        setOffset((prev) => prev + pageSize)
    }

    const handlePrevPage = () => {
        setOffset((prev) => Math.max(0, prev - pageSize))
    }

    const handleRowClick = (list: List) => {
        navigate(list.id.toString())
    }

    const handleDuplicateList = async (e: React.MouseEvent, id: UUID) => {
        e.stopPropagation()
        const list = await api.lists.duplicate(projectId, id)
        await navigate(list.id.toString())
    }

    const handleArchiveList = async (e: React.MouseEvent, id: UUID) => {
        e.stopPropagation()
        await api.lists.delete(projectId, id)
        await reload()
    }

    const handleUnarchiveList = async (id: UUID) => {
        await oapiClient.POST("/api/admin/projects/{projectID}/lists/{listID}/unarchive", {
            params: { 
                path: { 
                    projectID: projectId, 
                    listID: id 
                } 
            },
        })
        await Promise.all([reload(), reloadArchived()])
    }

    return (
        <div className="flex flex-col gap-4 p-4 sm:gap-6 sm:p-6">
            {/* Header */}
            <div className="flex items-start gap-4">
                <div className="flex h-14 w-14 items-center justify-center rounded-xl shrink-0 bg-muted [&>svg]:h-7 [&>svg]:w-7 [&>svg]:text-muted-foreground">
                    <ListsPageIcon />
                </div>
                <div className="space-y-1">
                    <h1 className="text-2xl font-semibold tracking-tight">{t("lists")}</h1>
                    <p className="text-sm text-muted-foreground">
                        {t(
                            "lists_description",
                            "Create dynamic and static lists to segment and target your audience.",
                        )}
                    </p>
                </div>
            </div>

            {/* Search and Actions */}
            <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between sm:gap-4">
                <div className="relative sm:max-w-sm flex-1">
                    <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                    <Input
                        placeholder={t("search_lists", "Search lists...")}
                        aria-label={t("search_lists", "Search lists")}
                        value={searchQuery}
                        onChange={(e) => handleSearch(e.target.value)}
                        className="pl-9"
                    />
                </div>
                <div className="flex w-full items-center gap-2 sm:w-auto">
                    <Button
                        variant="ghost"
                        size="sm"
                        className="h-8 w-8 p-0"
                        aria-label={t("show_archived", "Show archived")}
                        onClick={() => setArchivedOpen(true)}
                    >
                        <ArchiveRestore className="h-4 w-4" />
                    </Button>
                    <Button
                        onClick={() => setIsCreateOpen(true)}
                        className="flex-1 sm:flex-initial"
                        aria-label={t("create_list_from_header", "Create List from header")}
                    >
                        <Plus className="mr-2 h-4 w-4" />
                        {t("create_list")}
                    </Button>
                </div>
            </div>

            {/* Table */}
            <div className="rounded-lg border bg-card">
                <Table>
                    <TableHeader>
                        <TableRow>
                            <TableHead>{t("name")}</TableHead>
                            <TableHead className="hidden sm:table-cell">{t("type")}</TableHead>
                            <TableHead className="hidden sm:table-cell">
                                {t("users_count")}
                            </TableHead>
                            <TableHead>{t("state")}</TableHead>
                            <TableHead className="hidden md:table-cell">
                                {t("created_at")}
                            </TableHead>
                            <TableHead className="hidden md:table-cell">
                                {t("updated_at")}
                            </TableHead>
                            <TableHead className="w-[50px]"></TableHead>
                        </TableRow>
                    </TableHeader>
                    <TableBody>
                        {!lists ? (
                            Array.from({ length: 5 }).map((_, i) => (
                                <TableRow key={i}>
                                    <TableCell>
                                        <div className="flex items-center gap-3">
                                            <Skeleton className="h-8 w-8 rounded-md" />
                                            <Skeleton className="h-4 w-36" />
                                        </div>
                                    </TableCell>
                                    <TableCell className="hidden sm:table-cell">
                                        <Skeleton className="h-4 w-16" />
                                    </TableCell>
                                    <TableCell className="hidden sm:table-cell">
                                        <Skeleton className="h-4 w-16" />
                                    </TableCell>
                                    <TableCell>
                                        <Skeleton className="h-5 w-16 rounded-md" />
                                    </TableCell>
                                    <TableCell className="hidden md:table-cell">
                                        <Skeleton className="h-4 w-28" />
                                    </TableCell>
                                    <TableCell className="hidden md:table-cell">
                                        <Skeleton className="h-4 w-28" />
                                    </TableCell>
                                    <TableCell>
                                        <Skeleton className="h-4 w-8" />
                                    </TableCell>
                                </TableRow>
                            ))
                        ) : lists.length === 0 ? (
                            <TableRow>
                                <TableCell colSpan={7} className="h-32 text-center">
                                    <div className="flex flex-col items-center gap-2 text-muted-foreground">
                                        <ListFilter className="h-8 w-8" />
                                        <p>
                                            {debouncedQuery
                                                ? t("no_lists_found", "No lists found")
                                                : t("no_lists_yet", "No lists yet")}
                                        </p>
                                        {!debouncedQuery && (
                                            <Button
                                                variant="outline"
                                                size="sm"
                                                onClick={() => setIsCreateOpen(true)}
                                                className="mt-2"
                                                aria-label={t(
                                                    "create_list_from_empty",
                                                    "Create List from empty state",
                                                )}
                                            >
                                                <Plus className="mr-2 h-4 w-4" />
                                                {t("create_list")}
                                            </Button>
                                        )}
                                    </div>
                                </TableCell>
                            </TableRow>
                        ) : (
                            lists.map((list) => {
                                const listColor = getRandomColor(list.name ?? list.id)
                                return (
                                    <TableRow
                                        key={list.id}
                                        className="cursor-pointer"
                                        onClick={() => handleRowClick(list)}
                                    >
                                        <TableCell>
                                            <div className="flex items-center gap-3">
                                                <div
                                                    className="flex h-8 w-8 items-center justify-center rounded-md shrink-0"
                                                    style={{ backgroundColor: listColor }}
                                                >
                                                    <ListFilter className="h-4 w-4 text-white" />
                                                </div>
                                                <div className="font-medium">{list.name}</div>
                                            </div>
                                        </TableCell>
                                        <TableCell className="hidden sm:table-cell text-muted-foreground">
                                            {snakeToTitle(list.type)}
                                        </TableCell>
                                        <TableCell className="hidden sm:table-cell text-muted-foreground">
                                            {list.users_count?.toLocaleString() ?? "—"}
                                        </TableCell>
                                        <TableCell>{getStateBadge(list.state, t)}</TableCell>
                                        <TableCell className="hidden md:table-cell text-muted-foreground">
                                            {formatDate(preferences, list.created_at, "PP")}
                                        </TableCell>
                                        <TableCell className="hidden md:table-cell text-muted-foreground">
                                            {formatDate(preferences, list.updated_at, "PP")}
                                        </TableCell>
                                        <TableCell>
                                            <DropdownMenu>
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
                                                    <DropdownMenuItem
                                                        onClick={(e) => {
                                                            e.stopPropagation()
                                                            handleRowClick(list)
                                                        }}
                                                    >
                                                        <ListFilter className="mr-2 h-4 w-4" />
                                                        {t("edit")}
                                                    </DropdownMenuItem>
                                                    <DropdownMenuItem
                                                        onClick={(e) =>
                                                            handleDuplicateList(e, list.id)
                                                        }
                                                    >
                                                        <Copy className="mr-2 h-4 w-4" />
                                                        {t("duplicate")}
                                                    </DropdownMenuItem>
                                                    <DropdownMenuItem
                                                        onClick={(e) =>
                                                            handleArchiveList(e, list.id)
                                                        }
                                                        className="text-destructive"
                                                    >
                                                        <Archive className="mr-2 h-4 w-4" />
                                                        {t("archive")}
                                                    </DropdownMenuItem>
                                                </DropdownMenuContent>
                                            </DropdownMenu>
                                        </TableCell>
                                    </TableRow>
                                )
                            })
                        )}
                    </TableBody>
                </Table>

                {/* Pagination */}
                {lists && lists.length > 0 && (
                    <div className="flex items-center justify-between border-t px-4 py-3">
                        <p className="text-sm text-muted-foreground">
                            {total} {t("lists").toLowerCase()}
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
            <div className="group relative overflow-hidden rounded-lg bg-gradient-to-br from-primary/10 via-primary/5 to-transparent border p-6">
                <div className="relative z-10 max-w-md">
                    <h3 className="font-semibold text-foreground">
                        {t("list_tip_title", "Segment your audience")}
                    </h3>
                    <p className="mt-1 text-sm text-muted-foreground">
                        {t(
                            "list_tip_description",
                            "Create dynamic and static lists to target the right users with personalized campaigns.",
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
                        <ListFilter
                            className="h-10 w-10 text-primary/40 transition-all duration-500 group-hover:text-primary/60 group-hover:scale-110"
                            strokeWidth={1.25}
                        />
                    </div>
                    <div className="flex h-20 w-20 items-center justify-center rounded-xl bg-primary/10 -rotate-6 translate-y-4 transition-all duration-500 ease-out delay-75 group-hover:rotate-3 group-hover:translate-y-0 group-hover:bg-primary/15">
                        <Users
                            className="h-10 w-10 text-primary/40 transition-all duration-500 delay-75 group-hover:text-primary/60 group-hover:scale-110"
                            strokeWidth={1.25}
                        />
                    </div>
                    <div className="flex h-20 w-20 items-center justify-center rounded-xl bg-primary/10 rotate-12 -translate-y-2 transition-all duration-500 ease-out delay-150 group-hover:-rotate-6 group-hover:-translate-y-4 group-hover:bg-primary/15">
                        <Settings
                            className="h-10 w-10 text-primary/40 transition-all duration-500 delay-150 group-hover:text-primary/60 group-hover:scale-110 group-hover:rotate-90"
                            strokeWidth={1.25}
                        />
                    </div>
                </div>
            </div>

            <Dialog open={archivedOpen} onOpenChange={setArchivedOpen}>
                <DialogContent className="max-w-xl p-0 overflow-hidden">
                    <DialogHeader className="gap-2 border-b bg-muted/30 px-6 py-4">
                        <div className="flex items-center justify-between gap-3">
                            <div className="flex items-center gap-3">
                                <div className="flex h-9 w-9 items-center justify-center rounded-md bg-primary/10 text-primary">
                                    <Archive className="h-4 w-4" />
                                </div>
                                <DialogTitle>{t("archived_lists", "Archived lists")}</DialogTitle>
                            </div>
                        </div>
                        <DialogDescription>
                            {t("archived_lists_description", "Restore a list to make it active again.")}
                        </DialogDescription>
                    </DialogHeader>
                    <div className="max-h-[60vh] overflow-y-auto px-6 py-4">
                        {!archivedResult ? (
                            <div className="flex flex-col items-center gap-2 py-8 text-sm text-muted-foreground">
                                <div className="flex h-10 w-10 items-center justify-center rounded-full bg-muted">
                                    <ArchiveRestore className="h-5 w-5" />
                                </div>
                                <p>{t("loading", "Loading...")}</p>
                            </div>
                        ) : archivedResult.length === 0 ? (
                            <div className="flex flex-col items-center gap-2 py-8 text-sm text-muted-foreground">
                                <div className="flex h-10 w-10 items-center justify-center rounded-full bg-muted">
                                    <ArchiveRestore className="h-5 w-5" />
                                </div>
                                <p>{t("no_archived_lists", "No archived lists")}</p>
                            </div>
                        ) : (
                            <div className="space-y-1">
                                {archivedResult.map((list) => {
                                    const listColor = getRandomColor(list.name ?? list.id)
                                    const archivedAt =
                                        ("deleted_at" in list
                                        ? (list as { deleted_at?: string }).deleted_at
                                        : undefined) ?? list.updated_at
                                    return (
                                        <div
                                            key={list.id}
                                            className="group flex items-center gap-3 rounded-md border border-transparent px-2 py-2 transition hover:border-border hover:bg-muted/40"
                                        >
                                            <div
                                                className="flex h-9 w-9 items-center justify-center rounded-md shrink-0"
                                                style={{ backgroundColor: listColor }}
                                            >
                                                <ListFilter className="h-4 w-4 text-white" />
                                            </div>
                                            <div className="flex-1 min-w-0">
                                                <div className="text-sm font-medium truncate">{list.name}</div>
                                                <div className="text-xs text-muted-foreground">
                                                    {t("archived_on", "Archived on")} {formatDate(preferences, archivedAt, "PP")}
                                                </div>
                                            </div>
                                            <Button
                                                variant="ghost"
                                                size="icon"
                                                className="shrink-0 text-primary hover:text-primary"
                                                onClick={() => handleUnarchiveList(list.id)}
                                                aria-label={t("unarchive", "Unarchive")}
                                            >
                                                <ArchiveRestore className="h-4 w-4" />
                                            </Button>
                                        </div>
                                    )
                                })}
                            </div>
                        )}
                    </div>
                </DialogContent>
            </Dialog>

            {/* Create List Dialog */}
            <Dialog open={isCreateOpen} onOpenChange={setIsCreateOpen}>
                <DialogContent>
                    <DialogHeader>
                        <DialogTitle>{t("create_list")}</DialogTitle>
                        <DialogDescription>
                            {t(
                                "create_list_description",
                                "Create a new list to segment and target your users.",
                            )}
                        </DialogDescription>
                    </DialogHeader>
                    <ListCreateForm
                        onCreated={async (list) => {
                            setIsCreateOpen(false)
                            await navigate(list.id.toString())
                        }}
                    />
                </DialogContent>
            </Dialog>
        </div>
    )
}
