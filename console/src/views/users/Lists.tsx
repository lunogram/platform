import { useCallback, useState, useRef } from "react"
import { useNavigate, useParams } from "react-router"
import { useTranslation } from "react-i18next"
import { useQuery } from "@tanstack/react-query"
import {
    Plus,
    Search,
    ChevronLeft,
    ChevronRight,
    ArrowRight,
    ListFilter,
    Users,
    Settings,
    ArchiveRestore,
} from "lucide-react"
import { NIL } from "uuid"

import { ListCreateForm } from "./ListCreateForm"
import { ListRow } from "./ListRow"
import { LoadErrorRow } from "./LoadErrorRow"
import {
    LIST_PAGE_SIZE,
    listPageQuery,
    useArchiveList,
    useDuplicateList,
    useUnarchiveList,
} from "./listQueries"
import { ListsIcon as ListsPageIcon } from "@/components/icons"

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

export default function Lists() {
    const { projectId = NIL as UUID } = useParams<{ projectId: UUID }>()
    const { t } = useTranslation()
    const navigate = useNavigate()
    const duplicateList = useDuplicateList()
    const archiveList = useArchiveList()
    const unarchiveList = useUnarchiveList()

    const pageSize = LIST_PAGE_SIZE
    const [searchQuery, setSearchQuery] = useState("")
    const [debouncedQuery, setDebouncedQuery] = useState("")
    const [isCreateOpen, setIsCreateOpen] = useState(false)
    const [offset, setOffset] = useState(0)
    const [archivedOffset, setArchivedOffset] = useState(0)
    const [showArchived, setShowArchived] = useState(false)
    const searchTimeoutRef = useRef<ReturnType<typeof setTimeout> | undefined>(undefined)

    const handleSearch = useCallback((value: string) => {
        setSearchQuery(value)
        setOffset(0)
        setArchivedOffset(0)
        clearTimeout(searchTimeoutRef.current)
        searchTimeoutRef.current = setTimeout(() => {
            setDebouncedQuery(value)
        }, 300)
    }, [])

    const isArchivedView = showArchived
    const currentOffset = isArchivedView ? archivedOffset : offset
    const result = useQuery(
        listPageQuery(projectId, {
            offset: currentOffset,
            search: debouncedQuery,
            archived: isArchivedView,
        }),
    )
    const lists = result.data?.results ?? []
    const total = result.data?.total ?? 0
    const hasNextPage = currentOffset + pageSize < total
    const hasPrevPage = currentOffset > 0

    const handleNextPage = () => {
        const setter = isArchivedView ? setArchivedOffset : setOffset
        setter((prev) => prev + pageSize)
    }

    const handlePrevPage = () => {
        const setter = isArchivedView ? setArchivedOffset : setOffset
        setter((prev) => Math.max(0, prev - pageSize))
    }

    const handleRowClick = (id: UUID) => {
        navigate(id.toString())
    }

    const handleDuplicateList = (e: React.MouseEvent, id: UUID) => {
        e.stopPropagation()
        duplicateList.mutate(id, { onSuccess: (list) => navigate(list.id.toString()) })
    }

    const handleArchiveList = (e: React.MouseEvent, id: UUID) => {
        e.stopPropagation()
        archiveList.mutate(id)
    }

    const handleUnarchiveList = (id: UUID) => {
        unarchiveList.mutate(id, {
            onSuccess: () => {
                setShowArchived(false)
                setArchivedOffset(0)
            },
        })
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
                        variant={showArchived ? "secondary" : "ghost"}
                        size="sm"
                        className="h-8 w-8 p-0"
                        aria-label={t("show_archived", "Show archived")}
                        aria-pressed={showArchived}
                        onClick={() => {
                            setArchivedOffset(0)
                            setShowArchived((prev) => !prev)
                        }}
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
                        {result.isPending ? (
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
                        ) : result.isError ? (
                            <LoadErrorRow
                                colSpan={7}
                                message={t("lists_load_failed", "Couldn't load lists.")}
                                onRetry={() => result.refetch()}
                            />
                        ) : lists.length === 0 ? (
                            <TableRow>
                                <TableCell colSpan={7} className="h-32 text-center">
                                    <div className="flex flex-col items-center gap-2 text-muted-foreground">
                                        <ListFilter className="h-8 w-8" />
                                        <p>
                                            {isArchivedView
                                                ? t("no_archived_lists", "No archived lists")
                                                : debouncedQuery
                                                  ? t("no_lists_found", "No lists found")
                                                  : t("no_lists_yet", "No lists yet")}
                                        </p>
                                        {!debouncedQuery && !isArchivedView && (
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
                            lists.map((list) => (
                                <ListRow
                                    key={list.id}
                                    list={list}
                                    archived={isArchivedView}
                                    onOpen={() => handleRowClick(list.id)}
                                    onDuplicate={(e) => handleDuplicateList(e, list.id)}
                                    onArchive={(e) => handleArchiveList(e, list.id)}
                                    onUnarchive={() => handleUnarchiveList(list.id)}
                                />
                            ))
                        )}
                    </TableBody>
                </Table>

                {/* Pagination */}
                {lists.length > 0 && (
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
