import { useCallback, useContext, useState, useRef } from "react"
import { useNavigate } from "react-router"
import { useTranslation } from "react-i18next"
import {
    Plus,
    Search,
    ChevronLeft,
    ChevronRight,
    ArrowRight,
    GitBranch,
    Workflow,
    Zap,
    MoreHorizontal,
    Copy,
    Archive,
    ArchiveRestore,
} from "lucide-react"

import api from "../../api"
import { oapiClient } from "@/oapi/client"
import { useResolver } from "../../hooks"
import { formatDate } from "../../utils"
import { getRandomColor } from "@/lib/colors"
import { ProjectContext } from "../../contexts"
import { PreferencesContext } from "@/contexts/PreferencesContext"
import { JourneyForm } from "./JourneyForm"
import { JourneysIcon } from "@/components/icons"

import type { Journey } from "../../types"
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

type JourneyStatus = "draft" | "published" | "archived"

function getStatusBadge(status: JourneyStatus, t: (key: string) => string) {
    const config: Record<JourneyStatus, { label: string; className: string }> = {
        draft: { label: t("draft"), className: "bg-secondary text-secondary-foreground" },
        published: { label: t("published"), className: "bg-green-100 text-green-700" },
        archived: { label: t("archived"), className: "bg-secondary text-secondary-foreground" },
    }
    const { label, className } = config[status] ?? config.draft
    return (
        <Badge variant="outline" className={`border-0 ${className}`}>
            {label}
        </Badge>
    )
}

export default function Journeys() {
    const [project] = useContext(ProjectContext)
    const [preferences] = useContext(PreferencesContext)
    const { t } = useTranslation()
    const navigate = useNavigate()

    const [searchQuery, setSearchQuery] = useState("")
    const [debouncedQuery, setDebouncedQuery] = useState("")
    const [isCreateOpen, setIsCreateOpen] = useState(false)
    const [cursor, setCursor] = useState<string | undefined>()
    const [pageDirection, setPageDirection] = useState<"next" | "prev" | undefined>()
    const [cursorHistory, setCursorHistory] = useState<string[]>([])
    const [archivedOffset, setArchivedOffset] = useState(0)
    const [showArchived, setShowArchived] = useState(false)
    const searchTimeoutRef = useRef<number>()

    const archivedPageSize = 15

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

    const [result, , reload] = useResolver(
        useCallback(async () => {
            return await api.journeys.search(project.id, {
                limit: 15,
                cursor,
                page: pageDirection,
                search: debouncedQuery || undefined,
            })
        }, [project.id, debouncedQuery, cursor, pageDirection]),
    )

    const [archivedResult, , reloadArchived] = useResolver(
        useCallback(async () => {
            if (!showArchived) return null
            const response = await oapiClient.GET("/api/admin/projects/{projectID}/journeys", {
                params: {
                    path: {
                        projectID: project.id,
                    },
                    query: {
                        limit: archivedPageSize,
                        offset: archivedOffset,
                        include_deleted: true,
                        search: debouncedQuery || undefined,
                    },
                },
            })
            if (response.error || !response.data) return null
            return response.data
        }, [project.id, debouncedQuery, showArchived, archivedOffset]),
    )

    const isArchivedView = showArchived
    const journeys = (isArchivedView ? archivedResult?.results : result?.results) ?? []
    const total = isArchivedView ? (archivedResult?.total ?? 0) : (result?.total ?? 0)
    const isLoading = isArchivedView ? archivedResult === null : !result
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

    const handleRowClick = (journey: Journey) => {
        navigate(journey.id.toString())
    }

    const handleDuplicateJourney = async (e: React.MouseEvent, id: UUID) => {
        e.stopPropagation()
        const journey = await api.journeys.duplicate(project.id, id)
        await navigate(journey.id.toString())
    }

    const handleArchiveJourney = async (e: React.MouseEvent, id: UUID) => {
        e.stopPropagation()
        await api.journeys.delete(project.id, id)
        await reload()
    }

    const handleUnarchiveJourney = async (id: UUID) => {
        const response = await oapiClient.POST(
            "/api/admin/projects/{projectID}/journeys/{journeyID}/unarchive",
            {
                params: {
                    path: {
                        projectID: project.id,
                        journeyID: id,
                    },
                },
            },
        )
        if (response.error) {
            throw response.error
        }

        setShowArchived(false)
        setArchivedOffset(0)
        await Promise.all([reload(), reloadArchived()])
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
                        {isLoading ? (
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
                        ) : journeys.length === 0 ? (
                            <TableRow>
                                <TableCell colSpan={5} className="h-32 text-center">
                                    <div className="flex flex-col items-center gap-2 text-muted-foreground">
                                        <GitBranch className="h-8 w-8" />
                                        <p>
                                            {isArchivedView
                                                ? t(
                                                      "no_archived_journeys",
                                                      "No archived journeys",
                                                  )
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
                            journeys.map((journey) => {
                                const journeyColor = getRandomColor(journey.name ?? journey.id)
                                return (
                                    <TableRow
                                        key={journey.id}
                                        className="cursor-pointer"
                                        onClick={() => handleRowClick(journey)}
                                    >
                                        <TableCell>
                                            <button
                                                type="button"
                                                className="flex w-full items-center gap-3 text-left focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
                                                onClick={(e) => {
                                                    e.stopPropagation()
                                                    handleRowClick(journey)
                                                }}
                                            >
                                                <div
                                                    className="flex h-8 w-8 items-center justify-center rounded-md shrink-0"
                                                    style={{ backgroundColor: journeyColor }}
                                                >
                                                    <GitBranch className="h-4 w-4 text-white" />
                                                </div>
                                                <div>
                                                    <div className="font-medium">
                                                        {journey.name}
                                                    </div>
                                                    {journey.description && (
                                                        <div className="text-sm text-muted-foreground truncate max-w-[300px]">
                                                            {journey.description}
                                                        </div>
                                                    )}
                                                </div>
                                            </button>
                                        </TableCell>
                                        <TableCell>
                                            {getStatusBadge(journey.status as JourneyStatus, t)}
                                        </TableCell>
                                        <TableCell className="hidden sm:table-cell text-muted-foreground">
                                            {formatDate(preferences, journey.created_at, "PP")}
                                        </TableCell>
                                        <TableCell className="hidden md:table-cell text-muted-foreground">
                                            {formatDate(preferences, journey.updated_at, "PP")}
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
                                                            handleRowClick(journey)
                                                        }}
                                                    >
                                                        <GitBranch className="mr-2 h-4 w-4" />
                                                        {t("edit")}
                                                    </DropdownMenuItem>
                                                    <DropdownMenuItem
                                                        onClick={(e) =>
                                                            handleDuplicateJourney(e, journey.id)
                                                        }
                                                    >
                                                        <Copy className="mr-2 h-4 w-4" />
                                                        {t("duplicate")}
                                                    </DropdownMenuItem>
                                                    {isArchivedView ? (
                                                        <DropdownMenuItem
                                                            onClick={(e) => {
                                                                e.stopPropagation()
                                                                handleUnarchiveJourney(journey.id)
                                                            }}
                                                        >
                                                            <ArchiveRestore className="mr-2 h-4 w-4" />
                                                            {t("unarchive", "Unarchive")}
                                                        </DropdownMenuItem>
                                                    ) : (
                                                        <DropdownMenuItem
                                                            onClick={(e) =>
                                                                handleArchiveJourney(
                                                                    e,
                                                                    journey.id,
                                                                )
                                                            }
                                                            className="text-destructive"
                                                        >
                                                            <Archive className="mr-2 h-4 w-4" />
                                                            {t("archive")}
                                                        </DropdownMenuItem>
                                                    )}
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

            {/* Tip Card */}
            <div className="group relative overflow-hidden rounded-lg bg-gradient-to-br from-primary/10 via-primary/5 to-transparent border p-6">
                <div className="relative z-10 max-w-md">
                    <h3 className="font-semibold text-foreground">
                        {t("journey_tip_title", "Design automated journeys")}
                    </h3>
                    <p className="mt-1 text-sm text-muted-foreground">
                        {t(
                            "journey_tip_description",
                            "Create multi-step automated workflows to engage users at the right moment with the right message.",
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
                        <GitBranch
                            className="h-10 w-10 text-primary/40 transition-all duration-500 group-hover:text-primary/60 group-hover:scale-110"
                            strokeWidth={1.25}
                        />
                    </div>
                    <div className="flex h-20 w-20 items-center justify-center rounded-xl bg-primary/10 -rotate-6 translate-y-4 transition-all duration-500 ease-out delay-75 group-hover:rotate-3 group-hover:translate-y-0 group-hover:bg-primary/15">
                        <Workflow
                            className="h-10 w-10 text-primary/40 transition-all duration-500 delay-75 group-hover:text-primary/60 group-hover:scale-110"
                            strokeWidth={1.25}
                        />
                    </div>
                    <div className="flex h-20 w-20 items-center justify-center rounded-xl bg-primary/10 rotate-12 -translate-y-2 transition-all duration-500 ease-out delay-150 group-hover:-rotate-6 group-hover:-translate-y-4 group-hover:bg-primary/15">
                        <Zap
                            className="h-10 w-10 text-primary/40 transition-all duration-500 delay-150 group-hover:text-primary/60 group-hover:scale-110"
                            strokeWidth={1.25}
                        />
                    </div>
                </div>
            </div>


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
