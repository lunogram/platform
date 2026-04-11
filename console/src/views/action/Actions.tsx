import { useCallback, useContext, useMemo, useRef, useState } from "react"
import { useNavigate } from "react-router"
import { useTranslation } from "react-i18next"
import { Search, ChevronLeft, ChevronRight, Zap, Plus, MoreHorizontal, Archive } from "lucide-react"

import oapiClient, { type Action, type ActionMeta } from "@/oapi/client"
import { useResolver } from "../../hooks"
import { formatDate, snakeToTitle } from "../../utils"
import { ProjectContext } from "../../contexts"
import { PreferencesContext } from "@/contexts/PreferencesContext"

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
import { Skeleton } from "@/components/ui/skeleton"
import { Badge } from "@/components/ui/badge"
import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { StaggeredMosaic } from "@/components/icon-mosaic"

const PAGE_SIZE = 15

export default function Actions() {
    const [preferences] = useContext(PreferencesContext)
    const [project] = useContext(ProjectContext)
    const navigate = useNavigate()
    const { t } = useTranslation()

    const [searchQuery, setSearchQuery] = useState("")
    const [debouncedQuery, setDebouncedQuery] = useState("")
    const [page, setPage] = useState(1)
    const searchTimeoutRef = useRef<ReturnType<typeof setTimeout>>()
    const [hoveredAction, setHoveredAction] = useState<Action | null>(null)

    const handleSearch = useCallback((value: string) => {
        setSearchQuery(value)
        setPage(1)
        clearTimeout(searchTimeoutRef.current)
        searchTimeoutRef.current = setTimeout(() => {
            setDebouncedQuery(value)
        }, 300)
    }, [])

    const [result, , reload] = useResolver(
        useCallback(async () => {
            const { data } = await oapiClient.GET("/api/admin/projects/{projectID}/actions", {
                params: {
                    path: { projectID: project.id },
                    query: {
                        limit: PAGE_SIZE,
                        offset: (page - 1) * PAGE_SIZE,
                        search: debouncedQuery || undefined,
                    },
                },
            })
            return data ?? null
        }, [project.id, debouncedQuery, page]),
    )

    const [actionMetas] = useResolver(
        useCallback(async () => {
            const { data } = await oapiClient.GET("/api/admin/projects/{projectID}/actions/meta", {
                params: { path: { projectID: project.id } },
            })
            return data ?? null
        }, [project.id]),
    )

    const actions = result?.results
    const total = result?.total ?? 0
    const totalPages = Math.ceil(total / PAGE_SIZE)
    const hasNextPage = page < totalPages
    const hasPrevPage = page > 1

    const handleNextPage = () => {
        if (hasNextPage) setPage((p) => p + 1)
    }

    const handlePrevPage = () => {
        if (hasPrevPage) setPage((p) => p - 1)
    }

    const handleArchiveAction = async (e: React.MouseEvent, action: Action) => {
        e.stopPropagation()
        await oapiClient.DELETE("/api/admin/projects/{projectID}/actions/{actionID}", {
            params: {
                path: { projectID: project.id, actionID: action.id },
            },
        })
        await reload()
    }

    const handleRowClick = (action: Action) => {
        navigate(`/projects/${project.id}/actions/${action.id.toString()}`)
    }

    const mosaicProvider = useMemo(() => {
        if (!hoveredAction || !actionMetas) return undefined
        const meta = actionMetas.find((m: ActionMeta) => m.type === hoveredAction.type)
        return {
            id: hoveredAction.type,
            name: meta?.name ?? hoveredAction.type,
            icon: meta?.icon,
            color: meta?.color,
        }
    }, [hoveredAction, actionMetas])

    return (
        <div className="flex flex-col min-h-full">
            {/* Header with ambient mosaic */}
            <div className="border-b bg-card/50 relative overflow-hidden">
                <div
                    className="absolute top-1/2 -translate-y-1/2 left-[50%] xl:left-[30%] right-0 hidden lg:block pointer-events-none opacity-[0.8]"
                    style={{
                        maskImage: "linear-gradient(to right, transparent 0%, black 40%)",
                        WebkitMaskImage: "linear-gradient(to right, transparent 0%, black 40%)",
                    }}
                >
                    <StaggeredMosaic provider={mosaicProvider} cols={12} rows={4} />
                </div>

                <div className="p-4 sm:p-6 py-8 sm:py-10 relative z-20">
                    <div className="flex items-start gap-4">
                        <div className="space-y-1">
                            <h1 className="text-2xl font-semibold tracking-tight">
                                {t("actions.plural")}
                            </h1>
                            <p className="text-sm text-muted-foreground">
                                {t(
                                    "actions_description",
                                    "Create and manage webhook actions and other integrations.",
                                )}
                            </p>
                        </div>
                    </div>
                </div>
            </div>

            {/* Content */}
            <div className="flex-1 overflow-y-auto p-4 sm:p-6 flex flex-col gap-4 sm:gap-6">
                {/* Search and Actions */}
                <div className="flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-3 sm:gap-4">
                    <div className="relative sm:max-w-sm flex-1">
                        <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                        <Input
                            placeholder={t("search_actions", "Search actions...")}
                            value={searchQuery}
                            onChange={(e) => handleSearch(e.target.value)}
                            className="pl-9"
                        />
                    </div>
                    <Button onClick={() => navigate(`/projects/${project.id}/actions/new`)}>
                        <Plus className="mr-2 h-4 w-4" />
                        {t("create")}
                    </Button>
                </div>

                {/* Table */}
                <div className="rounded-lg border bg-card">
                    <Table>
                        <TableHeader>
                            <TableRow>
                                <TableHead>{t("name")}</TableHead>
                                <TableHead className="hidden sm:table-cell">{t("type")}</TableHead>
                                <TableHead className="hidden md:table-cell">
                                    {t("updated_at")}
                                </TableHead>
                                <TableHead className="w-[50px]"></TableHead>
                            </TableRow>
                        </TableHeader>
                        <TableBody>
                            {!actions ? (
                                Array.from({ length: 5 }).map((_, i) => (
                                    <TableRow key={i}>
                                        <TableCell>
                                            <Skeleton className="h-4 w-36" />
                                        </TableCell>
                                        <TableCell className="hidden sm:table-cell">
                                            <Skeleton className="h-5 w-16 rounded-md" />
                                        </TableCell>
                                        <TableCell className="hidden md:table-cell">
                                            <Skeleton className="h-4 w-28" />
                                        </TableCell>
                                        <TableCell>
                                            <Skeleton className="h-4 w-8" />
                                        </TableCell>
                                    </TableRow>
                                ))
                            ) : actions.length === 0 ? (
                                <TableRow>
                                    <TableCell colSpan={4} className="h-32 text-center">
                                        <div className="flex flex-col items-center gap-2 text-muted-foreground">
                                            <Zap className="h-8 w-8" />
                                            <p>
                                                {debouncedQuery
                                                    ? t("no_results")
                                                    : t("no_actions_yet", "No actions yet")}
                                            </p>
                                            {!debouncedQuery && (
                                                <Button
                                                    variant="outline"
                                                    size="sm"
                                                    onClick={() =>
                                                        navigate(
                                                            `/projects/${project.id}/actions/new`,
                                                        )
                                                    }
                                                    className="mt-2"
                                                >
                                                    <Plus className="mr-2 h-4 w-4" />
                                                    {t("create")}
                                                </Button>
                                            )}
                                        </div>
                                    </TableCell>
                                </TableRow>
                            ) : (
                                actions.map((action) => (
                                    <TableRow
                                        key={action.id}
                                        className="cursor-pointer"
                                        onClick={() => handleRowClick(action)}
                                        onMouseEnter={() => setHoveredAction(action)}
                                        onMouseLeave={() => setHoveredAction(null)}
                                    >
                                        <TableCell className="font-medium">{action.name}</TableCell>
                                        <TableCell className="hidden sm:table-cell">
                                            <Badge variant="secondary">
                                                {snakeToTitle(action.type)}
                                            </Badge>
                                        </TableCell>
                                        <TableCell className="hidden md:table-cell text-muted-foreground">
                                            {formatDate(preferences, action.updated_at, "PP")}
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
                                                            handleRowClick(action)
                                                        }}
                                                    >
                                                        {t("edit")}
                                                    </DropdownMenuItem>
                                                    <DropdownMenuItem
                                                        onClick={(e) =>
                                                            handleArchiveAction(e, action)
                                                        }
                                                        className="text-destructive"
                                                    >
                                                        <Archive className="mr-2 h-4 w-4" />
                                                        {t("delete")}
                                                    </DropdownMenuItem>
                                                </DropdownMenuContent>
                                            </DropdownMenu>
                                        </TableCell>
                                    </TableRow>
                                ))
                            )}
                        </TableBody>
                    </Table>

                    {/* Pagination */}
                    {actions && actions.length > 0 && (
                        <div className="flex items-center justify-between border-t px-4 py-3">
                            <p className="text-sm text-muted-foreground">
                                {total} {t("actions.plural")}
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
            </div>
        </div>
    )
}
