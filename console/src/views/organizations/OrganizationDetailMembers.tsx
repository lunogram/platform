import React, { useCallback, useContext, useState } from "react"
import { useTranslation } from "react-i18next"
import {
    Users,
    Plus,
    Trash2,
    Search,
    ExternalLink,
    ChevronDown,
    ChevronRight,
    ChevronLeft,
    MoreHorizontal,
} from "lucide-react"
import { ProjectContext, OrganizationContext } from "../../contexts"
import { useResolver } from "../../hooks"
import { useRoute } from "../router"
import oapiClient, { type OrganizationMember } from "../../oapi/client"
import type { User as UserType } from "../../types"

import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from "@/components/ui/dialog"
import {
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from "@/components/ui/table"
import { Skeleton } from "@/components/ui/skeleton"
import { AttributeEditor } from "@/components/ui/attribute-editor"
import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { UserLookup } from "../users/UserLookup"
import { cn } from "@/utils"

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

interface MemberExpandedRowProps {
    member: OrganizationMember
    memberData: Record<string, unknown>
    hasChanges: boolean
    isSaving: boolean
    onDataChange: (data: Record<string, unknown>) => void
    onSave: () => void
    onDiscard: () => void
}

function MemberExpandedRow({
    member: _member,
    memberData,
    hasChanges,
    isSaving,
    onDataChange,
    onSave,
    onDiscard,
}: MemberExpandedRowProps) {
    const { t } = useTranslation()

    return (
        <TableRow className="bg-muted/50 hover:bg-muted/50">
            <TableCell colSpan={5} className="p-0">
                <div className="px-6 py-4 flex flex-col gap-3">
                    {/* Member Attributes */}
                    <p className="text-xs font-medium text-muted-foreground uppercase tracking-wider">
                        {t("member_attributes")}
                    </p>
                    <AttributeEditor
                        value={memberData}
                        onChange={onDataChange}
                        emptyTitle={t("no_member_attributes")}
                        emptyDescription={t("add_member_attributes_description")}
                    />

                    {/* Save/Discard Actions */}
                    {hasChanges && (
                        <div className="flex items-center gap-2">
                            <Button
                                size="sm"
                                onClick={(e) => {
                                    e.stopPropagation()
                                    onSave()
                                }}
                                disabled={isSaving}
                            >
                                {isSaving ? t("saving") : t("save_changes")}
                            </Button>
                            <Button
                                variant="outline"
                                size="sm"
                                onClick={(e) => {
                                    e.stopPropagation()
                                    onDiscard()
                                }}
                                disabled={isSaving}
                            >
                                {t("discard")}
                            </Button>
                        </div>
                    )}
                </div>
            </TableCell>
        </TableRow>
    )
}

export default function OrganizationDetailMembers() {
    const { t } = useTranslation()
    const route = useRoute()
    const [project] = useContext(ProjectContext)
    const [organization] = useContext(OrganizationContext)

    const [searchQuery, setSearchQuery] = useState("")
    const [debouncedQuery, setDebouncedQuery] = useState("")
    const [page, setPage] = useState(1)
    const [isAddMemberOpen, setIsAddMemberOpen] = useState(false)
    const [expandedMemberId, setExpandedMemberId] = useState<string | null>(null)
    const [memberToRemove, setMemberToRemove] = useState<OrganizationMember | null>(null)
    const [isRemoving, setIsRemoving] = useState(false)
    const [editedMemberData, setEditedMemberData] = useState<
        Record<string, Record<string, unknown>>
    >({})
    const [savingMemberId, setSavingMemberId] = useState<string | null>(null)

    const limit = 25

    const [result, , reload] = useResolver(
        useCallback(async () => {
            const { data } = await oapiClient.GET(
                "/api/admin/projects/{projectID}/subjects/organizations/{organizationID}/users",
                {
                    params: {
                        path: {
                            projectID: project.id,
                            organizationID: organization.id,
                        },
                        query: {
                            limit,
                            offset: (page - 1) * limit,
                            search: debouncedQuery || undefined,
                        },
                    },
                },
            )
            return {
                members: data?.results ?? [],
                total: data?.total ?? 0,
            }
        }, [project.id, organization.id, page, debouncedQuery]),
    )

    const members = result?.members
    const total = result?.total ?? 0
    const totalPages = Math.ceil(total / limit)
    const hasNextPage = page < totalPages
    const hasPrevPage = page > 1

    const handleSearch = (e: React.FormEvent) => {
        e.preventDefault()
        setDebouncedQuery(searchQuery)
        setPage(1)
    }

    const addMember = async (user: UserType) => {
        await oapiClient.POST(
            "/api/admin/projects/{projectID}/subjects/organizations/{organizationID}/users",
            {
                params: {
                    path: {
                        projectID: project.id,
                        organizationID: organization.id,
                    },
                },
                body: { user_id: user.id },
            },
        )
        await reload()
        setIsAddMemberOpen(false)
    }

    const removeMember = async () => {
        if (!memberToRemove) return
        setIsRemoving(true)
        try {
            await oapiClient.DELETE(
                "/api/admin/projects/{projectID}/subjects/organizations/{organizationID}/users/{userID}",
                {
                    params: {
                        path: {
                            projectID: project.id,
                            organizationID: organization.id,
                            userID: memberToRemove.id,
                        },
                    },
                },
            )
            await reload()
            setMemberToRemove(null)
            setExpandedMemberId(null)
        } finally {
            setIsRemoving(false)
        }
    }

    const updateMemberData = async (member: OrganizationMember) => {
        const data = editedMemberData[member.id]
        if (!data) return

        setSavingMemberId(member.id)
        try {
            await oapiClient.POST(
                "/api/admin/projects/{projectID}/subjects/organizations/{organizationID}/users",
                {
                    params: {
                        path: {
                            projectID: project.id,
                            organizationID: organization.id,
                        },
                    },
                    body: {
                        user_id: member.id,
                        data,
                    },
                },
            )
            setEditedMemberData((prev) => {
                const next = { ...prev }
                delete next[member.id]
                return next
            })
            await reload()
        } finally {
            setSavingMemberId(null)
        }
    }

    const getMemberOrganizationData = (member: OrganizationMember): Record<string, unknown> => {
        return (
            editedMemberData[member.id] ??
            (member.organization_data as Record<string, unknown>) ??
            {}
        )
    }

    const hasMemberChanges = (member: OrganizationMember): boolean => {
        if (!(member.id in editedMemberData)) return false
        const original = (member.organization_data as Record<string, unknown>) ?? {}
        const edited = editedMemberData[member.id]
        return JSON.stringify(original) !== JSON.stringify(edited)
    }

    const handleMemberDataChange = (memberId: string, data: Record<string, unknown>) => {
        setEditedMemberData((prev) => ({
            ...prev,
            [memberId]: data,
        }))
    }

    const discardMemberChanges = (memberId: string) => {
        setEditedMemberData((prev) => {
            const next = { ...prev }
            delete next[memberId]
            return next
        })
    }

    const getMemberDisplayName = (member: OrganizationMember) => {
        const data = member.data as Record<string, unknown>
        if (data?.full_name) return data.full_name as string
        if (member.email) return member.email
        return member.external_id ?? member.anonymous_id ?? "Unknown"
    }

    const getMemberInitials = (member: OrganizationMember) => {
        const name = getMemberDisplayName(member)
        return name.substring(0, 2).toUpperCase()
    }

    const toggleExpand = (memberId: string) => {
        setExpandedMemberId(expandedMemberId === memberId ? null : memberId)
    }

    return (
        <div className="space-y-4">
            {/* Section Header */}
            <div className="flex items-center justify-between gap-4">
                <form onSubmit={handleSearch} className="flex-1 max-w-sm">
                    <div className="relative">
                        <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                        <Input
                            placeholder={t("search_members")}
                            value={searchQuery}
                            onChange={(e) => setSearchQuery(e.target.value)}
                            className="pl-9"
                        />
                    </div>
                </form>
                <Button onClick={() => setIsAddMemberOpen(true)}>
                    <Plus className="mr-2 h-4 w-4" />
                    {t("add_member")}
                </Button>
            </div>

            {/* Members Table */}
            <div className="border rounded-lg">
                <Table>
                    <TableHeader>
                        <TableRow>
                            <TableHead className="w-8 p-0"></TableHead>
                            <TableHead>{t("member")}</TableHead>
                            <TableHead className="w-32">{t("external_id")}</TableHead>
                            <TableHead className="w-20">{t("attributes")}</TableHead>
                            <TableHead className="w-12"></TableHead>
                        </TableRow>
                    </TableHeader>
                    <TableBody>
                        {members === undefined ? (
                            Array.from({ length: 5 }).map((_, i) => (
                                <TableRow key={i}>
                                    <TableCell className="p-0 pl-3">
                                        <Skeleton className="h-4 w-4" />
                                    </TableCell>
                                    <TableCell>
                                        <div className="flex items-center gap-3">
                                            <Skeleton className="h-8 w-8 rounded-full" />
                                            <Skeleton className="h-4 w-36" />
                                        </div>
                                    </TableCell>
                                    <TableCell>
                                        <Skeleton className="h-4 w-20" />
                                    </TableCell>
                                    <TableCell>
                                        <Skeleton className="h-4 w-12" />
                                    </TableCell>
                                    <TableCell></TableCell>
                                </TableRow>
                            ))
                        ) : members.length === 0 ? (
                            <TableRow>
                                <TableCell colSpan={5} className="h-48">
                                    <div className="flex flex-col items-center justify-center">
                                        <div className="flex h-12 w-12 items-center justify-center rounded-full bg-muted mb-4">
                                            <Users className="h-6 w-6 text-muted-foreground" />
                                        </div>
                                        <p className="font-medium mb-1">
                                            {debouncedQuery
                                                ? t("no_members_found")
                                                : t("no_members_yet")}
                                        </p>
                                        {!debouncedQuery && (
                                            <Button
                                                variant="outline"
                                                size="sm"
                                                onClick={() => setIsAddMemberOpen(true)}
                                                className="mt-4"
                                            >
                                                <Plus className="mr-2 h-4 w-4" />
                                                {t("add_member")}
                                            </Button>
                                        )}
                                    </div>
                                </TableCell>
                            </TableRow>
                        ) : (
                            members.map((member) => {
                                const isExpanded = expandedMemberId === member.id
                                const attrCount = Object.keys(member.organization_data ?? {}).length

                                return (
                                    <React.Fragment key={member.id}>
                                        <TableRow
                                            className={cn(
                                                "cursor-pointer group",
                                                isExpanded && "bg-muted/50",
                                            )}
                                            onClick={() => toggleExpand(member.id)}
                                        >
                                            <TableCell className="p-0 pl-3">
                                                {isExpanded ? (
                                                    <ChevronDown className="h-4 w-4 text-muted-foreground" />
                                                ) : (
                                                    <ChevronRight className="h-4 w-4 text-muted-foreground" />
                                                )}
                                            </TableCell>
                                            <TableCell>
                                                <div className="flex items-center gap-3">
                                                    <div className="flex h-8 w-8 items-center justify-center rounded-full bg-primary/10 text-primary text-xs font-medium shrink-0">
                                                        {getMemberInitials(member)}
                                                    </div>
                                                    <span className="font-medium">
                                                        {getMemberDisplayName(member)}
                                                    </span>
                                                </div>
                                            </TableCell>
                                            <TableCell>
                                                {member.external_id ? (
                                                    <code className="text-sm text-muted-foreground">
                                                        {member.external_id}
                                                    </code>
                                                ) : (
                                                    <span className="text-muted-foreground">—</span>
                                                )}
                                            </TableCell>
                                            <TableCell>
                                                {attrCount > 0 ? (
                                                    <span className="text-xs text-muted-foreground bg-muted px-2 py-0.5 rounded">
                                                        {attrCount}
                                                    </span>
                                                ) : (
                                                    <span className="text-muted-foreground">—</span>
                                                )}
                                            </TableCell>
                                            <TableCell className="p-0 pr-2">
                                                <DropdownMenu>
                                                    <DropdownMenuTrigger asChild>
                                                        <Button
                                                            variant="ghost"
                                                            size="sm"
                                                            className="h-8 w-8 p-0 opacity-0 group-hover:opacity-100 data-[state=open]:opacity-100"
                                                            onClick={(e) => e.stopPropagation()}
                                                        >
                                                            <MoreHorizontal className="h-4 w-4" />
                                                        </Button>
                                                    </DropdownMenuTrigger>
                                                    <DropdownMenuContent align="end">
                                                        <DropdownMenuItem
                                                            onClick={(e) => {
                                                                e.stopPropagation()
                                                                route(`users/${member.id}`)
                                                            }}
                                                        >
                                                            <ExternalLink className="mr-2 h-4 w-4" />
                                                            {t("view_user_profile")}
                                                        </DropdownMenuItem>
                                                        <DropdownMenuItem
                                                            onClick={(e) => {
                                                                e.stopPropagation()
                                                                setMemberToRemove(member)
                                                            }}
                                                            className="text-destructive focus:text-destructive"
                                                        >
                                                            <Trash2 className="mr-2 h-4 w-4" />
                                                            {t("remove_member")}
                                                        </DropdownMenuItem>
                                                    </DropdownMenuContent>
                                                </DropdownMenu>
                                            </TableCell>
                                        </TableRow>

                                        {/* Expanded Row */}
                                        {isExpanded && (
                                            <MemberExpandedRow
                                                key={`${member.id}-expanded`}
                                                member={member}
                                                memberData={getMemberOrganizationData(member)}
                                                hasChanges={hasMemberChanges(member)}
                                                isSaving={savingMemberId === member.id}
                                                onDataChange={(data) =>
                                                    handleMemberDataChange(member.id, data)
                                                }
                                                onSave={() => updateMemberData(member)}
                                                onDiscard={() => discardMemberChanges(member.id)}
                                            />
                                        )}
                                    </React.Fragment>
                                )
                            })
                        )}
                    </TableBody>
                </Table>

                {/* Pagination */}
                <div className="flex items-center justify-between border-t px-4 py-3">
                    <p className="text-sm text-muted-foreground">
                        {total} {total === 1 ? t("member") : t("members")}
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
            </div>

            {/* Add Member Dialog */}
            <UserLookup
                open={isAddMemberOpen}
                onClose={() => setIsAddMemberOpen(false)}
                onSelected={addMember}
            />

            {/* Remove Member Confirmation */}
            <Dialog
                open={memberToRemove !== null}
                onOpenChange={(open) => !open && setMemberToRemove(null)}
            >
                <DialogContent>
                    <DialogHeader>
                        <DialogTitle>{t("remove_member")}</DialogTitle>
                        <DialogDescription>
                            {t(
                                "remove_member_warning",
                                "Are you sure you want to remove this member from the organization?",
                            )}
                        </DialogDescription>
                    </DialogHeader>
                    {memberToRemove && (
                        <div className="py-4">
                            <div className="flex items-center gap-3 p-3 rounded-lg bg-muted">
                                <div className="flex h-9 w-9 items-center justify-center rounded-full bg-primary/10 text-primary text-sm font-medium">
                                    {getMemberInitials(memberToRemove)}
                                </div>
                                <div>
                                    <p className="font-medium">
                                        {getMemberDisplayName(memberToRemove)}
                                    </p>
                                    <p className="text-sm text-muted-foreground">
                                        {memberToRemove.email || memberToRemove.external_id}
                                    </p>
                                </div>
                            </div>
                        </div>
                    )}
                    <DialogFooter>
                        <Button
                            variant="outline"
                            onClick={() => setMemberToRemove(null)}
                            disabled={isRemoving}
                        >
                            {t("cancel")}
                        </Button>
                        <Button variant="destructive" onClick={removeMember} disabled={isRemoving}>
                            {isRemoving ? t("removing") : t("remove_member")}
                        </Button>
                    </DialogFooter>
                </DialogContent>
            </Dialog>
        </div>
    )
}
