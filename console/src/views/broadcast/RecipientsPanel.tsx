import { useContext } from "react"
import { Link, useNavigate } from "react-router"
import { useTranslation } from "react-i18next"
import {
    AlertCircle,
    Search,
    ExternalLink,
    Eye,
    Users,
    ChevronLeft,
    ChevronRight,
} from "lucide-react"

import { snakeToTitle } from "@/utils"
import { getUserDisplayName } from "@/lib/name"
import { ProjectContext } from "@/contexts"

import type { Broadcast } from "@/types"
import type { RecipientRow } from "./broadcast-state"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { Input } from "@/components/ui/input"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"
import {
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from "@/components/ui/table"

interface RecipientsPanelProps {
    broadcast: Broadcast
    users: RecipientRow[] | null
    displayTotal: number | null
    usersTotal: number | null
    isPreview: boolean
    usersOffset: number
    usersSearch: string
    usersPageSize: number
    hasSearchQuery: boolean
    onUsersSearch: (value: string) => void
    onSetUsersOffset: React.Dispatch<React.SetStateAction<number>>
}

export function RecipientsPanel({
    broadcast,
    users,
    displayTotal,
    usersTotal,
    isPreview,
    usersOffset,
    usersSearch,
    usersPageSize,
    hasSearchQuery,
    onUsersSearch,
    onSetUsersOffset,
}: RecipientsPanelProps) {
    const { t } = useTranslation()
    const navigate = useNavigate()
    const [project] = useContext(ProjectContext)

    return (
        <div className="space-y-4">
            {/* Error Alert */}
            {broadcast.state === "failed" && broadcast.error && (
                <Alert variant="destructive">
                    <AlertCircle className="h-4 w-4" />
                    <AlertTitle>{t("broadcast_failed", "Broadcast Failed")}</AlertTitle>
                    <AlertDescription>{broadcast.error}</AlertDescription>
                </Alert>
            )}

            <div className="flex items-center gap-3 flex-wrap">
                <div className="relative flex-1 min-w-[180px] sm:max-w-sm">
                    <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                    <Input
                        placeholder={t("search_recipients", "Search recipients...")}
                        value={usersSearch}
                        onChange={(e) => onUsersSearch(e.target.value)}
                        className="pl-9"
                    />
                </div>
                {displayTotal != null && (
                    <Badge variant="secondary" className="font-normal">
                        {displayTotal.toLocaleString()} {t("users").toLowerCase()}
                    </Badge>
                )}
                {isPreview && (
                    <Tooltip>
                        <TooltipTrigger asChild>
                            <div className="flex items-center gap-1.5 rounded-md border border-amber-200 bg-amber-50/50 dark:border-amber-800 dark:bg-amber-950/20 px-2.5 py-1.5">
                                <Eye className="h-3.5 w-3.5 text-amber-600 dark:text-amber-400 shrink-0" />
                                <span className="text-xs text-amber-700 dark:text-amber-300 whitespace-nowrap">
                                    {t("preview", "Preview")}
                                </span>
                            </div>
                        </TooltipTrigger>
                        <TooltipContent>
                            {t(
                                "broadcast_preview_tooltip",
                                "Showing current list members. The actual recipients will be determined when the broadcast is sent.",
                            )}
                        </TooltipContent>
                    </Tooltip>
                )}
                {broadcast.list_id && (
                    <Button variant="outline" size="sm" asChild className="ml-auto">
                        <Link to={`/projects/${project.id}/lists/${broadcast.list_id}`}>
                            <ExternalLink className="mr-1.5 h-3.5 w-3.5" />
                            {t("view_list", "View list")}
                        </Link>
                    </Button>
                )}
            </div>

            <div className="rounded-lg border bg-card">
                <Table>
                    <TableHeader>
                        <TableRow>
                            <TableHead>{t("name")}</TableHead>
                            <TableHead>{t("email")}</TableHead>
                            <TableHead className="hidden sm:table-cell">{t("phone")}</TableHead>
                            {!isPreview && (
                                <TableHead className="hidden md:table-cell">
                                    {t("status", "Status")}
                                </TableHead>
                            )}
                        </TableRow>
                    </TableHeader>
                    <TableBody>
                        {!users ? (
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
                                    {!isPreview && (
                                        <TableCell className="hidden md:table-cell">
                                            <Skeleton className="h-4 w-16" />
                                        </TableCell>
                                    )}
                                </TableRow>
                            ))
                        ) : users.length === 0 ? (
                            <TableRow>
                                <TableCell colSpan={isPreview ? 3 : 4} className="h-32 text-center">
                                    <div className="flex flex-col items-center gap-2 text-muted-foreground">
                                        <Users className="h-8 w-8" />
                                        <p>
                                            {hasSearchQuery
                                                ? t("no_recipients_found", "No recipients found")
                                                : isPreview
                                                  ? t(
                                                        "no_recipients_in_list",
                                                        "No users in this list",
                                                    )
                                                  : broadcast?.state === "sending"
                                                    ? t(
                                                          "waiting_for_sends",
                                                          "Waiting for messages to be sent...",
                                                      )
                                                    : t(
                                                          "no_recipients_sent",
                                                          "No recipients were sent to",
                                                      )}
                                        </p>
                                    </div>
                                </TableCell>
                            </TableRow>
                        ) : (
                            users.map((user) => {
                                const userId = "user_id" in user ? user.user_id : user.id
                                const sendState = "state" in user && !isPreview ? user.state : null
                                const goToUser = () =>
                                    navigate(`/projects/${project.id}/users/${userId}`)
                                return (
                                    <TableRow
                                        key={user.id}
                                        className="cursor-pointer"
                                        tabIndex={0}
                                        onClick={goToUser}
                                        onKeyDown={(e) => {
                                            if (e.key === "Enter" || e.key === " ") {
                                                e.preventDefault()
                                                goToUser()
                                            }
                                        }}
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
                                        {!isPreview && (
                                            <TableCell className="hidden md:table-cell">
                                                {sendState ? (
                                                    <Badge
                                                        variant="outline"
                                                        className="text-xs font-normal"
                                                    >
                                                        {snakeToTitle(sendState)}
                                                    </Badge>
                                                ) : (
                                                    "—"
                                                )}
                                            </TableCell>
                                        )}
                                    </TableRow>
                                )
                            })
                        )}
                    </TableBody>
                </Table>

                {/* Pagination Footer */}
                {users && users.length > 0 && displayTotal != null && (
                    <div className="flex items-center justify-between border-t px-4 py-3">
                        <p className="text-sm text-muted-foreground">
                            {t(
                                "showing_of_total",
                                `Showing ${users.length} of ${displayTotal.toLocaleString()} recipients`,
                                {
                                    count: users.length,
                                    total: displayTotal.toLocaleString(),
                                },
                            )}
                        </p>
                        {(usersOffset > 0 || usersOffset + usersPageSize < (usersTotal ?? 0)) && (
                            <div className="flex items-center gap-2">
                                <Button
                                    variant="outline"
                                    size="sm"
                                    onClick={() =>
                                        onSetUsersOffset((prev) =>
                                            Math.max(0, prev - usersPageSize),
                                        )
                                    }
                                    disabled={usersOffset <= 0}
                                    aria-label={t("previous")}
                                >
                                    <ChevronLeft className="h-4 w-4 sm:mr-1" />
                                    <span className="hidden sm:inline">{t("previous")}</span>
                                </Button>
                                <Button
                                    variant="outline"
                                    size="sm"
                                    onClick={() => onSetUsersOffset((prev) => prev + usersPageSize)}
                                    disabled={usersOffset + usersPageSize >= (usersTotal ?? 0)}
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
