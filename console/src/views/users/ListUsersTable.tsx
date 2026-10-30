import { useCallback, useContext, useEffect, useRef, useState } from "react"
import { useTranslation } from "react-i18next"
import { useQuery, useQueryClient } from "@tanstack/react-query"
import { ChevronLeft, ChevronRight, Eye, Search, Users } from "lucide-react"
import { ProjectContext } from "../../contexts"
import type { List } from "../../types"
import { getUserDisplayName } from "@/lib/name"
import { useRoute } from "@/hooks/use-route"
import { LoadErrorRow } from "./LoadErrorRow"
import { LIST_USERS_PAGE_SIZE, listKeys, listUsersQuery } from "./listQueries"

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
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"

/** The users in a list (or, for an unpublished rule, the users it would match). */
export function ListUsersTable({ list }: { list: List }) {
    const { t } = useTranslation()
    const [project] = useContext(ProjectContext)
    const queryClient = useQueryClient()
    const route = useRoute()
    const [searchQuery, setSearchQuery] = useState("")
    const [debouncedQuery, setDebouncedQuery] = useState("")
    const [offset, setOffset] = useState(0)
    const searchTimeoutRef = useRef<ReturnType<typeof setTimeout> | undefined>(undefined)
    const pageSize = LIST_USERS_PAGE_SIZE

    const isPreviewMode = list.type === "dynamic" && !!list.draft_rule
    const isGenerating = list.state === "loading"

    // The params are part of the query key, so a slow response for an old
    // search or page can never replace a newer one.
    const result = useQuery({
        ...listUsersQuery(project.id, list.id, {
            offset,
            search: debouncedQuery,
            preview: isPreviewMode,
        }),
        // Follow the members while the list is being generated.
        refetchInterval: isGenerating ? 5000 : false,
    })
    const users = result.data?.results ?? []
    const total = result.data?.total ?? null

    // Once generation finishes, show the final members.
    const wasGenerating = useRef(isGenerating)
    useEffect(() => {
        if (wasGenerating.current && !isGenerating) {
            void queryClient.invalidateQueries({ queryKey: listKeys.users(project.id, list.id) })
        }
        wasGenerating.current = isGenerating
    }, [isGenerating, queryClient, project.id, list.id])

    const handleSearch = useCallback((value: string) => {
        setSearchQuery(value)
        setOffset(0)
        clearTimeout(searchTimeoutRef.current)
        searchTimeoutRef.current = setTimeout(() => {
            setDebouncedQuery(value)
        }, 300)
    }, [])

    const hasPrevPage = offset > 0
    const hasNextPage = total != null && offset + pageSize < total

    const handleNextPage = () => {
        if (hasNextPage) {
            setOffset((prev) => prev + pageSize)
        }
    }

    const handlePrevPage = () => {
        if (hasPrevPage) {
            setOffset((prev) => Math.max(0, prev - pageSize))
        }
    }

    return (
        <div className="space-y-4">
            <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 sm:gap-4">
                <h3 className="text-base font-semibold">{t("users")}</h3>
                <div className="flex items-center gap-3 flex-wrap">
                    {isPreviewMode && (
                        <div className="flex items-center gap-1.5 rounded-md border border-amber-200 bg-amber-50/50 dark:border-amber-800 dark:bg-amber-950/20 px-2.5 py-1.5">
                            <Eye className="h-3.5 w-3.5 text-amber-600 dark:text-amber-400 shrink-0" />
                            <span className="text-xs text-amber-700 dark:text-amber-300 whitespace-nowrap">
                                {t("preview_mode", "Preview mode")}
                                {total != null && (
                                    <>
                                        {" · "}
                                        <span className="font-medium">
                                            {total.toLocaleString()} {t("matching", "matching")}
                                        </span>
                                    </>
                                )}
                            </span>
                        </div>
                    )}
                    {isPreviewMode ? (
                        <Tooltip>
                            <TooltipTrigger asChild>
                                <div className="relative max-w-xs flex-1">
                                    <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground/50" />
                                    <Input
                                        placeholder={t("search_users", "Search users...")}
                                        aria-label={t("search_users", "Search users")}
                                        disabled
                                        className="pl-9 h-8"
                                    />
                                </div>
                            </TooltipTrigger>
                            <TooltipContent>
                                {t(
                                    "search_disabled_preview",
                                    "Search is not available in preview mode",
                                )}
                            </TooltipContent>
                        </Tooltip>
                    ) : (
                        <div className="relative max-w-xs flex-1">
                            <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                            <Input
                                placeholder={t("search_users", "Search users...")}
                                aria-label={t("search_users", "Search users")}
                                value={searchQuery}
                                onChange={(e) => handleSearch(e.target.value)}
                                className="pl-9 h-8"
                            />
                        </div>
                    )}
                </div>
            </div>

            <div className="rounded-lg border bg-card">
                <Table>
                    <TableHeader>
                        <TableRow>
                            <TableHead>{t("name")}</TableHead>
                            <TableHead>{t("email")}</TableHead>
                            <TableHead className="hidden sm:table-cell">{t("phone")}</TableHead>
                        </TableRow>
                    </TableHeader>
                    <TableBody>
                        {result.isPending ? (
                            Array.from({ length: 5 }).map((_, i) => (
                                <TableRow key={i}>
                                    <TableCell>
                                        <Skeleton className="h-4 w-32" />
                                    </TableCell>
                                    <TableCell>
                                        <Skeleton className="h-4 w-36" />
                                    </TableCell>
                                    <TableCell className="hidden sm:table-cell">
                                        <Skeleton className="h-4 w-24" />
                                    </TableCell>
                                </TableRow>
                            ))
                        ) : result.isError ? (
                            <LoadErrorRow
                                colSpan={3}
                                message={t(
                                    "list_users_load_failed",
                                    "Couldn't load the users of this list.",
                                )}
                                onRetry={() => result.refetch()}
                            />
                        ) : users.length === 0 ? (
                            <TableRow>
                                <TableCell colSpan={3} className="h-32 text-center">
                                    <div className="flex flex-col items-center gap-2 text-muted-foreground">
                                        <Users className="h-8 w-8" />
                                        <p>
                                            {debouncedQuery
                                                ? t("no_users_found", "No users found")
                                                : t("no_users_yet", "No users in this list yet")}
                                        </p>
                                    </div>
                                </TableCell>
                            </TableRow>
                        ) : (
                            users.map((user) => (
                                <TableRow
                                    key={user.id}
                                    className="cursor-pointer"
                                    onClick={() => route(`users/${user.id}`)}
                                >
                                    <TableCell className="font-medium">
                                        {getUserDisplayName(user, "—")}
                                    </TableCell>
                                    <TableCell className="text-muted-foreground">
                                        {user.email || "—"}
                                    </TableCell>
                                    <TableCell className="text-muted-foreground hidden sm:table-cell">
                                        {user.phone || "—"}
                                    </TableCell>
                                </TableRow>
                            ))
                        )}
                    </TableBody>
                </Table>

                {/* Pagination */}
                {users.length > 0 && !isPreviewMode && (
                    <div className="flex items-center justify-between border-t px-4 py-3">
                        <p className="text-sm text-muted-foreground">
                            {total != null
                                ? `${(offset + 1).toLocaleString()}–${Math.min(offset + pageSize, total).toLocaleString()} of ${total.toLocaleString()} ${t("users").toLowerCase()}`
                                : `${users.length} ${t("users").toLowerCase()}`}
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
    )
}
