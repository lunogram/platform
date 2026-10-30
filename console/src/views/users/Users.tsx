import { useCallback, useState, useRef, useContext } from "react"
import { useParams } from "react-router"
import { useTranslation } from "react-i18next"
import { useQuery } from "@tanstack/react-query"
import {
    Plus,
    UserCircle2,
    Search,
    ChevronLeft,
    ChevronRight,
    ArrowRight,
    Upload,
    Mail,
    Database,
} from "lucide-react"
import { UserImportDialog } from "@/components/ui/user-import-dialog"
import { NIL } from "uuid"
import { useRoute } from "@/hooks/use-route"
import { formatDate } from "../../utils"
import { getRandomColor } from "@/lib/colors"
import { getUserDisplayName, getUserInitials, getPrimaryExternalId } from "@/lib/name"
import { PreferencesContext } from "@/contexts/PreferencesContext"
import { UsersIcon as UsersPageIcon } from "@/components/icons"
import type { UUID } from "@/types/common"
import type { User } from "../../types"
import { CreateUserDialog } from "./CreateUserDialog"
import { LoadErrorRow } from "./LoadErrorRow"
import { useImportUsers, userSearchQuery } from "./queries"

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

export default function Users() {
    const { projectId = NIL as UUID } = useParams<{ projectId: UUID }>()
    const { t } = useTranslation()
    const route = useRoute()
    const [preferences] = useContext(PreferencesContext)

    const [searchQuery, setSearchQuery] = useState("")
    const [debouncedQuery, setDebouncedQuery] = useState("")
    const [isCreateOpen, setIsCreateOpen] = useState(false)
    const [isBulkImportOpen, setIsBulkImportOpen] = useState(false)
    const importUsers = useImportUsers()

    const [page, setPage] = useState(1)
    const limit = 15
    const searchTimeoutRef = useRef<ReturnType<typeof setTimeout> | undefined>(undefined)

    // Debounce search
    const handleSearch = useCallback((value: string) => {
        setSearchQuery(value)
        setPage(1)
        clearTimeout(searchTimeoutRef.current)
        searchTimeoutRef.current = setTimeout(() => {
            setDebouncedQuery(value)
        }, 300)
    }, [])

    const result = useQuery(
        userSearchQuery(projectId, {
            limit,
            offset: (page - 1) * limit,
            search: debouncedQuery,
        }),
    )

    const users = result.data?.results ?? []
    const total = result.data?.total ?? 0
    const totalPages = Math.ceil(total / limit)
    const hasNextPage = page < totalPages
    const hasPrevPage = page > 1

    // UserImportDialog does not catch a rejected import, so a throw here would
    // go unhandled. The mutation cache has already shown the error.
    const handleImportUsers = async (file: File) => {
        try {
            await importUsers.mutateAsync(file)
        } catch {
            // Reported by the mutation cache.
        }
    }

    const handleRowClick = (user: User) => {
        route(`users/${user.id}`)
    }

    return (
        <div className="flex flex-col gap-4 sm:gap-6 p-4 sm:p-6">
            {/* Header */}
            <div className="flex items-start gap-4">
                <div className="flex h-14 w-14 items-center justify-center rounded-xl shrink-0 bg-muted [&>svg]:h-7 [&>svg]:w-7 [&>svg]:text-muted-foreground">
                    <UsersPageIcon />
                </div>
                <div className="space-y-1">
                    <h1 className="text-2xl font-semibold tracking-tight">{t("users")}</h1>
                    <p className="text-sm text-muted-foreground">
                        {t(
                            "users_description",
                            "View, search, and manage the users in your project.",
                        )}
                    </p>
                </div>
            </div>

            {/* Search and Actions */}
            <div className="flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-3 sm:gap-4">
                <div className="relative sm:max-w-sm flex-1">
                    <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                    <Input
                        placeholder={t("search_users")}
                        value={searchQuery}
                        onChange={(e) => handleSearch(e.target.value)}
                        className="pl-9"
                    />
                </div>
                <div className="flex w-full items-center gap-2 sm:w-auto">
                    <Button
                        variant="outline"
                        onClick={() => setIsBulkImportOpen(true)}
                        className="flex-1 sm:flex-initial"
                    >
                        <Upload className="mr-2 h-4 w-4" />
                        <span className="hidden sm:inline">
                            {t("import_users", "Import users")}
                        </span>
                        <span className="sm:hidden">{t("import", "Import")}</span>
                    </Button>
                    <Button
                        onClick={() => setIsCreateOpen(true)}
                        className="flex-1 sm:flex-initial"
                    >
                        <Plus className="mr-2 h-4 w-4" />
                        {t("create_user")}
                    </Button>
                </div>
            </div>

            {/* Table */}
            <div className="rounded-lg border bg-card">
                <Table>
                    <TableHeader>
                        <TableRow>
                            <TableHead>{t("name")}</TableHead>
                            <TableHead className="hidden sm:table-cell">{t("email")}</TableHead>

                            <TableHead className="hidden md:table-cell">
                                {t("created_at")}
                            </TableHead>
                        </TableRow>
                    </TableHeader>
                    <TableBody>
                        {result.isPending ? (
                            // Loading skeleton
                            Array.from({ length: 5 }).map((_, i) => (
                                <TableRow key={i}>
                                    <TableCell>
                                        <div className="flex items-center gap-3">
                                            <Skeleton className="h-8 w-8 rounded-full" />
                                            <div className="space-y-1">
                                                <Skeleton className="h-4 w-32" />
                                                <Skeleton className="h-3 w-20" />
                                            </div>
                                        </div>
                                    </TableCell>
                                    <TableCell className="hidden sm:table-cell">
                                        <Skeleton className="h-4 w-36" />
                                    </TableCell>

                                    <TableCell className="hidden md:table-cell">
                                        <Skeleton className="h-4 w-28" />
                                    </TableCell>
                                </TableRow>
                            ))
                        ) : result.isError ? (
                            <LoadErrorRow
                                colSpan={3}
                                message={t("users_load_failed", "Couldn't load users.")}
                                onRetry={() => result.refetch()}
                            />
                        ) : users.length === 0 ? (
                            <TableRow>
                                <TableCell colSpan={3} className="h-32 text-center">
                                    <div className="flex flex-col items-center gap-2 text-muted-foreground">
                                        <UserCircle2 className="h-8 w-8" />
                                        <p>
                                            {debouncedQuery
                                                ? t("no_users_found", "No users found")
                                                : t("no_users_yet", "No users yet")}
                                        </p>
                                        {!debouncedQuery && (
                                            <Button
                                                variant="outline"
                                                size="sm"
                                                onClick={() => setIsCreateOpen(true)}
                                                className="mt-2"
                                            >
                                                <Plus className="mr-2 h-4 w-4" />
                                                {t("create_user")}
                                            </Button>
                                        )}
                                    </div>
                                </TableCell>
                            </TableRow>
                        ) : (
                            users.map((user) => {
                                const userColor = getRandomColor(
                                    user.email ??
                                        getPrimaryExternalId(
                                            user as unknown as Record<string, unknown>,
                                        ) ??
                                        user.id,
                                )
                                return (
                                    <TableRow
                                        key={user.id}
                                        className="cursor-pointer"
                                        onClick={() => handleRowClick(user)}
                                    >
                                        <TableCell>
                                            <div className="flex items-center gap-3">
                                                <div
                                                    className="flex h-8 w-8 items-center justify-center rounded-full text-white text-xs font-medium shrink-0"
                                                    style={{ backgroundColor: userColor }}
                                                >
                                                    {getUserInitials(user)}
                                                </div>
                                                <div>
                                                    <div className="font-medium">
                                                        {getUserDisplayName(user)}
                                                    </div>
                                                    {user.phone && (
                                                        <div className="text-sm text-muted-foreground">
                                                            {user.phone}
                                                        </div>
                                                    )}
                                                </div>
                                            </div>
                                        </TableCell>
                                        <TableCell className="hidden sm:table-cell text-muted-foreground">
                                            {user.email ?? "—"}
                                        </TableCell>

                                        <TableCell className="hidden md:table-cell text-muted-foreground">
                                            {formatDate(preferences, user.created_at, "PP")}
                                        </TableCell>
                                    </TableRow>
                                )
                            })
                        )}
                    </TableBody>
                </Table>

                {/* Pagination */}
                {total > 0 && (
                    <div className="flex items-center justify-between border-t px-4 py-3">
                        <p className="text-sm text-muted-foreground">
                            {total} {total === 1 ? t("user", "user") : t("users")}
                        </p>
                        {totalPages > 1 && (
                            <div className="flex items-center gap-2">
                                <Button
                                    variant="outline"
                                    size="sm"
                                    onClick={() => setPage((p) => p - 1)}
                                    disabled={!hasPrevPage}
                                    aria-label={t("previous")}
                                >
                                    <ChevronLeft className="h-4 w-4 sm:mr-1" />
                                    <span className="hidden sm:inline">{t("previous")}</span>
                                </Button>
                                <span className="text-sm text-muted-foreground px-2">
                                    {page} / {totalPages}
                                </span>
                                <Button
                                    variant="outline"
                                    size="sm"
                                    onClick={() => setPage((p) => p + 1)}
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
                        {t("sync_users_title", "Sync users via API")}
                    </h3>
                    <p className="mt-1 text-sm text-muted-foreground">
                        {t(
                            "sync_users_description",
                            "Keep your users in sync with your system by using the API to create and update them automatically.",
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

                {/* Decorative elements with hover animations */}
                <div className="absolute -right-6 -bottom-6 flex gap-4">
                    <div className="hidden sm:flex h-20 w-20 items-center justify-center rounded-xl bg-primary/10 rotate-12 transition-all duration-500 ease-out group-hover:rotate-6 group-hover:-translate-y-2 group-hover:bg-primary/15">
                        <UserCircle2
                            className="h-10 w-10 text-primary/40 transition-all duration-500 group-hover:text-primary/60 group-hover:scale-110"
                            strokeWidth={1.25}
                        />
                    </div>
                    <div className="flex h-20 w-20 items-center justify-center rounded-xl bg-primary/10 -rotate-6 translate-y-4 transition-all duration-500 ease-out delay-75 group-hover:rotate-3 group-hover:translate-y-0 group-hover:bg-primary/15">
                        <Mail
                            className="h-10 w-10 text-primary/40 transition-all duration-500 delay-75 group-hover:text-primary/60 group-hover:scale-110"
                            strokeWidth={1.25}
                        />
                    </div>
                    <div className="flex h-20 w-20 items-center justify-center rounded-xl bg-primary/10 rotate-12 -translate-y-2 transition-all duration-500 ease-out delay-150 group-hover:-rotate-6 group-hover:-translate-y-4 group-hover:bg-primary/15">
                        <Database
                            className="h-10 w-10 text-primary/40 transition-all duration-500 delay-150 group-hover:text-primary/60 group-hover:scale-110"
                            strokeWidth={1.25}
                        />
                    </div>
                </div>
            </div>

            <CreateUserDialog open={isCreateOpen} onOpenChange={setIsCreateOpen} />

            <UserImportDialog
                open={isBulkImportOpen}
                onOpenChange={setIsBulkImportOpen}
                onImport={handleImportUsers}
            />
        </div>
    )
}
