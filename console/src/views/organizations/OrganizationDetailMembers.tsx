import React, { useContext, useRef, useState } from "react"
import { useTranslation } from "react-i18next"
import { useQuery } from "@tanstack/react-query"
import {
    Users,
    Plus,
    Trash2,
    Search,
    ExternalLink,
    ChevronDown,
    ChevronRight,
    MoreHorizontal,
} from "lucide-react"
import { ProjectContext, OrganizationContext } from "../../contexts"
import { useRoute } from "@/hooks/use-route"
import type { OrganizationMember } from "../../oapi/client"
import { getUserDisplayName, getUserInitials } from "@/lib/name"
import type { User as UserType } from "../../types"
import { LoadErrorRow } from "./LoadErrorRow"
import { MemberExpandedRow } from "./MemberExpandedRow"
import { PageNumbers } from "./PageNumbers"
import { RemoveMemberDialog } from "./RemoveMemberDialog"
import { organizationMembersQuery, useSaveMember } from "./queries"

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
import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { UserLookup } from "../users/UserLookup"
import { cn } from "@/utils"

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
    const [editedMemberData, setEditedMemberData] = useState<
        Record<string, Record<string, unknown>>
    >({})
    const addMemberMutation = useSaveMember(
        t("organization_member_add_failed", "Couldn't add the member"),
    )
    const saveMemberData = useSaveMember(
        t("organization_member_save_failed", "Couldn't save the member's attributes"),
    )
    // UserLookup closes itself once onSelected resolves, and a rejection would
    // go unhandled, so a failed add resolves and vetoes that close here.
    const keepLookupOpen = useRef(false)

    const limit = 25

    const list = useQuery(
        organizationMembersQuery(project.id, organization.id, {
            limit,
            offset: (page - 1) * limit,
            search: debouncedQuery,
        }),
    )

    const members = list.data?.results ?? []
    const total = list.data?.total ?? 0
    const totalPages = Math.ceil(total / limit)

    const handleSearch = (e: React.FormEvent) => {
        e.preventDefault()
        setDebouncedQuery(searchQuery)
        setPage(1)
    }

    const addMember = async (user: UserType) => {
        try {
            await addMemberMutation.mutateAsync({ user_id: user.id })
            setIsAddMemberOpen(false)
        } catch {
            // The mutation cache already showed the error.
            keepLookupOpen.current = true
        }
    }

    const closeLookup = (open: boolean) => {
        if (!open && keepLookupOpen.current) {
            keepLookupOpen.current = false
            return
        }
        setIsAddMemberOpen(open)
    }

    // On failure the mutation cache shows the error and the edits are kept.
    const updateMemberData = (member: OrganizationMember) => {
        const data = editedMemberData[member.id]
        if (!data) return
        saveMemberData.mutate(
            { user_id: member.id, data },
            { onSuccess: () => discardMemberChanges(member.id) },
        )
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

    const toggleExpand = (memberId: string) => {
        setExpandedMemberId(expandedMemberId === memberId ? null : memberId)
    }

    return (
        <div className="space-y-4">
            {/* Section Header */}
            <div className="flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-3 sm:gap-4">
                <form onSubmit={handleSearch} className="flex-1 sm:max-w-sm">
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
                <Button
                    onClick={() => setIsAddMemberOpen(true)}
                    className="flex-1 sm:flex-initial"
                    aria-label={t("add_member")}
                >
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

                            <TableHead className="hidden md:table-cell w-20">
                                {t("attributes")}
                            </TableHead>
                            <TableHead className="w-12"></TableHead>
                        </TableRow>
                    </TableHeader>
                    <TableBody>
                        {list.isPending ? (
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

                                    <TableCell className="hidden md:table-cell">
                                        <Skeleton className="h-4 w-12" />
                                    </TableCell>
                                    <TableCell></TableCell>
                                </TableRow>
                            ))
                        ) : list.isError ? (
                            <LoadErrorRow
                                colSpan={4}
                                message={t(
                                    "organization_members_load_failed",
                                    "Couldn't load members.",
                                )}
                                onRetry={() => list.refetch()}
                            />
                        ) : members.length === 0 ? (
                            <TableRow>
                                <TableCell colSpan={4} className="h-48">
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
                                                aria-label={t(
                                                    "add_member_from_empty_state",
                                                    "Add member from empty state",
                                                )}
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
                                                        {getUserInitials(member)}
                                                    </div>
                                                    <span className="font-medium">
                                                        {getUserDisplayName(member)}
                                                    </span>
                                                </div>
                                            </TableCell>

                                            <TableCell className="hidden md:table-cell">
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
                                                isSaving={
                                                    saveMemberData.isPending &&
                                                    saveMemberData.variables?.user_id === member.id
                                                }
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
                        <PageNumbers page={page} totalPages={totalPages} onPageChange={setPage} />
                    )}
                </div>
            </div>

            {/* Add Member Dialog */}
            <UserLookup open={isAddMemberOpen} onClose={closeLookup} onSelected={addMember} />

            <RemoveMemberDialog
                member={memberToRemove}
                onClose={() => setMemberToRemove(null)}
                onRemoved={() => {
                    setMemberToRemove(null)
                    setExpandedMemberId(null)
                }}
            />
        </div>
    )
}
