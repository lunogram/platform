import { useCallback, useContext, useState } from "react"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { Route, ChevronLeft, ChevronRight, ExternalLink } from "lucide-react"
import { ProjectContext, UserContext } from "../../contexts"
import { PreferencesContext } from "@/contexts/PreferencesContext"
import { useResolver } from "../../hooks"
import { formatDate } from "../../utils"
import { oapiClient } from "@/oapi/client"

import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import {
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from "@/components/ui/table"

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

export default function UserDetailJourneys() {
    const { t } = useTranslation()
    const navigate = useNavigate()
    const [preferences] = useContext(PreferencesContext)
    const [project] = useContext(ProjectContext)
    const [user] = useContext(UserContext)

    const [page, setPage] = useState(1)
    const limit = 25

    const [result] = useResolver(
        useCallback(async () => {
            const res = await oapiClient.GET('/api/admin/projects/{projectID}/users/{userID}/journeys', {
                params: {
                    path: {
                        projectID: project.id,
                        userID: user.id,
                    },
                    query: {
                        limit,
                        offset: (page - 1) * limit,
                    },
                },
            })
            return res.data
        }, [project.id, user.id, page]),
    )

    const journeys = result?.results
    const total = result?.total ?? 0
    const totalPages = Math.ceil(total / limit)
    const hasNextPage = page < totalPages
    const hasPrevPage = page > 1

    return (
        <div className="space-y-4">
            {/* Journeys Table */}
            <div className="border rounded-lg overflow-x-auto scrollbar-none">
                <Table>
                    <TableHeader>
                        <TableRow>
                            <TableHead>{t("journey")}</TableHead>
                            <TableHead>{t("created_at")}</TableHead>
                            <TableHead>{t("ended_at")}</TableHead>
                            <TableHead className="w-12"></TableHead>
                        </TableRow>
                    </TableHeader>
                    <TableBody>
                        {journeys === undefined ? (
                            Array.from({ length: 5 }).map((_, i) => (
                                <TableRow key={i}>
                                    <TableCell>
                                        <Skeleton className="h-4 w-36" />
                                    </TableCell>
                                    <TableCell>
                                        <Skeleton className="h-4 w-28" />
                                    </TableCell>
                                    <TableCell>
                                        <Skeleton className="h-4 w-28" />
                                    </TableCell>
                                    <TableCell></TableCell>
                                </TableRow>
                            ))
                        ) : journeys.length === 0 ? (
                            <TableRow>
                                <TableCell colSpan={4} className="h-48">
                                    <div className="flex flex-col items-center justify-center">
                                        <div className="flex h-12 w-12 items-center justify-center rounded-full bg-muted mb-4">
                                            <Route className="h-6 w-6 text-muted-foreground" />
                                        </div>
                                        <p className="font-medium mb-1">
                                            {t("no_journeys_yet", "No journeys")}
                                        </p>
                                        <p className="text-sm text-muted-foreground max-w-xs text-center">
                                            {t(
                                                "no_journeys_description",
                                                "Journey entries will appear here when the user enters a journey",
                                            )}
                                        </p>
                                    </div>
                                </TableCell>
                            </TableRow>
                        ) : (
                            journeys.map((entry) => (
                                <TableRow
                                    key={entry.id}
                                    className="cursor-pointer"
                                    onClick={() => navigate(`../../journeys/${entry.journey?.id}`)}
                                >
                                    <TableCell className="font-medium">
                                        {entry.journey?.name ?? "—"}
                                    </TableCell>
                                    <TableCell className="text-muted-foreground">
                                        {formatDate(preferences, entry.created_at, "Pp")}
                                    </TableCell>
                                    <TableCell>
                                        {entry.ended_at ? (
                                            <span className="text-muted-foreground">
                                                {formatDate(preferences, entry.ended_at, "Pp")}
                                            </span>
                                        ) : (
                                            <span className="inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400">
                                                {t("running")}
                                            </span>
                                        )}
                                    </TableCell>
                                    <TableCell>
                                        <ExternalLink className="h-4 w-4 text-muted-foreground" />
                                    </TableCell>
                                </TableRow>
                            ))
                        )}
                    </TableBody>
                </Table>

                {/* Pagination */}
                {total > 0 && (
                    <div className="flex items-center justify-between border-t px-4 py-3">
                        <p className="text-sm text-muted-foreground">
                            {total} {total === 1 ? t("journey") : t("journeys")}
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
                )}
            </div>
        </div>
    )
}
