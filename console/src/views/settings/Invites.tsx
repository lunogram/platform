import { useCallback, useContext, useState, useRef } from "react"
import { useTranslation } from "react-i18next"
import {
    Plus,
    Search,
    UserPlus,
    MoreHorizontal,
    Copy,
    Mail,
    Clock,
    CheckCircle,
    XCircle,
    ClockFading,
    ChevronLeft,
    ChevronRight,
    SlidersHorizontal,
    X,
} from "lucide-react"
import { toast } from "sonner"
import api from "@/api"
import { ProjectContext } from "@/contexts"
import { useResolver } from "@/hooks"
import { snakeToTitle } from "@/utils"
import { projectRoles, type ProjectInvite, type ProjectRole } from "@/types"
import InviteDialog from "./InviteDialog"

import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from "@/components/ui/table"
import { Skeleton } from "@/components/ui/skeleton"
import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover"
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from "@/components/ui/dialog"

export default function Invites() {
    const { t } = useTranslation()
    const [project] = useContext(ProjectContext)

    const [searchQuery, setSearchQuery] = useState("")
    const [debouncedQuery, setDebouncedQuery] = useState("")
    const [statusFilter, setStatusFilter] = useState<string | undefined>(undefined)
    const [roleFilter, setRoleFilter] = useState<ProjectRole | undefined>(undefined)
    const [expiresAfter, setExpiresAfter] = useState("")
    const [expiresBefore, setExpiresBefore] = useState("")
    const [inviterAdminFilter, setInviterAdminFilter] = useState<string | undefined>(undefined)
    const [page, setPage] = useState(1)
    const limit = 15
    const searchTimeoutRef = useRef<ReturnType<typeof setTimeout>>(setTimeout(() => {}, 0))
    const [isCreating, setIsCreating] = useState(false)
    const [isSaving, setIsSaving] = useState(false)
    const [createdInviteToken, setCreatedInviteToken] = useState<string | null>(null)

    const handleSearch = useCallback((value: string) => {
        setSearchQuery(value)
        setPage(1)
        clearTimeout(searchTimeoutRef.current)
        searchTimeoutRef.current = setTimeout(() => {
            setDebouncedQuery(value)
        }, 300)
    }, [])

    const handleStatusFilterChange = useCallback((value: string) => {
        setStatusFilter(value === "all" ? undefined : value)
        setPage(1)
    }, [])

    const handleRoleFilterChange = useCallback((value: string) => {
        setRoleFilter(value === "all" ? undefined : (value as ProjectRole))
        setPage(1)
    }, [])

    const handleExpiresAfterChange = useCallback((value: string) => {
        setExpiresAfter(value)
        setPage(1)
    }, [])

    const handleExpiresBeforeChange = useCallback((value: string) => {
        setExpiresBefore(value)
        setPage(1)
    }, [])

    const handleInviterAdminFilterChange = useCallback((id: string | undefined) => {
        setInviterAdminFilter(id)
        setPage(1)
    }, [])

    const clearFilters = useCallback(() => {
        setStatusFilter(undefined)
        setRoleFilter(undefined)
        setExpiresAfter("")
        setExpiresBefore("")
        setInviterAdminFilter(undefined)
        setPage(1)
    }, [])

    const hasActiveFilters =
        statusFilter || roleFilter || expiresAfter || expiresBefore || inviterAdminFilter

    const [adminsResult] = useResolver(
        useCallback(async () => {
            return await api.projectAdmins.search(project.id, { limit: 100 })
        }, [project.id]),
    )

    const [result, , reload] = useResolver(
        useCallback(async () => {
            return await api.invites.list(project.id, {
                limit,
                offset: (page - 1) * limit,
                search: debouncedQuery || undefined,
                status: statusFilter,
                role: roleFilter,
                expires_after: expiresAfter || undefined,
                expires_before: expiresBefore || undefined,
                inviter_admin_id: inviterAdminFilter,
            })
        }, [
            project.id,
            debouncedQuery,
            statusFilter,
            roleFilter,
            expiresAfter,
            expiresBefore,
            inviterAdminFilter,
            page,
        ]),
    )

    const invites = result?.results ?? []
    const total = result?.total ?? 0
    const totalPages = Math.ceil(total / limit)
    const hasNextPage = page < totalPages
    const hasPrevPage = page > 1

    const handleRevoke = async (token: string) => {
        if (
            confirm(t("revoke_invite_confirmation", "Are you sure you want to revoke this invite?"))
        ) {
            await api.invites.revoke(project.id, token)
            toast.success(t("invite_revoked", "Invite revoked"))
            await reload()
        }
    }

    const handleCopyLink = async (token: string) => {
        const inviteLink = `${window.location.origin}/invites/${token}`
        await navigator.clipboard.writeText(inviteLink)
        toast.success(t("copied_invite_link", "Copied invite link"))
    }

    const formatDate = (dateStr: string | null | undefined) => {
        if (!dateStr) return "—"
        return new Date(dateStr).toLocaleDateString(undefined, {
            year: "numeric",
            month: "short",
            day: "numeric",
            hour: "2-digit",
            minute: "2-digit",
        })
    }

    const mashTokenNonce = (nonce: string, token: string) => {
        try {
            const nonceBytes = Uint8Array.from(
                atob(nonce.replace(/-/g, "+").replace(/_/g, "/")),
                (c) => c.charCodeAt(0),
            )
            const tokenBytes = Uint8Array.from(
                atob(token.replace(/-/g, "+").replace(/_/g, "/")),
                (c) => c.charCodeAt(0),
            )

            console.log("nonceBytes", nonceBytes)
            console.log("tokenBytes", tokenBytes)

            const mashed = new Uint8Array(nonceBytes.length + tokenBytes.length)
            mashed.set(nonceBytes)
            mashed.set(tokenBytes, nonceBytes.length)

            console.log("mashed", mashed, "mashed.length", mashed.length)

            return btoa(String.fromCharCode(...mashed))
                .replace(/\+/g, "-")
                .replace(/\//g, "_")
                .replace(/=/g, "")
        } catch (e) {
            console.error("mash failed", e)
            return nonce + token
        }
    }

    const getInviteStatus = (invite: ProjectInvite) => {
        if (invite.revoked_at) {
            return { status: "revoked", icon: XCircle, className: "text-red-500" }
        }
        if (invite.accepted_at) {
            return { status: "accepted", icon: CheckCircle, className: "text-green-500" }
        }
        if (new Date(invite.expires_at) < new Date()) {
            return { status: "expired", icon: ClockFading, className: "text-muted-foreground" }
        }
        return { status: "pending", icon: Clock, className: "text-blue-500" }
    }

    const canCreateInvite = ["editor", "admin", "owner"].includes(project.role ?? "")

    return (
        <div className="flex flex-col gap-6">
            <div className="flex items-center justify-between">
                <h2 className="text-2xl font-semibold tracking-tight">{t("invites")}</h2>
            </div>

            <div className="flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-3 sm:gap-4">
                <div className="flex items-center gap-2 flex-1 sm:max-w-xl">
                    <div className="relative flex-1">
                        <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                        <Input
                            placeholder={t("search")}
                            value={searchQuery}
                            onChange={(e) => handleSearch(e.target.value)}
                            className="pl-9"
                        />
                    </div>
                    <Popover>
                        <PopoverTrigger asChild>
                            <Button
                                variant="outline"
                                aria-label={t("filter", "Filter")}
                                className={`relative ${hasActiveFilters ? "border-primary text-primary" : ""}`}
                            >
                                <SlidersHorizontal className="mr-2 h-4 w-4" />
                                {t("filter")}
                                {hasActiveFilters && (
                                    <span className="absolute -top-1 -right-1 rounded-full bg-primary w-2 h-2" />
                                )}
                            </Button>
                        </PopoverTrigger>
                        <PopoverContent align="start" className="w-96 p-4">
                            <div className="grid gap-4">
                                <div className="grid gap-2">
                                    <Label className="text-xs font-medium">{t("status")}</Label>
                                    <div className="flex flex-wrap gap-1">
                                        {["all", "pending", "accepted", "expired", "revoked"].map(
                                            (status) => (
                                                <Button
                                                    key={status}
                                                    variant={
                                                        statusFilter === status ||
                                                        (!statusFilter && status === "all")
                                                            ? "secondary"
                                                            : "ghost"
                                                    }
                                                    size="sm"
                                                    className="h-7 px-2 text-xs"
                                                    onClick={() => handleStatusFilterChange(status)}
                                                >
                                                    {status === "all" ? t("all", "All") : t(status)}
                                                </Button>
                                            ),
                                        )}
                                    </div>
                                </div>
                                <div className="grid gap-2">
                                    <Label className="text-xs font-medium">{t("role")}</Label>
                                    <div className="flex flex-wrap gap-1">
                                        {["all", ...projectRoles].map((role) => (
                                            <Button
                                                key={role}
                                                variant={
                                                    roleFilter === role ||
                                                    (!roleFilter && role === "all")
                                                        ? "secondary"
                                                        : "ghost"
                                                }
                                                size="sm"
                                                className="h-7 px-2 text-xs"
                                                onClick={() =>
                                                    handleRoleFilterChange(role as string)
                                                }
                                            >
                                                {role === "all"
                                                    ? t("all", "All")
                                                    : snakeToTitle(role)}
                                            </Button>
                                        ))}
                                    </div>
                                </div>
                                <div className="grid gap-2">
                                    <Label className="text-xs font-medium">
                                        {t("invited_by", "Invited by")}
                                    </Label>
                                    <div className="flex flex-wrap gap-1">
                                        <Button
                                            variant={!inviterAdminFilter ? "secondary" : "ghost"}
                                            size="sm"
                                            className="h-7 px-2 text-xs"
                                            onClick={() =>
                                                handleInviterAdminFilterChange(undefined)
                                            }
                                        >
                                            {t("all", "All")}
                                        </Button>
                                        {adminsResult?.results.map((admin) => (
                                            <Button
                                                key={admin.admin_id}
                                                variant={
                                                    inviterAdminFilter === admin.admin_id
                                                        ? "secondary"
                                                        : "ghost"
                                                }
                                                size="sm"
                                                className="h-7 px-2 text-xs"
                                                onClick={() =>
                                                    handleInviterAdminFilterChange(admin.admin_id)
                                                }
                                            >
                                                {admin.email}
                                            </Button>
                                        ))}
                                    </div>
                                </div>
                                <div className="grid gap-2">
                                    <Label className="text-xs font-medium">{t("expires")}</Label>
                                    <div className="flex items-center gap-2">
                                        <Input
                                            type="date"
                                            value={expiresAfter}
                                            onChange={(e) =>
                                                handleExpiresAfterChange(e.target.value)
                                            }
                                            className="h-8 text-xs"
                                            placeholder={t("after", "After")}
                                        />
                                        <span className="text-muted-foreground">-</span>
                                        <Input
                                            type="date"
                                            value={expiresBefore}
                                            onChange={(e) =>
                                                handleExpiresBeforeChange(e.target.value)
                                            }
                                            className="h-8 text-xs"
                                            placeholder={t("before", "Before")}
                                        />
                                    </div>
                                </div>
                                {hasActiveFilters && (
                                    <Button
                                        variant="ghost"
                                        size="sm"
                                        className="justify-start text-xs w-fit"
                                        onClick={clearFilters}
                                    >
                                        <X className="mr-2 h-3 w-3" />
                                        {t("clear_filters", "Clear filters")}
                                    </Button>
                                )}
                            </div>
                        </PopoverContent>
                    </Popover>
                </div>
                <Button
                    onClick={() => setIsCreating(true)}
                    disabled={!canCreateInvite}
                    aria-label={t("create_invite", "Create invite")}
                >
                    <Plus className="mr-2 h-4 w-4" />
                    {t("create_invite")}
                </Button>
            </div>

            {!canCreateInvite && (
                <p className="text-sm text-muted-foreground bg-muted p-3 rounded-lg">
                    {t("invite_permission_denied", "You need admin role to create invites.")}
                </p>
            )}

            <div className="rounded-lg border bg-card">
                <Table>
                    <TableHeader>
                        <TableRow>
                            <TableHead>{t("invitee_email", "Invitee")}</TableHead>
                            <TableHead>{t("role")}</TableHead>
                            <TableHead className="hidden lg:table-cell">
                                {t("invited_by", "Invited by")}
                            </TableHead>
                            <TableHead className="hidden md:table-cell">{t("expires")}</TableHead>
                            <TableHead className="hidden lg:table-cell">{t("status")}</TableHead>
                            <TableHead className="w-[70px]" />
                        </TableRow>
                    </TableHeader>
                    <TableBody>
                        {!result ? (
                            Array.from({ length: 3 }).map((_, i) => (
                                <TableRow key={i}>
                                    <TableCell>
                                        <Skeleton className="h-4 w-40" />
                                    </TableCell>
                                    <TableCell>
                                        <Skeleton className="h-4 w-16" />
                                    </TableCell>
                                    <TableCell className="hidden lg:table-cell">
                                        <Skeleton className="h-4 w-32" />
                                    </TableCell>
                                    <TableCell className="hidden md:table-cell">
                                        <Skeleton className="h-4 w-28" />
                                    </TableCell>
                                    <TableCell className="hidden lg:table-cell">
                                        <Skeleton className="h-4 w-20" />
                                    </TableCell>
                                    <TableCell>
                                        <Skeleton className="h-4 w-8" />
                                    </TableCell>
                                </TableRow>
                            ))
                        ) : invites.length === 0 ? (
                            <TableRow>
                                <TableCell colSpan={6} className="h-32 text-center">
                                    <div className="flex flex-col items-center gap-2 text-muted-foreground">
                                        <UserPlus className="h-8 w-8" />
                                        <p>
                                            {debouncedQuery
                                                ? t("no_results")
                                                : t("no_invites_yet", "No invites yet")}
                                        </p>
                                        {!debouncedQuery && canCreateInvite && (
                                            <Button
                                                variant="outline"
                                                size="sm"
                                                onClick={() => setIsCreating(true)}
                                                className="mt-2"
                                                aria-label={t(
                                                    "create_invite_from_empty",
                                                    "Create invite from empty state",
                                                )}
                                            >
                                                <Plus className="mr-2 h-4 w-4" />
                                                {t("create_invite")}
                                            </Button>
                                        )}
                                    </div>
                                </TableCell>
                            </TableRow>
                        ) : (
                            invites.map((invite) => {
                                const {
                                    status,
                                    icon: StatusIcon,
                                    className: statusClassName,
                                } = getInviteStatus(invite)
                                return (
                                    <TableRow key={invite.id}>
                                        <TableCell>
                                            <div className="flex items-center gap-2">
                                                <Mail className="h-4 w-4 text-muted-foreground shrink-0" />
                                                <span className="font-medium truncate max-w-[200px]">
                                                    {invite.invitee_email}
                                                </span>
                                            </div>
                                        </TableCell>
                                        <TableCell className="text-muted-foreground">
                                            {snakeToTitle(invite.role)}
                                        </TableCell>
                                        <TableCell className="hidden lg:table-cell">
                                            <div className="flex items-center gap-2">
                                                <UserPlus className="h-3.5 w-3.5 text-muted-foreground shrink-0" />
                                                <span className="text-sm text-muted-foreground truncate max-w-45">
                                                    {invite.inviter_admin_email ?? "—"}
                                                </span>
                                            </div>
                                        </TableCell>
                                        <TableCell className="hidden md:table-cell text-muted-foreground">
                                            {formatDate(invite.expires_at)}
                                        </TableCell>
                                        <TableCell className="hidden lg:table-cell">
                                            <div
                                                className={`flex items-center gap-1.5 ${statusClassName}`}
                                            >
                                                <StatusIcon className="h-4 w-4" />
                                                <span className="capitalize">{status}</span>
                                            </div>
                                        </TableCell>
                                        <TableCell>
                                            <DropdownMenu>
                                                <DropdownMenuTrigger asChild>
                                                    <Button
                                                        variant="ghost"
                                                        className="h-8 w-8 p-0"
                                                        onClick={(e) => e.stopPropagation()}
                                                        aria-label={t("options")}
                                                    >
                                                        <MoreHorizontal className="h-4 w-4" />
                                                    </Button>
                                                </DropdownMenuTrigger>
                                                <DropdownMenuContent align="end">
                                                    <DropdownMenuItem
                                                        onClick={async (e) => {
                                                            e.stopPropagation()
                                                            await handleCopyLink(
                                                                mashTokenNonce(
                                                                    invite.nonce,
                                                                    invite.token,
                                                                ),
                                                            )
                                                        }}
                                                    >
                                                        <Copy className="mr-2 h-4 w-4" />
                                                        {t("copy_link")}
                                                    </DropdownMenuItem>
                                                    {!invite.revoked_at &&
                                                        !invite.accepted_at &&
                                                        canCreateInvite && (
                                                            <DropdownMenuItem
                                                                className="text-destructive"
                                                                onClick={async (e) => {
                                                                    e.stopPropagation()
                                                                    await handleRevoke(
                                                                        mashTokenNonce(
                                                                            invite.nonce,
                                                                            invite.token,
                                                                        ),
                                                                    )
                                                                }}
                                                            >
                                                                <XCircle className="mr-2 h-4 w-4" />
                                                                {t("revoke")}
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

                {total > 0 && (
                    <div className="flex items-center justify-between border-t px-4 py-3">
                        <p className="text-sm text-muted-foreground">
                            {total} {total === 1 ? t("invite", "invite") : t("invites")}
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
                                    <ChevronRight className="h-4 w-4 sm:mr-1" />
                                </Button>
                            </div>
                        )}
                    </div>
                )}
            </div>

            <Dialog
                open={!!createdInviteToken}
                onOpenChange={(open) => {
                    if (!open) setCreatedInviteToken(null)
                }}
            >
                <DialogContent className="sm:max-w-md">
                    <DialogHeader>
                        <DialogTitle>{t("invite_created", "Invite created")}</DialogTitle>
                        <DialogDescription>
                            {t(
                                "invite_link_description",
                                "Share this link with the person you're inviting.",
                            )}
                        </DialogDescription>
                    </DialogHeader>
                    <div className="flex items-center gap-2">
                        <Input
                            readOnly
                            value={
                                createdInviteToken
                                    ? `${window.location.origin}/invites/${createdInviteToken}`
                                    : ""
                            }
                            className="flex-1 text-sm"
                            onClick={(e) => (e.target as HTMLInputElement).select()}
                        />
                        <Button
                            type="button"
                            size="icon"
                            variant="outline"
                            onClick={async () => {
                                if (!createdInviteToken) return
                                await handleCopyLink(createdInviteToken)
                            }}
                            aria-label={t("copy_link")}
                        >
                            <Copy className="h-4 w-4" />
                        </Button>
                    </div>
                    <DialogFooter>
                        <Button onClick={() => setCreatedInviteToken(null)}>
                            {t("done", "Done")}
                        </Button>
                    </DialogFooter>
                </DialogContent>
            </Dialog>

            <InviteDialog
                isOpen={isCreating}
                onClose={() => setIsCreating(false)}
                onSave={async (data) => {
                    setIsSaving(true)
                    try {
                        const invite = await api.invites.create(project.id, data)
                        await reload()
                        setIsCreating(false)
                        setCreatedInviteToken(mashTokenNonce(invite.nonce, invite.token))
                    } finally {
                        setIsSaving(false)
                    }
                }}
                isSaving={isSaving}
                userRole={project.role}
            />
        </div>
    )
}
