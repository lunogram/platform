import { useCallback, useContext, useState, useRef } from "react"
import { useNavigate } from "react-router"
import { useTranslation } from "react-i18next"
import { useQuery } from "@tanstack/react-query"
import { Plus, Search, ChevronLeft, ChevronRight, GitBranch, ArchiveRestore } from "lucide-react"

import { ProjectContext } from "../../contexts"
import { JourneyForm } from "./JourneyForm"
import { JourneyRow } from "./JourneyRow"
import { JourneyTipCard } from "./JourneyTipCard"
import { JourneysIcon } from "@/components/icons"
import { archivedJourneyListQuery, JOURNEY_PAGE_SIZE, journeyListQuery } from "./queries"
import { useArchiveJourney, useDuplicateJourney, useUnarchiveJourney } from "./mutations"

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

export default function Journeys() {
    const [project] = useContext(ProjectContext)
    const { t } = useTranslation()
    const navigate = useNavigate()
    const duplicateJourney = useDuplicateJourney()
    const archiveJourney = useArchiveJourney()
    const unarchiveJourney = useUnarchiveJourney()

    const [searchQuery, setSearchQuery] = useState("")
    const [debouncedQuery, setDebouncedQuery] = useState("")
    const [isCreateOpen, setIsCreateOpen] = useState(false)
    const [cursor, setCursor] = useState<string | undefined>()
    const [pageDirection, setPageDirection] = useState<"next" | "prev" | undefined>()
    const [cursorHistory, setCursorHistory] = useState<string[]>([])
    const [archivedOffset, setArchivedOffset] = useState(0)
    const [showArchived, setShowArchived] = useState(false)
    const searchTimeoutRef = useRef<ReturnType<typeof setTimeout> | undefined>(undefined)

    const archivedPageSize = JOURNEY_PAGE_SIZE

    const handleSearch = useCallback((value: string) => {
        setSearchQuery(value)
        setCursor(undefined)
        setPageDirection(undefined)
        setCursorHistory([])
        setArchivedOffset(0)
        clearTimeout(searchTimeoutRef.current)
        searchTimeoutRef.current = setTimeout(() => {
            setDebouncedQuery(value)
        }, 300)
    }, [])

    const isArchivedView = showArchived
    // Only the list on screen is fetched. Its params are in the query key, so a
    // slow response for an earlier search or page can't replace a newer one.
    const active = useQuery({
        ...journeyListQuery(project.id, { search: debouncedQuery, cursor, page: pageDirection }),
        enabled: !isArchivedView,
    })
    const archived = useQuery({
        ...archivedJourneyListQuery(project.id, { search: debouncedQuery, offset: archivedOffset }),
        enabled: isArchivedView,
    })
    const list = isArchivedView ? archived : active
    const result = active.data

    const journeys = (isArchivedView ? archived.data?.results : result?.results) ?? []
    const total = list.data?.total ?? 0
    const hasNextPage = isArchivedView
        ? archivedOffset + archivedPageSize < total
        : !!result?.nextCursor
    const hasPrevPage = isArchivedView ? archivedOffset > 0 : cursorHistory.length > 0

    const handleNextPage = () => {
        if (isArchivedView) {
            if (hasNextPage) {
                setArchivedOffset((prev) => prev + archivedPageSize)
            }
            return
        }
        if (result?.nextCursor) {
            setCursorHistory((prev) => [...prev, cursor ?? ""])
            setCursor(result.nextCursor)
            setPageDirection("next")
        }
    }

    const handlePrevPage = () => {
        if (isArchivedView) {
            if (hasPrevPage) {
                setArchivedOffset((prev) => Math.max(0, prev - archivedPageSize))
            }
            return
        }
        if (cursorHistory.length > 0) {
            const prev = [...cursorHistory]
            const prevCursor = prev.pop()
            setCursorHistory(prev)
            setCursor(prevCursor || undefined)
            setPageDirection(prevCursor ? "next" : undefined)
        }
    }

    const handleRowClick = (id: UUID) => {
        navigate(id.toString())
    }

    const handleDuplicateJourney = (e: React.MouseEvent, id: UUID) => {
        e.stopPropagation()
        duplicateJourney.mutate(id, {
            onSuccess: (journey) => navigate(journey.id.toString()),
        })
    }

    const handleArchiveJourney = (e: React.MouseEvent, id: UUID) => {
        e.stopPropagation()
        archiveJourney.mutate(id)
    }

    const handleUnarchiveJourney = (id: UUID) => {
        unarchiveJourney.mutate(id, {
            onSuccess: () => {
                setShowArchived(false)
                setArchivedOffset(0)
            },
        })
    }

    return (
        <div className="flex flex-col gap-4 sm:gap-6 p-4 sm:p-6">
            {/* Header */}
            <div className="flex items-start gap-4">
                <div className="flex h-14 w-14 items-center justify-center rounded-xl shrink-0 bg-muted [&>svg]:h-7 [&>svg]:w-7 [&>svg]:text-muted-foreground">
                    <JourneysIcon />
                </div>
                <div className="space-y-1">
                    <h1 className="text-2xl font-semibold tracking-tight">{t("journeys")}</h1>
                    <p className="text-sm text-muted-foreground">
                        {t(
                            "journeys_description",
                            "Design multi-step automated workflows to engage users at the right moment.",
                        )}
                    </p>
                </div>
            </div>

            {/* Search and Actions */}
            <div className="flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-3 sm:gap-4">
                <div className="relative sm:max-w-sm flex-1">
                    <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                    <Input
                        placeholder={t("search_journeys", "Search journeys...")}
                        value={searchQuery}
                        onChange={(e) => handleSearch(e.target.value)}
                        className="pl-9"
                    />
                </div>
                <div className="flex items-center gap-2">
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
                        aria-label={t("create_journey_from_header", "Create Journey from header")}
                    >
                        <Plus className="mr-2 h-4 w-4" />
                        {t("create_journey")}
                    </Button>
                </div>
            </div>

            {/* Table */}
            <div className="rounded-lg border bg-card">
                <Table>
                    <TableHeader>
                        <TableRow>
                            <TableHead>{t("name")}</TableHead>
                            <TableHead>{t("status")}</TableHead>
                            <TableHead className="hidden sm:table-cell">
                                {t("created_at")}
                            </TableHead>
                            <TableHead className="hidden md:table-cell">
                                {t("updated_at")}
                            </TableHead>
                            <TableHead className="w-[50px]"></TableHead>
                        </TableRow>
                    </TableHeader>
                    <TableBody>
                        {list.isPending ? (
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
                                    <TableCell>
                                        <Skeleton className="h-5 w-16 rounded-md" />
                                    </TableCell>
                                    <TableCell className="hidden sm:table-cell">
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
                        ) : list.isError ? (
                            <TableRow>
                                <TableCell colSpan={5} className="h-32 text-center">
                                    <div className="flex flex-col items-center gap-3 text-muted-foreground">
                                        <p>
                                            {t("journeys_load_failed", "Couldn't load journeys.")}
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
                        ) : journeys.length === 0 ? (
                            <TableRow>
                                <TableCell colSpan={5} className="h-32 text-center">
                                    <div className="flex flex-col items-center gap-2 text-muted-foreground">
                                        <GitBranch className="h-8 w-8" />
                                        <p>
                                            {isArchivedView
                                                ? t("no_archived_journeys", "No archived journeys")
                                                : debouncedQuery
                                                  ? t("no_journeys_found", "No journeys found")
                                                  : t("no_journeys_yet", "No journeys yet")}
                                        </p>
                                        {!debouncedQuery && !isArchivedView && (
                                            <Button
                                                variant="outline"
                                                size="sm"
                                                onClick={() => setIsCreateOpen(true)}
                                                className="mt-2"
                                                aria-label={t(
                                                    "create_journey_from_empty",
                                                    "Create Journey from empty state",
                                                )}
                                            >
                                                <Plus className="mr-2 h-4 w-4" />
                                                {t("create_journey")}
                                            </Button>
                                        )}
                                    </div>
                                </TableCell>
                            </TableRow>
                        ) : (
                            journeys.map((journey) => (
                                <JourneyRow
                                    key={journey.id}
                                    journey={journey}
                                    isArchivedView={isArchivedView}
                                    onOpen={() => handleRowClick(journey.id)}
                                    onDuplicate={(e) => handleDuplicateJourney(e, journey.id)}
                                    onArchive={(e) => handleArchiveJourney(e, journey.id)}
                                    onUnarchive={() => handleUnarchiveJourney(journey.id)}
                                />
                            ))
                        )}
                    </TableBody>
                </Table>

                {/* Pagination */}
                {journeys.length > 0 && (
                    <div className="flex items-center justify-between border-t px-4 py-3">
                        <p className="text-sm text-muted-foreground">
                            {total} {t("journeys").toLowerCase()}
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

            <JourneyTipCard />

            {/* Create Journey Dialog */}
            <Dialog open={isCreateOpen} onOpenChange={setIsCreateOpen}>
                <DialogContent>
                    <DialogHeader>
                        <DialogTitle>{t("create_journey")}</DialogTitle>
                        <DialogDescription>
                            {t(
                                "create_journey_description",
                                "Create a new automated journey to engage your users.",
                            )}
                        </DialogDescription>
                    </DialogHeader>
                    <JourneyForm
                        onSaved={async (journey) => {
                            setIsCreateOpen(false)
                            await navigate(journey.id.toString())
                        }}
                    />
                </DialogContent>
            </Dialog>
        </div>
    )
}
