import { useCallback, useState, useRef } from "react"
import { useParams } from "react-router"
import { useTranslation } from "react-i18next"
import {
    Plus,
    Building2,
    Search,
    ChevronLeft,
    ChevronRight,
    ArrowRight,
    Users,
    Settings,
} from "lucide-react"
import { NIL } from "uuid"
import { useRoute } from "../router"
import { useResolver } from "../../hooks"
import { formatDate } from "../../utils"
import { getRandomColor } from "@/lib/colors"
import { getPrimaryExternalId } from "@/lib/name"
import { PreferencesContext } from "@/contexts/PreferencesContext"
import { useContext } from "react"
import { BuildingIcon } from "@/components/icons"
import type { UUID } from "@/types/common"
import oapiClient, { type Organization, type UpsertOrganization } from "../../oapi/client"

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
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from "@/components/ui/dialog"
import { Label } from "@/components/ui/label"
import { Skeleton } from "@/components/ui/skeleton"
import { AttributeEditor } from "@/components/ui/attribute-editor"

export default function Organizations() {
    const { projectId = NIL as UUID } = useParams<{ projectId: UUID }>()
    const { t } = useTranslation()
    const route = useRoute()
    const [preferences] = useContext(PreferencesContext)

    const [searchQuery, setSearchQuery] = useState("")
    const [debouncedQuery, setDebouncedQuery] = useState("")
    const [isCreateOpen, setIsCreateOpen] = useState(false)
    const [isCreating, setIsCreating] = useState(false)
    const [newOrgExternalId, setNewOrgExternalId] = useState("")
    const [newOrgName, setNewOrgName] = useState("")
    const [newOrgData, setNewOrgData] = useState<Record<string, unknown>>({})
    const [page, setPage] = useState(1)
    const limit = 25
    const searchTimeoutRef = useRef<ReturnType<typeof setTimeout>>()

    // Debounce search
    const handleSearch = useCallback((value: string) => {
        setSearchQuery(value)
        setPage(1) // Reset to first page on search
        clearTimeout(searchTimeoutRef.current)
        searchTimeoutRef.current = setTimeout(() => {
            setDebouncedQuery(value)
        }, 300)
    }, [])

    const [result, , reload] = useResolver(
        useCallback(async () => {
            const { data } = await oapiClient.GET(
                "/api/admin/projects/{projectID}/subjects/organizations",
                {
                    params: {
                        path: { projectID: projectId },
                        query: {
                            limit,
                            offset: (page - 1) * limit,
                            search: debouncedQuery || undefined,
                        },
                    },
                },
            )
            return data ?? null
        }, [projectId, debouncedQuery, page]),
    )

    const organizations = result?.results
    const total = result?.total ?? 0
    const totalPages = Math.ceil(total / limit)
    const hasNextPage = page < totalPages
    const hasPrevPage = page > 1

    const createOrganization = async () => {
        if (!newOrgExternalId.trim()) return

        setIsCreating(true)
        try {
            await oapiClient.POST("/api/admin/projects/{projectID}/subjects/organizations", {
                params: { path: { projectID: projectId } },
                body: {
                    identifier: [{ source: "default", external_id: newOrgExternalId.trim() }],
                    name: newOrgName.trim() || undefined,
                    data: Object.keys(newOrgData).length > 0 ? newOrgData : undefined,
                } as UpsertOrganization,
            })
            await reload()
            setIsCreateOpen(false)
            setNewOrgExternalId("")
            setNewOrgName("")
            setNewOrgData({})
        } finally {
            setIsCreating(false)
        }
    }

    const handleRowClick = (org: Organization) => {
        route(`organizations/${org.id}`)
    }

    return (
        <div className="flex flex-col gap-4 sm:gap-6 p-4 sm:p-6">
            {/* Header */}
            <div className="flex items-start gap-4">
                <div className="flex h-14 w-14 items-center justify-center rounded-xl shrink-0 bg-muted [&>svg]:h-7 [&>svg]:w-7 [&>svg]:text-muted-foreground">
                    <BuildingIcon />
                </div>
                <div className="space-y-1">
                    <h1 className="text-2xl font-semibold tracking-tight">{t("organizations")}</h1>
                    <p className="text-sm text-muted-foreground">
                        {t(
                            "organizations_description",
                            "Group and manage users by organization to target them collectively.",
                        )}
                    </p>
                </div>
            </div>

            {/* Search and Create */}
            <div className="flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-3 sm:gap-4">
                <div className="relative sm:max-w-sm flex-1">
                    <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                    <Input
                        placeholder={t("search_organizations")}
                        value={searchQuery}
                        onChange={(e) => handleSearch(e.target.value)}
                        className="pl-9"
                    />
                </div>
                <Button onClick={() => setIsCreateOpen(true)} className="flex-1 sm:flex-initial">
                    <Plus className="mr-2 h-4 w-4" />
                    {t("create_organization")}
                </Button>
            </div>

            {/* Table */}
            <div className="rounded-lg border bg-card">
                <Table>
                    <TableHeader>
                        <TableRow>
                            <TableHead>{t("name")}</TableHead>
                            <TableHead className="hidden sm:table-cell">
                                {t("created_at")}
                            </TableHead>
                            <TableHead className="hidden md:table-cell">
                                {t("updated_at")}
                            </TableHead>
                        </TableRow>
                    </TableHeader>
                    <TableBody>
                        {!organizations ? (
                            // Loading skeleton
                            Array.from({ length: 5 }).map((_, i) => (
                                <TableRow key={i}>
                                    <TableCell>
                                        <div className="flex items-center gap-3">
                                            <Skeleton className="h-8 w-8 rounded" />
                                            <div className="space-y-1">
                                                <Skeleton className="h-4 w-32" />
                                                <Skeleton className="h-3 w-20" />
                                            </div>
                                        </div>
                                    </TableCell>
                                    <TableCell className="hidden sm:table-cell">
                                        <Skeleton className="h-4 w-28" />
                                    </TableCell>
                                    <TableCell className="hidden md:table-cell">
                                        <Skeleton className="h-4 w-28" />
                                    </TableCell>
                                </TableRow>
                            ))
                        ) : organizations.length === 0 ? (
                            <TableRow>
                                <TableCell colSpan={3} className="h-32 text-center">
                                    <div className="flex flex-col items-center gap-2 text-muted-foreground">
                                        <Building2 className="h-8 w-8" />
                                        <p>
                                            {debouncedQuery
                                                ? t("no_organizations_found")
                                                : t("no_organizations_yet")}
                                        </p>
                                        {!debouncedQuery && (
                                            <Button
                                                variant="outline"
                                                size="sm"
                                                onClick={() => setIsCreateOpen(true)}
                                                className="mt-2"
                                            >
                                                <Plus className="mr-2 h-4 w-4" />
                                                {t("create_organization")}
                                            </Button>
                                        )}
                                    </div>
                                </TableCell>
                            </TableRow>
                        ) : (
                            organizations.map((org) => {
                                const orgColor = getRandomColor(
                                    getPrimaryExternalId(
                                        org as unknown as Record<string, unknown>,
                                    ) ?? org.id,
                                )
                                return (
                                    <TableRow
                                        key={org.id}
                                        className="cursor-pointer"
                                        onClick={() => handleRowClick(org)}
                                    >
                                        <TableCell>
                                            <div className="flex items-center gap-3">
                                                <div
                                                    className="flex h-8 w-8 items-center justify-center rounded-md shrink-0"
                                                    style={{ backgroundColor: orgColor }}
                                                >
                                                    <Building2 className="h-4 w-4 text-white" />
                                                </div>
                                                <div>
                                                    <div className="font-medium">
                                                        {org.name ||
                                                            getPrimaryExternalId(
                                                                org as unknown as Record<
                                                                    string,
                                                                    unknown
                                                                >,
                                                            )}
                                                    </div>
                                                    <div className="text-sm text-muted-foreground">
                                                        {getPrimaryExternalId(
                                                            org as unknown as Record<
                                                                string,
                                                                unknown
                                                            >,
                                                        )}
                                                    </div>
                                                </div>
                                            </div>
                                        </TableCell>
                                        <TableCell className="hidden sm:table-cell text-muted-foreground">
                                            {formatDate(preferences, org.created_at, "PP")}
                                        </TableCell>
                                        <TableCell className="hidden md:table-cell text-muted-foreground">
                                            {formatDate(preferences, org.updated_at, "PP")}
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
                            {total} {total === 1 ? t("organization") : t("organizations")}
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
                        {t("sync_organizations_title", "Sync organizations via API")}
                    </h3>
                    <p className="mt-1 text-sm text-muted-foreground">
                        {t(
                            "sync_organizations_description",
                            "Keep your organizations in sync with your system by using the API to create and update them automatically.",
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
                        <Building2
                            className="h-10 w-10 text-primary/40 transition-all duration-500 group-hover:text-primary/60 group-hover:scale-110"
                            strokeWidth={1.25}
                        />
                    </div>
                    <div className="flex h-20 w-20 items-center justify-center rounded-xl bg-primary/10 -rotate-6 translate-y-4 transition-all duration-500 ease-out delay-75 group-hover:rotate-3 group-hover:translate-y-0 group-hover:bg-primary/15">
                        <Users
                            className="h-10 w-10 text-primary/40 transition-all duration-500 delay-75 group-hover:text-primary/60 group-hover:scale-110"
                            strokeWidth={1.25}
                        />
                    </div>
                    <div className="flex h-20 w-20 items-center justify-center rounded-xl bg-primary/10 rotate-12 -translate-y-2 transition-all duration-500 ease-out delay-150 group-hover:-rotate-6 group-hover:-translate-y-4 group-hover:bg-primary/15">
                        <Settings
                            className="h-10 w-10 text-primary/40 transition-all duration-500 delay-150 group-hover:text-primary/60 group-hover:scale-110 group-hover:rotate-90"
                            strokeWidth={1.25}
                        />
                    </div>
                </div>
            </div>

            {/* Create Organization Dialog */}
            <Dialog open={isCreateOpen} onOpenChange={setIsCreateOpen}>
                <DialogContent className="sm:max-w-2xl max-h-[90vh] overflow-y-auto">
                    <DialogHeader>
                        <DialogTitle>{t("create_organization")}</DialogTitle>
                        <DialogDescription>
                            {t(
                                "create_organization_description",
                                "Create a new organization to group users together.",
                            )}
                        </DialogDescription>
                    </DialogHeader>
                    <div className="grid gap-4 py-4">
                        <div className="grid sm:grid-cols-2 gap-4">
                            <div className="grid gap-2 content-start">
                                <Label htmlFor="external_id">
                                    {t("identifier", "Identifier")} *
                                </Label>
                                <Input
                                    id="external_id"
                                    placeholder={t("enter_identifier", "e.g., org-123")}
                                    value={newOrgExternalId}
                                    onChange={(e) => setNewOrgExternalId(e.target.value)}
                                />
                                <p className="text-xs text-muted-foreground">
                                    {t("identifier_help", "A unique identifier from your system")}
                                </p>
                            </div>
                            <div className="grid gap-2 content-start">
                                <Label htmlFor="name">{t("name")}</Label>
                                <Input
                                    id="name"
                                    placeholder={t("enter_organization_name", "e.g., Acme Corp")}
                                    value={newOrgName}
                                    onChange={(e) => setNewOrgName(e.target.value)}
                                />
                            </div>
                        </div>
                        <div className="grid gap-2">
                            <Label>{t("data", "Data")}</Label>
                            <AttributeEditor
                                value={newOrgData}
                                onChange={setNewOrgData}
                                emptyTitle={t("no_data", "No data")}
                                emptyDescription={t(
                                    "no_data_description_org",
                                    "Add custom attributes to this organization.",
                                )}
                            />
                        </div>
                    </div>
                    <DialogFooter>
                        <Button
                            variant="outline"
                            onClick={() => setIsCreateOpen(false)}
                            disabled={isCreating}
                        >
                            {t("cancel")}
                        </Button>
                        <Button
                            onClick={createOrganization}
                            disabled={!newOrgExternalId.trim() || isCreating}
                        >
                            {isCreating ? t("creating") : t("create")}
                        </Button>
                    </DialogFooter>
                </DialogContent>
            </Dialog>
        </div>
    )
}
