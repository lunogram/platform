import { useCallback, useContext, useState, useRef } from "react"
import { Link, useNavigate } from "react-router"
import { useTranslation } from "react-i18next"
import { Search, ChevronLeft, ChevronRight, Megaphone, ArchiveRestore } from "lucide-react"
import { useQuery } from "@tanstack/react-query"
import { ProjectContext } from "../../contexts"
import { Alert, AlertTitle, AlertDescription } from "@/components/ui/alert"
import { CampaignsIcon } from "@/components/icons"

import { ApiTipCard } from "./ApiTipCard"
import { CampaignRow } from "./CampaignRow"
import { CreateCampaign } from "./CreateCampaign"
import {
    CAMPAIGN_PAGE_SIZE,
    campaignListQuery,
    useArchiveCampaign,
    useDuplicateCampaign,
    useUnarchiveCampaign,
} from "./queries"

import type { UUID } from "@/types/common"

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

interface CampaignsProps {
    create?: boolean
}

export default function Campaigns({ create = false }: CampaignsProps) {
    const pageSize = CAMPAIGN_PAGE_SIZE
    const [project] = useContext(ProjectContext)
    const navigate = useNavigate()
    const { t } = useTranslation()
    const duplicateCampaign = useDuplicateCampaign()
    const archiveCampaign = useArchiveCampaign()
    const unarchiveCampaign = useUnarchiveCampaign()

    const [searchQuery, setSearchQuery] = useState("")
    const [debouncedQuery, setDebouncedQuery] = useState("")
    const [offset, setOffset] = useState(0)
    const [archivedOffset, setArchivedOffset] = useState(0)
    const [showArchived, setShowArchived] = useState(false)
    const searchTimeoutRef = useRef<ReturnType<typeof setTimeout> | undefined>(undefined)

    const handleSearch = useCallback((value: string) => {
        setSearchQuery(value)
        setOffset(0)
        setArchivedOffset(0)
        clearTimeout(searchTimeoutRef.current)
        searchTimeoutRef.current = setTimeout(() => {
            setDebouncedQuery(value)
        }, 300)
    }, [])

    const isArchivedView = showArchived
    const currentOffset = isArchivedView ? archivedOffset : offset
    const list = useQuery(
        campaignListQuery(project.id, {
            offset: currentOffset,
            search: debouncedQuery,
            archived: isArchivedView,
        }),
    )
    const campaigns = list.data?.results ?? []
    const total = list.data?.total ?? 0
    const hasNextPage = currentOffset + pageSize < total
    const hasPrevPage = currentOffset > 0

    const handleNextPage = () => {
        if (!hasNextPage) return
        const setter = isArchivedView ? setArchivedOffset : setOffset
        setter((prev) => prev + pageSize)
    }

    const handlePrevPage = () => {
        if (!hasPrevPage) return
        const setter = isArchivedView ? setArchivedOffset : setOffset
        setter((prev) => Math.max(0, prev - pageSize))
    }

    const handleDuplicateCampaign = (e: React.MouseEvent, id: UUID) => {
        e.stopPropagation()
        duplicateCampaign.mutate(id, {
            onSuccess: (duplicate) => {
                if (duplicate?.id) navigate(`/projects/${project.id}/campaigns/${duplicate.id}`)
            },
        })
    }

    const handleArchiveCampaign = (e: React.MouseEvent, id: UUID) => {
        e.stopPropagation()
        archiveCampaign.mutate(id, { onSuccess: () => setShowArchived(false) })
    }

    const handleUnarchiveCampaign = (id: UUID) => {
        unarchiveCampaign.mutate(id, {
            onSuccess: () => {
                setShowArchived(false)
                setArchivedOffset(0)
            },
        })
    }

    const handleRowClick = (campaign: { id: UUID } & unknown) => {
        navigate(`/projects/${project.id}/campaigns/${campaign.id.toString()}`)
    }

    return (
        <div className="flex flex-col gap-4 sm:gap-6 p-4 sm:p-6">
            {/* Header */}
            <div className="flex items-start gap-4">
                <div className="flex h-14 w-14 items-center justify-center rounded-xl shrink-0 bg-muted [&>svg]:h-7 [&>svg]:w-7 [&>svg]:text-muted-foreground">
                    <CampaignsIcon />
                </div>
                <div className="space-y-1">
                    <h1 className="text-2xl font-semibold tracking-tight">
                        {t("campaign.plural")}
                    </h1>
                    <p className="text-sm text-muted-foreground">
                        {t(
                            "campaigns_description",
                            "Create and manage email, SMS, and push notification campaigns to engage your audience.",
                        )}
                    </p>
                </div>
            </div>

            {/* Provider setup banner */}
            {project.has_provider === false && (
                <Alert>
                    <AlertTitle>{t("setup")}</AlertTitle>
                    <AlertDescription className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
                        <span>{t("setup_integration_description")}</span>
                        <Link to={`/projects/${project.id}/integrations`} className="shrink-0">
                            <Button className="w-full sm:w-auto">{t("setup_integration")}</Button>
                        </Link>
                    </AlertDescription>
                </Alert>
            )}

            {/* Search and Actions */}
            <div className="flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-3 sm:gap-4">
                <div className="relative sm:max-w-sm flex-1">
                    <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                    <Input
                        placeholder={t("search_campaigns", "Search campaigns...")}
                        value={searchQuery}
                        onChange={(e) => handleSearch(e.target.value)}
                        className="pl-9"
                    />
                </div>
                <div className="flex items-center gap-2">
                    <Button
                        variant="ghost"
                        size="sm"
                        className="h-8 w-8 p-0"
                        aria-label={t("show_archived", "Show archived")}
                        aria-pressed={showArchived}
                        onClick={() => {
                            setArchivedOffset(0)
                            setShowArchived((prev) => !prev)
                        }}
                    >
                        <ArchiveRestore className="h-4 w-4" />
                    </Button>
                    <CreateCampaign open={create} />
                </div>
            </div>

            {/* Table */}
            <div className="rounded-lg border bg-card">
                <Table>
                    <TableHeader>
                        <TableRow>
                            <TableHead>{t("name")}</TableHead>
                            <TableHead className="hidden sm:table-cell">{t("delivery")}</TableHead>
                            <TableHead className="hidden md:table-cell">
                                {t("updated_at")}
                            </TableHead>
                            <TableHead className="w-[50px]"></TableHead>
                        </TableRow>
                    </TableHeader>
                    <TableBody>
                        {list.isPending ? (
                            Array.from({ length: 5 }).map((_, i) => (
                                <TableRow key={i}>
                                    <TableCell>
                                        <div className="flex items-center gap-3">
                                            <Skeleton className="h-8 w-8 rounded-md" />
                                            <div className="space-y-1">
                                                <Skeleton className="h-4 w-36" />
                                                <Skeleton className="h-3 w-20" />
                                            </div>
                                        </div>
                                    </TableCell>
                                    <TableCell className="hidden sm:table-cell">
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
                        ) : list.isError ? (
                            <TableRow>
                                <TableCell colSpan={4} className="h-32 text-center">
                                    <div className="flex flex-col items-center gap-3 text-muted-foreground">
                                        <p>
                                            {t("campaigns_load_failed", "Couldn't load campaigns.")}
                                        </p>
                                        <Button
                                            variant="outline"
                                            size="sm"
                                            onClick={() => list.refetch()}
                                        >
                                            {t("retry", "Retry")}
                                        </Button>
                                    </div>
                                </TableCell>
                            </TableRow>
                        ) : campaigns.length === 0 ? (
                            <TableRow>
                                <TableCell colSpan={4} className="h-32 text-center">
                                    <div className="flex flex-col items-center gap-2 text-muted-foreground">
                                        <Megaphone className="h-8 w-8" />
                                        <p>
                                            {isArchivedView
                                                ? t(
                                                      "no_archived_campaigns",
                                                      "No archived campaigns",
                                                  )
                                                : debouncedQuery
                                                  ? t("no_campaigns_found")
                                                  : t("no_campaigns_yet", "No campaigns yet")}
                                        </p>
                                    </div>
                                </TableCell>
                            </TableRow>
                        ) : (
                            campaigns.map((campaign) => (
                                <CampaignRow
                                    key={campaign.id}
                                    campaign={campaign}
                                    archived={isArchivedView}
                                    onOpen={() => handleRowClick(campaign)}
                                    onDuplicate={(e) => handleDuplicateCampaign(e, campaign.id)}
                                    onArchive={(e) => handleArchiveCampaign(e, campaign.id)}
                                    onUnarchive={() => handleUnarchiveCampaign(campaign.id)}
                                />
                            ))
                        )}
                    </TableBody>
                </Table>

                {/* Pagination */}
                {campaigns.length > 0 && (
                    <div className="flex items-center justify-between border-t px-4 py-3">
                        <p className="text-sm text-muted-foreground">
                            {total} {t("campaign.plural").toLowerCase()}
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

            <ApiTipCard />
        </div>
    )
}
