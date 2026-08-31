import { useCallback, useContext, useRef, useState } from "react"
import { useTranslation } from "react-i18next"
import { Search, Users, MoreHorizontal, Trash2, ChevronLeft, ChevronRight } from "lucide-react"
import { toast } from "sonner"
import api from "@/api"
import { AdminContext, ProjectContext } from "@/contexts"
import { useResolver } from "@/hooks"
import { assignableProjectRoles, checkProjectRole, problemDetail, snakeToTitle } from "@/utils"
import type { ProjectAdmin, ProjectRole } from "@/types"

import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
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
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select"
import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import {
    AlertDialog,
    AlertDialogAction,
    AlertDialogCancel,
    AlertDialogContent,
    AlertDialogDescription,
    AlertDialogFooter,
    AlertDialogHeader,
    AlertDialogTitle,
} from "@/components/ui/alert-dialog"

interface MemberListProps {
    canManage: boolean
}

interface SelfRoleChange {
    member: ProjectAdmin
    role: ProjectRole
}

function memberName(member: ProjectAdmin) {
    return [member.first_name, member.last_name].filter(Boolean).join(" ")
}

// MemberList is the roster half of the Members screen: everybody who already
// has access to this project, the role they hold, and the controls to change or
// revoke it.
export default function MemberList({ canManage }: MemberListProps) {
    const { t } = useTranslation()
    const [project, setProject] = useContext(ProjectContext)
    const admin = useContext(AdminContext)

    const [searchQuery, setSearchQuery] = useState("")
    const [debouncedQuery, setDebouncedQuery] = useState("")
    const [page, setPage] = useState(1)
    const [pendingRemoval, setPendingRemoval] = useState<ProjectAdmin | null>(null)
    const [pendingSelfChange, setPendingSelfChange] = useState<SelfRoleChange | null>(null)
    const [savingAdminId, setSavingAdminId] = useState<string | null>(null)
    const limit = 10
    const searchTimeoutRef = useRef<ReturnType<typeof setTimeout>>(setTimeout(() => {}, 0))

    const handleSearch = useCallback((value: string) => {
        setSearchQuery(value)
        setPage(1)
        clearTimeout(searchTimeoutRef.current)
        searchTimeoutRef.current = setTimeout(() => setDebouncedQuery(value), 300)
    }, [])

    const [result, , reload] = useResolver(
        useCallback(async () => {
            return await api.projectAdmins.search(project.id, {
                limit,
                offset: (page - 1) * limit,
                search: debouncedQuery || undefined,
            })
        }, [project.id, debouncedQuery, page]),
    )

    const members = result?.results ?? []
    const total = result?.total ?? 0
    const totalPages = Math.ceil(total / limit)

    const assignableRoles = assignableProjectRoles(project.role)

    const handleRoleChange = async (member: ProjectAdmin, role: ProjectRole) => {
        if (role === member.role) return
        setSavingAdminId(member.admin_id)
        try {
            await api.projectAdmins.updateRole(project.id, member.admin_id, { role })
            toast.success(t("member_role_updated", "Role updated"))
            await reload()
            // The project's role is what gates every control on this screen, so
            // after re-roling yourself it has to be refetched rather than
            // assumed: an organization owner keeps project admin by inheritance.
            if (member.admin_id === admin?.id) {
                setProject(await api.projects.get(project.id))
            }
        } catch (err) {
            toast.error(
                problemDetail(err) ?? t("member_role_update_failed", "Couldn't change the role"),
            )
        } finally {
            setSavingAdminId(null)
        }
    }

    const requestRoleChange = (member: ProjectAdmin, role: ProjectRole) => {
        if (role === member.role) return
        // Dropping yourself below admin takes this screen's controls with it, and
        // only somebody else can hand them back. Say so before it happens.
        if (member.admin_id === admin?.id && !checkProjectRole("admin", role)) {
            setPendingSelfChange({ member, role })
            return
        }
        void handleRoleChange(member, role)
    }

    const confirmSelfChange = async () => {
        if (!pendingSelfChange) return
        const { member, role } = pendingSelfChange
        setPendingSelfChange(null)
        await handleRoleChange(member, role)
    }

    const confirmRemoval = async () => {
        if (!pendingRemoval) return
        try {
            await api.projectAdmins.remove(project.id, pendingRemoval.admin_id)
            toast.success(t("member_removed", "Member removed"))
            await reload()
        } catch (err) {
            toast.error(
                problemDetail(err) ?? t("member_remove_failed", "Couldn't remove the member"),
            )
        } finally {
            setPendingRemoval(null)
        }
    }

    return (
        <section className="flex flex-col gap-4">
            <div className="relative sm:max-w-md">
                <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                <Input
                    placeholder={t("search")}
                    value={searchQuery}
                    onChange={(e) => handleSearch(e.target.value)}
                    className="pl-9"
                />
            </div>

            <div className="overflow-hidden rounded-xl border bg-card">
                <Table>
                    <TableHeader>
                        <TableRow className="hover:bg-transparent">
                            <TableHead>{t("member")}</TableHead>
                            <TableHead className="w-[180px]">{t("role")}</TableHead>
                            <TableHead className="hidden md:table-cell">
                                {t("member_since", "Member since")}
                            </TableHead>
                            <TableHead className="w-[70px]" />
                        </TableRow>
                    </TableHeader>
                    <TableBody>
                        {!result ? (
                            Array.from({ length: 3 }).map((_, i) => (
                                <TableRow key={i} className="hover:bg-transparent">
                                    <TableCell>
                                        <Skeleton className="h-4 w-48" />
                                    </TableCell>
                                    <TableCell>
                                        <Skeleton className="h-4 w-24" />
                                    </TableCell>
                                    <TableCell className="hidden md:table-cell">
                                        <Skeleton className="h-4 w-28" />
                                    </TableCell>
                                    <TableCell>
                                        <Skeleton className="h-4 w-8" />
                                    </TableCell>
                                </TableRow>
                            ))
                        ) : members.length === 0 ? (
                            <TableRow className="hover:bg-transparent">
                                <TableCell colSpan={4} className="h-32 text-center">
                                    <div className="flex flex-col items-center gap-2 text-muted-foreground">
                                        <Users className="h-7 w-7" strokeWidth={1.5} />
                                        <p className="text-sm">
                                            {debouncedQuery
                                                ? t("no_members_found")
                                                : t("no_members_yet")}
                                        </p>
                                    </div>
                                </TableCell>
                            </TableRow>
                        ) : (
                            members.map((member) => {
                                const isSelf = member.admin_id === admin?.id
                                const name = memberName(member)
                                // A member holding a role above your own is not
                                // yours to re-role: the server refuses it, so the
                                // dropdown must not offer it either.
                                const outranksYou = !assignableRoles.includes(member.role)
                                return (
                                    <TableRow key={member.id}>
                                        <TableCell>
                                            <div className="flex flex-col">
                                                <div className="flex items-center gap-2">
                                                    <span className="font-medium">
                                                        {name || member.email}
                                                    </span>
                                                    {isSelf && (
                                                        <Badge
                                                            variant="secondary"
                                                            className="text-xs"
                                                        >
                                                            {t("you", "You")}
                                                        </Badge>
                                                    )}
                                                </div>
                                                {name && (
                                                    <span className="text-sm text-muted-foreground">
                                                        {member.email}
                                                    </span>
                                                )}
                                            </div>
                                        </TableCell>
                                        <TableCell>
                                            {canManage && !outranksYou ? (
                                                <Select
                                                    value={member.role}
                                                    disabled={savingAdminId === member.admin_id}
                                                    onValueChange={(role) =>
                                                        requestRoleChange(
                                                            member,
                                                            role as ProjectRole,
                                                        )
                                                    }
                                                >
                                                    <SelectTrigger
                                                        className="h-8"
                                                        aria-label={t("role")}
                                                    >
                                                        <SelectValue />
                                                    </SelectTrigger>
                                                    <SelectContent>
                                                        {assignableRoles.map((role) => (
                                                            <SelectItem key={role} value={role}>
                                                                {snakeToTitle(role)}
                                                            </SelectItem>
                                                        ))}
                                                    </SelectContent>
                                                </Select>
                                            ) : (
                                                <span className="text-muted-foreground">
                                                    {snakeToTitle(member.role)}
                                                </span>
                                            )}
                                        </TableCell>
                                        <TableCell className="hidden md:table-cell text-muted-foreground">
                                            {new Date(member.created_at).toLocaleDateString(
                                                undefined,
                                                { year: "numeric", month: "short", day: "numeric" },
                                            )}
                                        </TableCell>
                                        <TableCell>
                                            {canManage && !outranksYou && (
                                                <DropdownMenu>
                                                    <DropdownMenuTrigger asChild>
                                                        <Button
                                                            variant="ghost"
                                                            className="h-8 w-8 p-0"
                                                            aria-label={t("options")}
                                                        >
                                                            <MoreHorizontal className="h-4 w-4" />
                                                        </Button>
                                                    </DropdownMenuTrigger>
                                                    <DropdownMenuContent align="end">
                                                        <DropdownMenuItem
                                                            className="text-destructive"
                                                            onClick={() =>
                                                                setPendingRemoval(member)
                                                            }
                                                        >
                                                            <Trash2 className="mr-2 h-4 w-4" />
                                                            {t("remove")}
                                                        </DropdownMenuItem>
                                                    </DropdownMenuContent>
                                                </DropdownMenu>
                                            )}
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
                            {total} {total === 1 ? t("member") : t("members")}
                        </p>
                        {totalPages > 1 && (
                            <div className="flex items-center gap-2">
                                <Button
                                    variant="outline"
                                    size="sm"
                                    onClick={() => setPage((p) => p - 1)}
                                    disabled={page <= 1}
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
                                    disabled={page >= totalPages}
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

            <AlertDialog
                open={!!pendingRemoval}
                onOpenChange={(open) => {
                    if (!open) setPendingRemoval(null)
                }}
            >
                <AlertDialogContent>
                    <AlertDialogHeader>
                        <AlertDialogTitle>{t("remove_member")}</AlertDialogTitle>
                        <AlertDialogDescription>
                            {t("remove_member_confirmation", {
                                defaultValue:
                                    "{{name}} will immediately lose access to this project. They can be invited back at any time.",
                                name: pendingRemoval
                                    ? memberName(pendingRemoval) || pendingRemoval.email
                                    : "",
                            })}
                        </AlertDialogDescription>
                    </AlertDialogHeader>
                    <AlertDialogFooter>
                        <AlertDialogCancel>{t("cancel")}</AlertDialogCancel>
                        <AlertDialogAction onClick={confirmRemoval}>
                            {t("remove")}
                        </AlertDialogAction>
                    </AlertDialogFooter>
                </AlertDialogContent>
            </AlertDialog>

            <AlertDialog
                open={!!pendingSelfChange}
                onOpenChange={(open) => {
                    if (!open) setPendingSelfChange(null)
                }}
            >
                <AlertDialogContent>
                    <AlertDialogHeader>
                        <AlertDialogTitle>
                            {t("change_own_role", "Change your own role?")}
                        </AlertDialogTitle>
                        <AlertDialogDescription>
                            {t("change_own_role_confirmation", {
                                defaultValue:
                                    "You are moving yourself from {{current}} to {{next}}. Managing members is admin-only, so unless you are an owner or admin of the organization you will lose access to this screen, and another admin will have to change your role back.",
                                current: pendingSelfChange
                                    ? snakeToTitle(pendingSelfChange.member.role)
                                    : "",
                                next: pendingSelfChange ? snakeToTitle(pendingSelfChange.role) : "",
                            })}
                        </AlertDialogDescription>
                    </AlertDialogHeader>
                    <AlertDialogFooter>
                        <AlertDialogCancel>{t("cancel")}</AlertDialogCancel>
                        <AlertDialogAction onClick={confirmSelfChange}>
                            {t("change_role", "Change role")}
                        </AlertDialogAction>
                    </AlertDialogFooter>
                </AlertDialogContent>
            </AlertDialog>
        </section>
    )
}
